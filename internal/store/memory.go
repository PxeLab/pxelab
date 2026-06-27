package store

import (
	"context"
	"encoding/json"
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
	mu              sync.RWMutex
	hosts           map[string]*models.Host
	profiles        map[string]*models.Profile
	events          []models.Event
	leases          map[string]*models.Lease
	overlays        map[string]*models.NetbootOverlay
	answerTemplates map[uint]*models.AnswerTemplate
	tmplVersions    []models.AnswerTemplateVersion
	installTasks    map[string]*models.InstallTask
	hostIdx    int
	profIdx    int
	tmplIdx    uint
	tmplVerIdx uint
	tmplVerMu  sync.RWMutex
}

func NewMemory() Interface {
	return &memoryStore{
		hosts:           make(map[string]*models.Host),
		profiles:        make(map[string]*models.Profile),
		events:          make([]models.Event, 0),
		leases:          make(map[string]*models.Lease),
		overlays:        make(map[string]*models.NetbootOverlay),
		answerTemplates: make(map[uint]*models.AnswerTemplate),
		tmplVersions:    make([]models.AnswerTemplateVersion, 0),
		installTasks:    make(map[string]*models.InstallTask),
	}
}

func (s *memoryStore) Migrate() error { return nil }

func (s *memoryStore) Seed() error {
	if len(s.profiles) == 0 {
		profile := &models.Profile{
			Name:        "默认引导配置",
			Description: "系统自动创建的默认引导配置，首条为本地硬盘启动",
			IsDefault:   true,
			Arch:        "x86_64",
		}
		localEntry := models.MenuEntry{
			Label: "Boot from local disk",
			Type:  "local",
		}
		profile.SetMenu(&models.BootMenu{Entries: []models.MenuEntry{localEntry}})
		if err := s.CreateProfile(context.Background(), profile); err != nil {
			return err
		}
	}

	// 迁移旧版多条目 Profile → 单条目
	return s.migrateMultiEntryProfiles()
}

func (s *memoryStore) migrateMultiEntryProfiles() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.profiles {
		menu, err := p.GetMenu()
		if err != nil || len(menu.Entries) <= 1 {
			continue
		}
		// 保留第一个非 local 的条目；全是 local 则保留第一个
		keep := menu.Entries[0]
		for _, e := range menu.Entries {
			if e.Type != "local" {
				keep = e
				break
			}
		}
		menu.Entries = []models.MenuEntry{keep}
		data, err := json.Marshal(menu)
		if err != nil {
			return err
		}
		p.MenuJSON = string(data)
	}
	return nil
}

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

// ── Netboot Overlays ──

func (s *memoryStore) ListNetbootOverlays(_ context.Context) ([]models.NetbootOverlay, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.NetbootOverlay
	for _, o := range s.overlays {
		list = append(list, *o)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].DistroName < list[j].DistroName
	})
	return list, nil
}

func (s *memoryStore) GetNetbootOverlay(_ context.Context, distroName string) (*models.NetbootOverlay, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.overlays[distroName]
	if !ok {
		return nil, ErrNotFound
	}
	return o, nil
}

func (s *memoryStore) UpsertNetbootOverlay(_ context.Context, o *models.NetbootOverlay) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if existing, ok := s.overlays[o.DistroName]; ok {
		o.ID = existing.ID
		o.CreatedAt = existing.CreatedAt
		o.UpdatedAt = now
	} else {
		o.ID = uint(len(s.overlays) + 1)
		o.CreatedAt = now
		o.UpdatedAt = now
	}
	s.overlays[o.DistroName] = o
	return nil
}

func (s *memoryStore) DeleteNetbootOverlay(_ context.Context, distroName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.overlays[distroName]; !ok {
		return ErrNotFound
	}
	delete(s.overlays, distroName)
	return nil
}

// ── Answer Templates ──

