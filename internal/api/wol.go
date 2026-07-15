package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/metrics"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

var wolMetrics = metrics.DefaultRegistry.GetOrCreate("wol")

type WOLHandler struct {
	store    store.Interface
	config   *config.Config
	eventBus *eventbus.Bus
}

func (h *WOLHandler) broadcastForIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}
	for _, iface := range h.config.Interfaces {
		for _, sn := range iface.Subnets {
			_, cidr, err := net.ParseCIDR(sn.CIDR)
			if err != nil {
				continue
			}
			if cidr.Contains(ip) {
				bcast := make(net.IP, 4)
				for i := range bcast {
					bcast[i] = ip.To4()[i] | ^cidr.Mask[i]
				}
				return bcast.String()
			}
		}
	}
	return ""
}

func (h *WOLHandler) localIPs() []string {
	var ips []string
	for _, iface := range h.config.Interfaces {
		if iface.IP != "" {
			ips = append(ips, iface.IP)
		}
	}
	if len(ips) == 0 {
		ips = append(ips, "")
	}
	return ips
}

func sendWOLPacket(bcastIP, localIP string, mac net.HardwareAddr) error {
	packet := make([]byte, 0, 102)
	packet = append(packet, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}...)
	for i := 0; i < 16; i++ {
		packet = append(packet, mac...)
	}

	ports := []int{9, 7}
	var lastErr error
	for _, port := range ports {
		addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", bcastIP, port))
		if err != nil {
			lastErr = err
			continue
		}
		var local *net.UDPAddr
		if localIP != "" {
			local, _ = net.ResolveUDPAddr("udp4", localIP+":0")
		}
		conn, err := net.DialUDP("udp4", local, addr)
		if err != nil {
			lastErr = err
			continue
		}
		_, writeErr := conn.Write(packet)
		conn.Close()
		if writeErr != nil {
			lastErr = writeErr
			continue
		}
		lastErr = nil
	}
	return lastErr
}

func (h *WOLHandler) recordHistory(mac, hostName, bcast, sourceIP string, success bool, errMsg string) {
	h.store.CreateWOLHistory(nil, &models.WOLHistory{
		MAC:       mac,
		HostName:  hostName,
		Broadcast: bcast,
		SourceIP:  sourceIP,
		Success:   success,
		ErrorMsg:  errMsg,
	})
}

func (h *WOLHandler) wakeAndRecord(mac, hostName, bcast, localIP string) bool {
	wolMetrics.RecordRequest()
	if bcast == "" {
		bcast = "255.255.255.255"
	}
	macHW, err := net.ParseMAC(mac)
	if err != nil {
		wolMetrics.RecordError()
		h.recordHistory(mac, hostName, bcast, localIP, false, "无效 MAC")
		return false
	}
	if err := sendWOLPacket(bcast, localIP, macHW); err != nil {
		wolMetrics.RecordError()
		h.recordHistory(mac, hostName, bcast, localIP, false, err.Error())
		return false
	}
	h.recordHistory(mac, hostName, bcast, localIP, true, "")
	return true
}

// POST /api/v1/hosts/{id}/wake
func (h *WOLHandler) Wake(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host, err := h.store.GetHost(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "主机未找到")
		return
	}
	bcastIP := h.broadcastForIP(host.IP)
	localIP, _ := r.URL.Query()["interface"]
	local := ""
	if len(localIP) > 0 {
		local = localIP[0]
	}
	ok := h.wakeAndRecord(host.MAC, host.Name, bcastIP, local)
	if !ok {
		Error(w, http.StatusInternalServerError, "发送唤醒包失败")
		return
	}
	h.eventBus.Publish("event", models.Event{
		Type:  "WOL",
		Level: models.EventInfo,
		Message: fmt.Sprintf("WOL 唤醒: %s (%s) → %s", host.Name, host.MAC, bcastIP),
	})
	OK(w, map[string]any{"message": "WOL 魔术包已发送", "mac": host.MAC, "broadcast": bcastIP})
}

