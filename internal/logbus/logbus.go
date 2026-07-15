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

	"github.com/pxelab/pxelab/internal/eventbus"
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
	attrs   []slog.Attr // 来自 WithAttrs 的预置属性
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

// Enabled always returns true to receive all levels.
// The next handler's Enabled() is checked in Handle() to skip output it would drop,
// while the eventbus still receives everything for the SSE log stream.
func (h *BusHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *BusHandler) Handle(ctx context.Context, r slog.Record) error {
	entry := LogEntry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   make(map[string]any),
	}

	// 先应用预置属性（来自 WithAttrs，如 service=DHCP）
	// 后续被调用侧显式传入的同名属性覆盖
	for _, a := range h.attrs {
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
	}

	// 再迭代调用侧显式传入的属性（覆盖预置值）
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

	// 写入按服务分离的日志文件（所有级别都写）
	if h.logDir != "" && entry.Service != "" {
		h.writeServiceLog(entry)
	}

	// 异步发布到 eventbus（供 SSE 实时日志使用，所有级别都发布）
	go h.bus.Publish("log", entry)

	// 仅当下一级 handler 接受此级别时才输出到控制台/文件
	if h.next.Enabled(ctx, r.Level) {
		return h.next.Handle(ctx, r)
	}
	return nil
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
	// 合并已有属性和新属性，新属性同名覆盖
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	// 先加已有的，再用新的覆盖或追加
	existing := make(map[string]bool)
	for _, a := range h.attrs {
		merged = append(merged, a)
		existing[a.Key] = true
	}
	for _, a := range attrs {
		if existing[a.Key] {
			// 替换同名的
			for i := range merged {
				if merged[i].Key == a.Key {
					merged[i] = a
					break
				}
			}
		} else {
			merged = append(merged, a)
		}
	}

	return &BusHandler{
		next:   h.next.WithAttrs(attrs),
		bus:    h.bus,
		logDir: h.logDir,
		files:  h.files,
		attrs:  merged,
	}
}

func (h *BusHandler) WithGroup(name string) slog.Handler {
	return &BusHandler{next: h.next.WithGroup(name), bus: h.bus, logDir: h.logDir, files: h.files, attrs: h.attrs}
}
