package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pxelab/pxelab/internal/network"
)

type NetworkHandler struct{}

func (h *NetworkHandler) Ping(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host      string `json:"host"`
		Count     int    `json:"count"`
		TimeoutMs int    `json:"timeout_ms"`
		IntervalMs int   `json:"interval_ms"`
		Size      int    `json:"size"`
		TTL       int    `json:"ttl"`
		Interface string `json:"interface"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Host == "" {
		Error(w, http.StatusBadRequest, "主机地址不能为空")
		return
	}

	// Resolve interface to local IP
	var localIP string
	if req.Interface != "" {
		localIP = resolveInterfaceIP(req.Interface)
	}

	opts := network.PingOptions{
		Count:     req.Count,
		Timeout:   time.Duration(req.TimeoutMs) * time.Millisecond,
		Interval:  time.Duration(req.IntervalMs) * time.Millisecond,
		Size:      req.Size,
		TTL:       req.TTL,
		LocalAddr: localIP,
	}

	result, err := network.Ping(r.Context(), req.Host, opts, nil)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, result)
}

// POST /api/v1/network/ping/stream — SSE continuous ping
func (h *NetworkHandler) PingStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "不支持 SSE")
		return
	}

	var req struct {
		Host      string `json:"host"`
		Count     int    `json:"count"`
		TimeoutMs int    `json:"timeout_ms"`
		IntervalMs int   `json:"interval_ms"`
		Size      int    `json:"size"`
		TTL       int    `json:"ttl"`
		Interface string `json:"interface"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Host == "" {
		Error(w, http.StatusBadRequest, "主机地址不能为空")
		return
	}

	var localIP string
	if req.Interface != "" {
		localIP = resolveInterfaceIP(req.Interface)
	}

	opts := network.PingOptions{
		Count:     req.Count,
		Timeout:   time.Duration(req.TimeoutMs) * time.Millisecond,
		Interval:  time.Duration(req.IntervalMs) * time.Millisecond,
		Size:      req.Size,
		TTL:       req.TTL,
		LocalAddr: localIP,
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		<-r.Context().Done()
		cancel()
	}()

	var wg sync.WaitGroup
	wg.Add(1)

	result, err := network.Ping(ctx, req.Host, opts, func(pkt network.PingPacket) {
		data, _ := json.Marshal(pkt)
		select {
		case <-ctx.Done():
			return
		default:
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
			flusher.Flush()
		}
	})

	_ = err
	wg.Done()

	if result != nil {
		summary, _ := json.Marshal(map[string]any{
			"type":     "summary",
			"sent":     result.Sent,
			"received": result.Received,
			"lost":     result.Lost,
			"min_rtt":  result.MinRTT,
			"max_rtt":  result.MaxRTT,
			"avg_rtt":  result.AvgRTT,
		})
		_, _ = w.Write([]byte("data: " + string(summary) + "\n\n"))
		flusher.Flush()
	}
}

func (h *NetworkHandler) Traceroute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host      string `json:"host"`
		MaxHops   int    `json:"max_hops"`
		TimeoutMs int    `json:"timeout_ms"`
		Probes    int    `json:"probes"`
		Interface string `json:"interface"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Host == "" {
		Error(w, http.StatusBadRequest, "主机地址不能为空")
		return
	}

	var localIP string
	if req.Interface != "" {
		localIP = resolveInterfaceIP(req.Interface)
	}

	opts := network.TracerouteOptions{
		MaxHops:   req.MaxHops,
		Timeout:   time.Duration(req.TimeoutMs) * time.Millisecond,
		Probes:    req.Probes,
		LocalAddr: localIP,
	}

	result, err := network.Traceroute(r.Context(), req.Host, opts)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, result)
}

// GET /api/v1/network/interfaces — 返回可用网卡（名称 + IP）
func (h *NetworkHandler) ListInterfaces(w http.ResponseWriter, r *http.Request) {
	type ifaceInfo struct {
		Name string   `json:"name"`
		IPs  []string `json:"ips"`
	}
	var result []ifaceInfo
	ifaces, err := net.Interfaces()
	if err != nil {
		OK(w, result)
		return
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		info := ifaceInfo{Name: iface.Name}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil {
				info.IPs = append(info.IPs, ip.String())
			}
		}
		if len(info.IPs) > 0 {
			result = append(result, info)
		}
	}
	OK(w, result)
}

func resolveInterfaceIP(name string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Name != name {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.To4() != nil {
				return ip.String()
			}
		}
	}
	return ""
}
