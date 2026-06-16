package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/logbus"
)

type LogStreamHandler struct {
	eventBus *eventbus.Bus
}

func NewLogStreamHandler(bus *eventbus.Bus) *LogStreamHandler {
	return &LogStreamHandler{eventBus: bus}
}

func (h *LogStreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "不支持 SSE")
		return
	}

	service := r.URL.Query().Get("service")
	level := r.URL.Query().Get("level")

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan eventbus.Event, 200)
	id := h.eventBus.Subscribe("log", func(e eventbus.Event) {
		select {
		case ch <- e:
		default:
		}
	})
	defer h.eventBus.Unsubscribe("log", id)

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			entry, ok := e.Payload.(logbus.LogEntry)
			if !ok {
				continue
			}

			if service != "" && entry.Service != service {
				continue
			}
			if level != "" && entry.Level != level {
				continue
			}

			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