// POST /api/v1/wol/batch
func (h *WOLHandler) BatchWake(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs       []string `json:"ids"`
		MACs      []string `json:"macs"`
		Interface string   `json:"interface"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	type result struct {
		MAC       string `json:"mac"`
		HostName  string `json:"host_name"`
		Success   bool   `json:"success"`
		Broadcast string `json:"broadcast"`
		Error     string `json:"error,omitempty"`
	}
	var results []result
	seen := make(map[string]bool)

	for _, id := range req.IDs {
		host, err := h.store.GetHost(r.Context(), id)
		if err != nil {
			results = append(results, result{MAC: id, Success: false, Error: "主机未找到"})
			continue
		}
		if seen[host.MAC] {
			continue
		}
		seen[host.MAC] = true
		bcast := h.broadcastForIP(host.IP)
		ok := h.wakeAndRecord(host.MAC, host.Name, bcast, req.Interface)
		results = append(results, result{
			MAC: host.MAC, HostName: host.Name, Success: ok,
			Broadcast: bcast,
		})
		if !ok {
			results[len(results)-1].Error = "发送失败"
		}
	}
	for _, mac := range req.MACs {
		if seen[mac] {
			continue
		}
		seen[mac] = true
		hostName := mac
		host, err := h.store.GetHostByMAC(r.Context(), mac)
		if err == nil {
			hostName = host.Name
		}
		bcast := ""
		if host != nil {
			bcast = h.broadcastForIP(host.IP)
		}
		ok := h.wakeAndRecord(mac, hostName, bcast, req.Interface)
		results = append(results, result{
			MAC: mac, HostName: hostName, Success: ok,
			Broadcast: bcast,
		})
		if !ok {
			results[len(results)-1].Error = "发送失败"
		}
	}
	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
			h.eventBus.Publish("event", models.Event{
				Type:  "WOL",
				Level: models.EventInfo,
				Message: fmt.Sprintf("WOL 唤醒: %s (%s)", r.HostName, r.MAC),
			})
		}
	}
	OK(w, map[string]any{"results": results, "success_count": successCount, "total": len(results)})
}

// GET /api/v1/wol/history
func (h *WOLHandler) ListHistory(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	records, total, err := h.store.ListWOLHistory(r.Context(), page, size)
	if err != nil {
		Error(w, http.StatusInternalServerError, "查询失败")
		return
	}
	OK(w, map[string]any{"records": records, "total": total, "page": page, "size": size})
}

// GET /api/v1/wol/history/{mac}
func (h *WOLHandler) ListHistoryByMAC(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	records, err := h.store.ListWOLHistoryByMAC(r.Context(), mac, 20)
	if err != nil {
		Error(w, http.StatusInternalServerError, "查询失败")
		return
	}
	OK(w, map[string]any{"records": records})
}

// POST /api/v1/wol/schedule
func (h *WOLHandler) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MAC        string `json:"mac"`
		ScheduleAt string `json:"schedule_at"`
		CronExpr   string `json:"cron_expr"`
		RepeatType string `json:"repeat_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.MAC == "" {
		Error(w, http.StatusBadRequest, "MAC 地址不能为空")
		return
	}
	scheduleAt, err := time.Parse(time.RFC3339, req.ScheduleAt)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的定时时间")
		return
	}
	if req.RepeatType == "" {
		req.RepeatType = "once"
	}
	hostName := req.MAC
	host, err := h.store.GetHostByMAC(r.Context(), req.MAC)
	if err == nil {
		hostName = host.Name
	}
	schedule := &models.WOLSchedule{
		MAC:        req.MAC,
		HostName:   hostName,
		ScheduleAt: scheduleAt,
		CronExpr:   req.CronExpr,
		RepeatType: req.RepeatType,
		Enabled:    true,
	}
	if err := h.store.CreateWOLSchedule(r.Context(), schedule); err != nil {
		Error(w, http.StatusInternalServerError, "创建定时唤醒失败")
		return
	}
	OK(w, schedule)
}

// GET /api/v1/wol/schedules
func (h *WOLHandler) ListSchedules(w http.ResponseWriter, r *http.Request) {
	schedules, err := h.store.ListWOLSchedules(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "查询失败")
		return
	}
	OK(w, map[string]any{"schedules": schedules})
}

// DELETE /api/v1/wol/schedule/{id}
func (h *WOLHandler) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.store.DeleteWOLSchedule(r.Context(), uint(id64)); err != nil {
		Error(w, http.StatusInternalServerError, "删除失败")
		return
	}
	OK(w, map[string]string{"message": "已删除"})
}

// GET /api/v1/wol/interfaces — 返回可选网卡 IP 列表
func (h *WOLHandler) ListInterfaces(w http.ResponseWriter, r *http.Request) {
	type ifaceInfo struct {
		Name string   `json:"name"`
		IPs  []string `json:"ips"`
	}
	var result []ifaceInfo
	for _, iface := range h.config.Interfaces {
		info := ifaceInfo{Name: iface.Name}
		if iface.IP != "" {
			info.IPs = append(info.IPs, iface.IP)
		}
		result = append(result, info)
	}
	OK(w, result)
}
