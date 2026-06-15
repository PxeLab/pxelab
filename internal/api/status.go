package api

import (
	"net/http"
	"time"
)

var startTime = time.Now()

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	services := h.Services
	if services == nil {
		services = map[string]string{}
	}
	OK(w, map[string]any{
		"status":   "ok",
		"version":  "0.1.0",
		"uptime":   int(time.Since(startTime).Seconds()),
		"services": services,
	})
}
