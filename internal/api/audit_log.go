package api

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

type AuditLogHandler struct {
	store store.Interface
}

func NewAuditLogHandler(st store.Interface) *AuditLogHandler {
	return &AuditLogHandler{store: st}
}

// RecordAudit logs a user action asynchronously (fire-and-forget).
func RecordAudit(ctx context.Context, st store.Interface, action models.AuditAction, resource, resourceID, remoteIP, detail string) {
	log := &models.AuditLog{
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		RemoteIP:   remoteIP,
		Detail:     detail,
		Timestamp:  time.Now(),
	}
	if err := st.CreateAuditLog(ctx, log); err != nil {
		slog.Warn("audit log write failed", "err", err)
	}
}

func remoteIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}

func (h *AuditLogHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 50
	}

	filter := store.AuditLogFilter{
		Action:     r.URL.Query().Get("action"),
		Resource:   r.URL.Query().Get("resource"),
		ResourceID: r.URL.Query().Get("resource_id"),
		RemoteIP:   r.URL.Query().Get("remote_ip"),
		Page:       page,
		Size:       size,
	}

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.From = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.To = t
		}
	}

	logs, total, err := h.store.ListAuditLogs(r.Context(), filter)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if logs == nil {
		logs = []models.AuditLog{}
	}
	OK(w, map[string]any{"logs": logs, "meta": Meta{Page: page, Size: size, Total: total}})
}
