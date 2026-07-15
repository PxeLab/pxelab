package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/logbus"
)

type LogStreamHandler struct {
	eventBus *eventbus.Bus
	logDir   string
}

func NewLogStreamHandler(bus *eventbus.Bus, logDir string) *LogStreamHandler {
	return &LogStreamHandler{eventBus: bus, logDir: logDir}
}

func (h *LogStreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "不支持 SSE")
		return
	}

	service := r.URL.Query().Get("service")
	level := r.URL.Query().Get("level")

	slog.Debug("SSE logs stream connected", "service", service, "level", level, "remote", r.RemoteAddr)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send "connected" event immediately to confirm the stream is alive
	fmt.Fprintf(w, "event: connected\ndata: {\"time\":\"%s\"}\n\n", time.Now().Format(time.RFC3339))
	flusher.Flush()

	// Load recent history from log files
	if h.logDir != "" {
		h.sendHistory(w, flusher, service, level)
	}

	// Subscribe to real-time events
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

// sendHistory reads recent log entries from disk and sends them as SSE events.
func (h *LogStreamHandler) sendHistory(w http.ResponseWriter, flusher http.Flusher, serviceFilter, levelFilter string) {
	entries, err := h.readLogFiles(serviceFilter, levelFilter)
	if err != nil || len(entries) == 0 {
		return
	}
	for _, entry := range entries {
		data, _ := json.Marshal(entry)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	flusher.Flush()
}

// readLogFiles reads the last N entries from each service log file.
func (h *LogStreamHandler) readLogFiles(serviceFilter, levelFilter string) ([]logbus.LogEntry, error) {
	matches, err := filepath.Glob(filepath.Join(h.logDir, "*.log"))
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}

	// Sort by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		si, _ := os.Stat(matches[i])
		sj, _ := os.Stat(matches[j])
		if si == nil || sj == nil {
			return false
		}
		return si.ModTime().After(sj.ModTime())
	})

	const maxLinesPerFile = 10
	const maxTotalEntries = 30
	var all []logbus.LogEntry

	for _, path := range matches {
		serviceName := strings.TrimSuffix(filepath.Base(path), ".log")
		// Map lowercase filename to proper service name
		serviceName = strings.ToUpper(serviceName)

		if serviceFilter != "" && !strings.EqualFold(serviceName, serviceFilter) {
			continue
		}

		lines, err := tailFile(path, maxLinesPerFile)
		if err != nil {
			continue
		}

		for _, line := range lines {
			entry := parseLogLine(line, serviceName)
			if entry == nil {
				continue
			}
			if levelFilter != "" && entry.Level != levelFilter {
				continue
			}
			all = append(all, *entry)
			if len(all) >= maxTotalEntries {
				goto done
			}
		}
	}
done:
	// Sort by time (oldest first, so frontend displays chronologically)
	sort.Slice(all, func(i, j int) bool {
		return all[i].Time.Before(all[j].Time)
	})
	return all, nil
}

// tailFile reads the last n lines from a file.
func tailFile(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	// Use a larger buffer for long lines
	scanner.Buffer(make([]byte, 1024*64), 1024*64)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		// Keep only the last n
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	}
	return lines, scanner.Err()
}

// parseLogLine parses a log line written by logbus.BusHandler.
// Format: 2006-01-02T15:04:05.000-07:00 LEVEL message key=value ...
func parseLogLine(line, serviceName string) *logbus.LogEntry {
	if len(line) < 30 {
		return nil
	}
	// Parse timestamp (first 29 chars: 2006-01-02T15:04:05.000-07:00)
	ts, err := time.Parse("2006-01-02T15:04:05.000-07:00", line[:29])
	if err != nil {
		// Try without timezone
		ts, err = time.Parse("2006-01-02T15:04:05.000", line[:23])
		if err != nil {
			return nil
		}
	}

	rest := strings.TrimSpace(line[29:])
	fields := strings.Fields(rest)
	if len(fields) < 2 {
		return nil
	}

	level := fields[0]
	entry := &logbus.LogEntry{
		Time:    ts,
		Level:   level,
		Service: serviceName,
		Attrs:   make(map[string]any),
	}

	// Key=value pairs are all at the end. Walk backwards from the last field
	// to find where they start (every attr token contains "=").
	attrStart := len(fields)
	for i := len(fields) - 1; i >= 1; i-- {
		if strings.Contains(fields[i], "=") {
			attrStart = i
		} else {
			break
		}
	}

	// Everything between level and attrStart is the message (may contain spaces)
	if attrStart > 1 {
		entry.Message = strings.Join(fields[1:attrStart], " ")
	}

	// Parse key=value pairs
	for _, token := range fields[attrStart:] {
		if kv := strings.SplitN(token, "=", 2); len(kv) == 2 {
			switch kv[0] {
			case "mac":
				entry.MAC = kv[1]
			case "ip":
				entry.IP = kv[1]
			default:
				entry.Attrs[kv[0]] = kv[1]
			}
		}
	}
	return entry
}
