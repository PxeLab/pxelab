package api

import (
	"net/http"
	"time"
)

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	OK(w, map[string]any{
		"status":    "ok",
		"version":   "0.1.0",
		"uptime":    time.Now().Unix(),
		"services":  map[string]string{},
	})
}
