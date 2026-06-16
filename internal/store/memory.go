package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pxego/pxego/internal/models"
)

var ErrNotFound = errors.New("record not found")

type memoryStore struct {
	mu       sync.RWMutex
	hosts    map[string]*models.Host
	profiles map[string]*models.Profile
	events   []models.Event
	leases   map[string]*models.Lease
	hostIdx  int
	profIdx  int
}

func NewMemory() Interface {
	return &memoryStore{
		hosts:    make(map[string]*models.Host),
		profiles: make(map[string]*models.Profile),
		events:   make([]models.Event, 0),
		leases:   make(map[string]*models.Lease),
	}
}

func (s *memoryStore) Migrate() error { return nil }

func (s *memoryStore) Close() error { return nil }

// ── Hosts ──

func (s *memoryStore) ListHosts(_ context.Context, search string, page, size int) ([]models.Host, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []models.Host
	for _, h := range s.hosts {
		if search != "" {
			q := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(h.Name), q) &&
				!strings.Contains(strings.ToLower(h.MAC), q) &&
				!strings.Contains(h.IP, q) {
				continue
			}
		}
		list = append(list, *h)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})

	total := int64(len(list))
	offset := (page - 1) * size
	if offset >= len(list) {
		return nil, total, nil
	}
	end := offset + size
	if end > len(list) {
		end = len(list)
	}
	return list[offset:end], total, nil
}

func (s *memoryStore) GetHost(_ context.Context, id string) (*models.Host, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	h, ok := s.hosts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return h, nil
}

func (s *memoryStore) GetHostByMAC(_ context.Context, mac string) (*models.Host, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, h := range s.hosts {
		if strings.EqualFold(h.MAC, mac) {
			return h, nil
		}
	}
	return nil, ErrNotFound
}

func (s *memoryStore) CreateHost(_ context.Context, host *models.Host) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostIdx++
	host.ID = fmt.Sprintf("mem-host-%d", s.hostIdx)
	host.CreatedAt = time.Now()
	host.UpdatedAt = time.Now()
	s.hosts[host.ID] = host
	return nil
}

func (s *memoryStore) UpdateHost(_ context.Context, host *models.Host) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.hosts[host.ID]; !ok {
		return ErrNotFound
	}
	host.UpdatedAt = time.Now()
	s.hosts[host.ID] = host
	return nil
}

func (s *memoryStore) DeleteHost(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.hosts[id]; !ok {
		return ErrNotFound
	}
	delete(s.hosts, id)
	return nil
}

// ── Profiles ──

func (s *memoryStore) ListProfiles(_ context.Context) ([]models.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.Profile
	for _, p := range s.profiles {
		list = append(list, *p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) GetDefaultProfile(_ context.Context) (*models.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.profiles {
		if p.IsDefault {
			return p, nil
		}
	}
	return nil, ErrNotFound
}

func (s *memoryStore) GetProfile(_ context.Context, id string) (*models.Profile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.profiles[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

func (s *memoryStore) CreateProfile(_ context.Context, profile *models.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if profile.IsDefault {
		for _, p := range s.profiles {
			p.IsDefault = false
		}
	}
	s.profIdx++
	profile.ID = fmt.Sprintf("mem-prof-%d", s.profIdx)
	profile.CreatedAt = time.Now()
	profile.UpdatedAt = time.Now()
	s.profiles[profile.ID] = profile
	return nil
}

func (s *memoryStore) UpdateProfile(_ context.Context, profile *models.Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.profiles[profile.ID]; !ok {
		return ErrNotFound
	}
	if profile.IsDefault {
		for _, p := range s.profiles {
			if p.ID != profile.ID {
				p.IsDefault = false
			}
		}
	}
	profile.UpdatedAt = time.Now()
	s.profiles[profile.ID] = profile
	return nil
}

func (s *memoryStore) DeleteProfile(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.profiles[id]; !ok {
		return ErrNotFound
	}
	delete(s.profiles, id)
	return nil
}

// ── Events ──

func (s *memoryStore) ListEvents(_ context.Context, filter EventFilter) ([]models.Event, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []models.Event
	for _, e := range s.events {
		if filter.Type != "" && !strings.EqualFold(string(e.Type), filter.Type) {
			continue
		}
		if filter.Level != "" && !strings.EqualFold(string(e.Level), filter.Level) {
			continue
		}
		list = append(list, e)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Timestamp.After(list[j].Timestamp)
	})

	total := int64(len(list))
	offset := (filter.Page - 1) * filter.Size
	if offset >= len(list) {
		return nil, total, nil
	}
	end := offset + filter.Size
	if end > len(list) {
		end = len(list)
	}
	return list[offset:end], total, nil
}

func (s *memoryStore) CreateEvent(_ context.Context, event *models.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	s.events = append(s.events, *event)
	return nil
}

func (s *memoryStore) PruneEvents(_ context.Context, before int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Unix(before, 0)
	var kept []models.Event
	for _, e := range s.events {
		if !e.Timestamp.Before(cutoff) {
			kept = append(kept, e)
		}
	}
	s.events = kept
	return nil
}

// ── Leases ──

func (s *memoryStore) ListLeases(_ context.Context) ([]models.Lease, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.Lease
	for _, l := range s.leases {
		list = append(list, *l)
	}
	return list, nil
}

func (s *memoryStore) CreateLease(_ context.Context, lease *models.Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.leases[lease.MAC] = lease
	return nil
}

func (s *memoryStore) DeleteLease(_ context.Context, mac string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.leases, mac)
	return nil
}

func (s *memoryStore) PruneLeases(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for mac, l := range s.leases {
		if l.ExpiresAt.Before(now) {
			delete(s.leases, mac)
		}
	}
	return nil
}