func (s *memoryStore) ListAnswerTemplates(_ context.Context) ([]models.AnswerTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.AnswerTemplate
	for _, t := range s.answerTemplates {
		list = append(list, *t)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) GetAnswerTemplate(_ context.Context, id uint) (*models.AnswerTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.answerTemplates[id]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

func (s *memoryStore) CreateAnswerTemplate(_ context.Context, t *models.AnswerTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tmplIdx++
	t.ID = s.tmplIdx
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	s.answerTemplates[t.ID] = t
	return nil
}

func (s *memoryStore) UpdateAnswerTemplate(_ context.Context, t *models.AnswerTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.answerTemplates[t.ID]; !ok {
		return ErrNotFound
	}
	t.UpdatedAt = time.Now()
	s.answerTemplates[t.ID] = t
	return nil
}

func (s *memoryStore) DeleteAnswerTemplate(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.answerTemplates[id]; !ok {
		return ErrNotFound
	}
	delete(s.answerTemplates, id)
	return nil
}

// ── Answer Template Versions ──

func (s *memoryStore) ListAnswerTemplateVersions(_ context.Context, templateID uint) ([]models.AnswerTemplateVersion, error) {
	s.tmplVerMu.RLock()
	defer s.tmplVerMu.RUnlock()
	var result []models.AnswerTemplateVersion
	for _, v := range s.tmplVersions {
		if v.TemplateID == templateID {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version > result[j].Version
	})
	return result, nil
}

func (s *memoryStore) GetAnswerTemplateVersion(_ context.Context, templateID uint, version int) (*models.AnswerTemplateVersion, error) {
	s.tmplVerMu.RLock()
	defer s.tmplVerMu.RUnlock()
	for _, v := range s.tmplVersions {
		if v.TemplateID == templateID && v.Version == version {
			return &v, nil
		}
	}
	return nil, ErrNotFound
}

func (s *memoryStore) CreateAnswerTemplateVersion(_ context.Context, v *models.AnswerTemplateVersion) error {
	s.tmplVerMu.Lock()
	defer s.tmplVerMu.Unlock()
	s.tmplVerIdx++
	v.ID = s.tmplVerIdx
	s.tmplVersions = append(s.tmplVersions, *v)
	return nil
}

func (s *memoryStore) DeleteAnswerTemplateVersions(_ context.Context, templateID uint) error {
	s.tmplVerMu.Lock()
	defer s.tmplVerMu.Unlock()
	var kept []models.AnswerTemplateVersion
	for _, v := range s.tmplVersions {
		if v.TemplateID != templateID {
			kept = append(kept, v)
		}
	}
	s.tmplVersions = kept
	return nil
}

// ── Install Tasks ──

func (s *memoryStore) ListInstallTasks(_ context.Context) ([]models.InstallTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.InstallTask
	for _, t := range s.installTasks {
		list = append(list, *t)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) GetInstallTask(_ context.Context, id string) (*models.InstallTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.installTasks[id]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

func (s *memoryStore) CreateInstallTask(_ context.Context, task *models.InstallTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	s.installTasks[task.ID] = task
	return nil
}

func (s *memoryStore) UpdateInstallTask(_ context.Context, task *models.InstallTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.installTasks[task.ID]; !ok {
		return ErrNotFound
	}
	task.UpdatedAt = time.Now()
	s.installTasks[task.ID] = task
	return nil
}

func (s *memoryStore) DeleteInstallTask(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.installTasks[id]; !ok {
		return ErrNotFound
	}
	delete(s.installTasks, id)
	return nil
}

func (s *memoryStore) GetInstallTaskByHostMAC(_ context.Context, mac string) (*models.InstallTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, h := range s.hosts {
		if strings.EqualFold(h.MAC, mac) {
			for _, t := range s.installTasks {
				if t.HostID == h.ID && (t.Status == "pending" || t.Status == "installing") {
					return t, nil
				}
			}
		}
	}
	return nil, ErrNotFound
}
