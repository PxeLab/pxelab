package store

import (
	"context"
	"strings"
	"time"

	"github.com/pxelab/pxelab/internal/models"
)

func (s *memoryStore) UpsertPxeBootRecord(_ context.Context, mac, loader, context, ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mac = strings.ToLower(strings.TrimSpace(mac))
	if mac == "" {
		return nil
	}
	now := time.Now()
	if rec, ok := s.pxeBoot[mac]; ok {
		rec.Count++
		rec.LastSeen = now
		if ip != "" {
			rec.IP = ip
		}
		if loader != "" {
			rec.Loader = loader
		}
		if context != "" {
			rec.LastContext = context
		}
		s.pxeBoot[mac] = rec
		return nil
	}
	s.pxeBoot[mac] = models.PxeBootRecord{
		MAC: mac, IP: ip, Loader: loader, LastContext: context,
		FirstSeen: now, LastSeen: now, Count: 1,
	}
	return nil
}

func (s *memoryStore) ListPxeBootRecords(_ context.Context) ([]models.PxeBootRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.PxeBootRecord, 0, len(s.pxeBoot))
	for _, v := range s.pxeBoot {
		out = append(out, v)
	}
	return out, nil
}

func (s *memoryStore) DeletePxeBootRecord(_ context.Context, mac string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pxeBoot, strings.ToLower(strings.TrimSpace(mac)))
	return nil
}

func (s *memoryStore) ClearPxeBootRecords(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pxeBoot = map[string]models.PxeBootRecord{}
	return nil
}

func (s *memoryStore) ClaimPxeBootRecord(_ context.Context, mac, hostID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.pxeBoot[strings.ToLower(strings.TrimSpace(mac))]
	if !ok {
		return nil
	}
	id := hostID
	rec.ClaimedHostID = &id
	s.pxeBoot[rec.MAC] = rec
	return nil
}
