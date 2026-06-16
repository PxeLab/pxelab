package logbus

import (
	"context"
	"log/slog"
	"time"

	"github.com/pxego/pxego/internal/eventbus"
)

type LogEntry struct {
	Time    time.Time              `json:"time"`
	Level   string                 `json:"level"`
	Message string                 `json:"message"`
	Service string                 `json:"service,omitempty"`
	MAC     string                 `json:"mac,omitempty"`
	IP      string                 `json:"ip,omitempty"`
	Attrs   map[string]any        `json:"attrs,omitempty"`
}

type BusHandler struct {
	next slog.Handler
	bus  *eventbus.Bus
}

func NewBusHandler(next slog.Handler, bus *eventbus.Bus) *BusHandler {
	return &BusHandler{next: next, bus: bus}
}

func (h *BusHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *BusHandler) Handle(ctx context.Context, r slog.Record) error {
	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   make(map[string]any),
	}

	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "service":
			entry.Service = a.Value.String()
		case "mac":
			entry.MAC = a.Value.String()
		case "ip":
			entry.IP = a.Value.String()
		default:
			entry.Attrs[a.Key] = a.Value.Any()
		}
		return true
	})

	// 异步发布，不阻塞 slog 输出
	go h.bus.Publish("log", entry)

	return h.next.Handle(ctx, r)
}

func (h *BusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BusHandler{next: h.next.WithAttrs(attrs), bus: h.bus}
}

func (h *BusHandler) WithGroup(name string) slog.Handler {
	return &BusHandler{next: h.next.WithGroup(name), bus: h.bus}
}
