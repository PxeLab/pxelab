package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

var (
	eventIDMu   sync.Mutex
	eventIDNext int64
)

// nextEventID 生成唯一事件 ID，格式: 时间戳-序列号
func nextEventID() string {
	eventIDMu.Lock()
	eventIDNext++
	seq := eventIDNext
	eventIDMu.Unlock()
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), seq)
}

const ringBufferSize = 1000

type eventRing struct {
	events [ringBufferSize]models.Event
	pos    int
	full   bool
	mu     sync.Mutex
}

func newEventRing() *eventRing {
	return &eventRing{}
}

func (r *eventRing) Push(e models.Event) {
	r.mu.Lock()
	r.events[r.pos] = e
	r.pos = (r.pos + 1) % ringBufferSize
	if r.pos == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

func (r *eventRing) List(filter store.EventFilter) ([]models.Event, int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	total := r.pos
	if r.full {
		total = ringBufferSize
	}

	all := make([]models.Event, 0, total)
	for i := 0; i < total; i++ {
		idx := (r.pos - total + i) % ringBufferSize
		if idx < 0 {
			idx += ringBufferSize
		}
		all = append(all, r.events[idx])
	}

	// 反向：最新的在前
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	// 过滤
	filtered := make([]models.Event, 0, len(all))
	for _, e := range all {
		if filter.Type != "" && !strings.EqualFold(string(e.Type), filter.Type) {
			continue
		}
		if filter.Level != "" && !strings.EqualFold(string(e.Level), filter.Level) {
			continue
		}
		if filter.Mac != "" && (e.MAC == nil || !strings.EqualFold(*e.MAC, filter.Mac)) {
			continue
		}
		filtered = append(filtered, e)
	}

	totalCount := int64(len(filtered))

	// 分页
	page := filter.Page
	size := filter.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 50
	}
	start := (page - 1) * size
	if start >= len(filtered) {
		return []models.Event{}, totalCount
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], totalCount
}

type EventHandler struct {
	store    store.Interface
	eventBus *eventbus.Bus
	ring     *eventRing
	subOnce  sync.Once
}

func NewEventHandler(st store.Interface, bus *eventbus.Bus) *EventHandler {
	h := &EventHandler{
		store:    st,
		eventBus: bus,
		ring:     newEventRing(),
	}
	// 立即订阅事件，确保启动后所有事件都被收集到环形缓冲区
	bus.Subscribe("event", func(e eventbus.Event) {
		if evt, ok := e.Payload.(models.Event); ok {
			evt.ID = nextEventID()
			if evt.Timestamp.IsZero() {
				evt.Timestamp = time.Now()
			}
			h.ring.Push(evt)
			// WARN/ERROR 级别持久化到数据库
			if evt.Level == models.EventWarn || evt.Level == models.EventError {
				if err := h.store.CreateEvent(context.Background(), &evt); err != nil {
					// 静默失败，不影响主流程
				}
			}
		}
	})
	return h
}

func (h *EventHandler) ensureSubscribed() {
	h.subOnce.Do(func() {
		// 保留确保向后兼容，订阅已在构造函数中完成
	})
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))

	h.ensureSubscribed()

	filter := store.EventFilter{
		Type:  r.URL.Query().Get("type"),
		Level: r.URL.Query().Get("level"),
		Mac:   r.URL.Query().Get("mac"),
		Page:  page,
		Size:  size,
	}

	events, total := h.ring.List(filter)
	OK(w, map[string]any{"events": events, "meta": Meta{Page: filter.Page, Size: filter.Size, Total: total}})
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
