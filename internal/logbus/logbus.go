package logbus

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	next    slog.Handler
	bus     *eventbus.Bus
	logDir  string
	files   map[string]*os.File
	fileMu  sync.Mutex
	closed  bool
}

func NewBusHandler(next slog.Handler, bus *eventbus.Bus, logDir string) *BusHandler {
	h := &BusHandler{
		next:   next,
		bus:    bus,
		logDir: logDir,
		files:  make(map[string]*os.File),
	}
	if logDir != "" {
		os.MkdirAll(logDir, 0755)
	}
	return h
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

	// 写入按服务分离的日志文件
	if h.logDir != "" && entry.Service != "" {
		h.writeServiceLog(entry)
	}

	// 异步发布到 eventbus（供 SSE 实时日志使用）
	go h.bus.Publish("log", entry)

	return h.next.Handle(ctx, r)
}

func (h *BusHandler) writeServiceLog(entry LogEntry) {
	// 服务名转小写文件名
	fname := strings.ToLower(entry.Service) + ".log"
	fpath := filepath.Join(h.logDir, fname)

	h.fileMu.Lock()
	defer h.fileMu.Unlock()

	if h.closed {
		return
	}

	f, ok := h.files[fname]
	if !ok {
		var err error
		f, err = os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		h.files[fname] = f
	}

	// 格式: 时间 LEVEL  message  key=value ...
	var b strings.Builder
	b.Grow(128 + len(entry.Attrs)*32)
	b.WriteString(entry.Time.Format("2006-01-02T15:04:05.000-07:00"))
	b.WriteByte(' ')
	b.WriteString(entry.Level)
	b.WriteByte(' ')
	b.WriteString(entry.Message)
	if entry.MAC != "" {
		b.WriteString(" mac=")
		b.WriteString(entry.MAC)
	}
	if entry.IP != "" {
		b.WriteString(" ip=")
		b.WriteString(entry.IP)
	}
	for k, v := range entry.Attrs {
		b.WriteByte(' ')
		b.WriteString(k)
		b.WriteByte('=')
		fmt.Fprint(&b, v)
	}
	b.WriteByte('\n')

	f.WriteString(b.String())
}

// Close 关闭所有打开的日志文件
func (h *BusHandler) Close() {
	h.fileMu.Lock()
	defer h.fileMu.Unlock()
	h.closed = true
	for name, f := range h.files {
		f.Close()
		delete(h.files, name)
	}
}

func (h *BusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &BusHandler{next: h.next.WithAttrs(attrs), bus: h.bus, logDir: h.logDir, files: h.files}
}

func (h *BusHandler) WithGroup(name string) slog.Handler {
	return &BusHandler{next: h.next.WithGroup(name), bus: h.bus, logDir: h.logDir, files: h.files}
}
