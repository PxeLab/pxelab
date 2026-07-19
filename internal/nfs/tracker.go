package nfs

import (
	"context"
	"sync"
	"time"
)

const (
	connCleanupInterval = 30 * time.Second
	connStaleTimeout    = 60 * time.Second
)

type ClientInfo struct {
	IP           string    `json:"ip"`
	ConnectedAt time.Time `json:"connected_at"`
	LastActivity time.Time `json:"last_activity"`
}

type MountStats struct {
	Connections int          `json:"connections"`
	Clients     []ClientInfo `json:"clients"`
}

type ConnectionTracker struct {
	mu     sync.RWMutex
	conns  map[string]map[string]*ClientInfo // exportPath -> clientIP -> info
	cancel func()
}

func NewConnectionTracker() *ConnectionTracker {
	t := &ConnectionTracker{
		conns: make(map[string]map[string]*ClientInfo),
	}
	return t
}

func (t *ConnectionTracker) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	go t.cleanupLoop(ctx)
}

func (t *ConnectionTracker) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}

func (t *ConnectionTracker) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(connCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t.cleanup()
		}
	}
}

func (t *ConnectionTracker) cleanup() {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	for exportPath, clients := range t.conns {
		for ip, info := range clients {
			if now.Sub(info.LastActivity) > connStaleTimeout {
				delete(clients, ip)
			}
		}
		if len(clients) == 0 {
			delete(t.conns, exportPath)
		}
	}
}

func (t *ConnectionTracker) RecordMount(exportPath, clientIP string) {
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.conns[exportPath] == nil {
		t.conns[exportPath] = make(map[string]*ClientInfo)
	}
	t.conns[exportPath][clientIP] = &ClientInfo{
		IP:           clientIP,
		ConnectedAt: now,
		LastActivity: now,
	}
}

func (t *ConnectionTracker) RecordUnmount(exportPath, clientIP string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if clients, ok := t.conns[exportPath]; ok {
		delete(clients, clientIP)
		if len(clients) == 0 {
			delete(t.conns, exportPath)
		}
	}
}

func (t *ConnectionTracker) RecordActivity(exportPath, clientIP string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if clients, ok := t.conns[exportPath]; ok {
		if info, ok := clients[clientIP]; ok {
			info.LastActivity = time.Now()
		}
	}
}

func (t *ConnectionTracker) GetStats() map[string]MountStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	stats := make(map[string]MountStats, len(t.conns))
	for exportPath, clients := range t.conns {
		var clientList []ClientInfo
		for _, info := range clients {
			clientList = append(clientList, *info)
		}
		stats[exportPath] = MountStats{
			Connections: len(clients),
			Clients:     clientList,
		}
	}
	return stats
}
