package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

// PxeBootHandler 管理“PXE 引导记录 / 主机认领”。
type PxeBootHandler struct {
	store store.Interface
}

func NewPxeBootHandler(st store.Interface) *PxeBootHandler {
	return &PxeBootHandler{store: st}
}

// List 返回全部引导记录（按最近引导倒序）。
func (h *PxeBootHandler) List(w http.ResponseWriter, r *http.Request) {
	recs, err := h.store.ListPxeBootRecords(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 附带认领主机名（若仍在）
	hostName := map[string]string{}
	if hosts, _, err := h.store.ListHosts(r.Context(), "", 1, 100); err == nil {
		for i := range hosts {
			hostName[hosts[i].ID] = hosts[i].Name
		}
	}
	items := make([]map[string]any, 0, len(recs))
	for i := range recs {
		rec := recs[i]
		claimed := ""
		name := ""
		if rec.ClaimedHostID != nil && *rec.ClaimedHostID != "" {
			claimed = *rec.ClaimedHostID
			name = hostName[claimed]
		}
		items = append(items, map[string]any{
			"mac":             rec.MAC,
			"ip":              rec.IP,
			"loader":          rec.Loader,
			"last_context":    rec.LastContext,
			"first_seen":      rec.FirstSeen.Format("2006-01-02T15:04:05Z"),
			"last_seen":       rec.LastSeen.Format("2006-01-02T15:04:05Z"),
			"count":           rec.Count,
			"claimed_host_id": claimed,
			"claimed_host":    name,
		})
	}
	OK(w, map[string]any{"records": items})
}

// Delete 删除单条记录（忽略，用于清理误报）。
func (h *PxeBootHandler) Delete(w http.ResponseWriter, r *http.Request) {
	mac := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "mac")))
	if err := h.store.DeletePxeBootRecord(r.Context(), mac); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "pxe_boot_record", mac, remoteIP(r), "删除 PXE 引导记录: "+mac)
	w.WriteHeader(http.StatusNoContent)
}

// Clear 清空全部引导记录。
func (h *PxeBootHandler) Clear(w http.ResponseWriter, r *http.Request) {
	if err := h.store.ClearPxeBootRecords(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "pxe_boot_record", "*", remoteIP(r), "清空 PXE 引导记录")
	OK(w, map[string]string{"status": "cleared"})
}

// Claim 认领：把记录标记为主机（主机本身已由前端通过 POST /hosts 创建）。
func (h *PxeBootHandler) Claim(w http.ResponseWriter, r *http.Request) {
	mac := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "mac")))
	var req struct {
		HostID string `json:"host_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.HostID == "" {
		Error(w, http.StatusBadRequest, "host_id 不能为空")
		return
	}
	if _, err := h.store.GetHost(r.Context(), req.HostID); err != nil {
		Error(w, http.StatusNotFound, "主机未找到")
		return
	}
	if err := h.store.ClaimPxeBootRecord(r.Context(), mac, req.HostID); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "pxe_boot_record", mac, remoteIP(r), "认领引导记录为主机: "+req.HostID)
	OK(w, map[string]any{"mac": mac, "claimed_host_id": req.HostID})
}
