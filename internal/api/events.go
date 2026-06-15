package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

type EventHandler struct {
	store    store.Interface
	eventBus *eventbus.Bus
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 { page = 1 }
	if size < 1 || size > 100 { size = 50 }

	filter := store.EventFilter{
		Type:  r.URL.Query().Get("type"),
		Level: r.URL.Query().Get("level"),
		Page:  page,
		Size:  size,
	}

	events, total, err := h.store.ListEvents(r.Context(), filter)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"events": events, "meta": Meta{Page: page, Size: size, Total: total}})
}

func (h *EventHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "不支持 SSE")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan eventbus.Event, 100)
	id := h.eventBus.Subscribe("event", func(e eventbus.Event) {
		select {
		case ch <- e:
		default:
		}
	})
	defer h.eventBus.Unsubscribe("event", id)

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			evt := e.Payload.(models.Event)
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
