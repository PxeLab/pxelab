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

	"github.com/pxelab/pxelab/internal/models"
)

var ErrNotFound = errors.New("record not found")

type memoryStore struct {
	mu               sync.RWMutex
	hosts            map[string]*models.Host
	profiles         map[string]*models.Profile
	events           []models.Event
	leases           map[string]*models.Lease
	overlays         map[string]*models.NetbootOverlay
	answerTemplates  map[uint]*models.AnswerTemplate
	tmplVersions     []models.AnswerTemplateVersion
	installTasks     map[string]*models.InstallTask
	dnsRecords       map[uint]*models.DNSRecord
	blacklist        map[string]*models.BlacklistEntry
	whitelist        map[string]*models.WhitelistEntry
	unauthDevices    map[string]*models.UnauthorizedDevice
	bmcConfigs       map[int64]*models.BMCConfig
	dhcpReservations map[uint]*models.DHCPReservation
	blIdx            uint
	wlIdx            uint
	uaIdx            uint
	bmcIdx           int64
	dhcpResIdx       uint
	hostIdx          int
	profIdx          int
	svIdx            uint
	tmplIdx          uint
	tmplVerIdx       uint
	dnsRecIdx        uint
	tmplVerMu        sync.RWMutex
	wolHistory       []models.WOLHistory
	wolSchedules     map[uint]*models.WOLSchedule
	wolHistIdx       uint
	wolSchedIdx      uint
	osImages         map[uint]*models.OSImage
	osImgIdx         uint
	scriptVersions   map[uint]*models.ProfileScriptVersion
	auditLogs        []models.AuditLog
	auditLogIdx      uint
	baselines        map[string]*memoryBaseline
	scIdx            uint
	scripts          map[uint]*models.Script
}

func NewMemory() Interface {
	return &memoryStore{
		hosts:            make(map[string]*models.Host),
		profiles:         make(map[string]*models.Profile),
		scriptVersions:   make(map[uint]*models.ProfileScriptVersion),
		events:           make([]models.Event, 0),
		leases:           make(map[string]*models.Lease),
		overlays:         make(map[string]*models.NetbootOverlay),
		answerTemplates:  make(map[uint]*models.AnswerTemplate),
		tmplVersions:     make([]models.AnswerTemplateVersion, 0),
		installTasks:     make(map[string]*models.InstallTask),
		dnsRecords:       make(map[uint]*models.DNSRecord),
		blacklist:        make(map[string]*models.BlacklistEntry),
		whitelist:        make(map[string]*models.WhitelistEntry),
		unauthDevices:    make(map[string]*models.UnauthorizedDevice),
		bmcConfigs:       make(map[int64]*models.BMCConfig),
		dhcpReservations: make(map[uint]*models.DHCPReservation),
		wolHistory:       make([]models.WOLHistory, 0),
		wolSchedules:     make(map[uint]*models.WOLSchedule),
		osImages:         make(map[uint]*models.OSImage),
		baselines:        make(map[string]*memoryBaseline),
		scripts:          make(map[uint]*models.Script),
	}
}

func (s *memoryStore) Migrate() error { return nil }

func (s *memoryStore) Seed() error {
	if len(s.profiles) == 0 {
		profile := &models.Profile{
			Name:        "Default Boot Configuration",
			Description: "Auto-created default boot configuration; first entry boots from local disk",
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

func (s *memoryStore) GetHostBySN(_ context.Context, sn string) (*models.Host, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, h := range s.hosts {
		if h.SN != "" && strings.EqualFold(h.SN, sn) {
			return h, nil
		}
	}
	return nil, ErrNotFound
}

func (s *memoryStore) CountHostsByScript(_ context.Context, scriptID uint) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var n int64
	for _, h := range s.hosts {
		ids, err := h.GetScriptIDs()
		if err != nil {
			continue
		}
		for _, id := range ids {
			if id == scriptID {
				n++
				break
			}
		}
	}
	return n, nil
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
	existing, ok := s.hosts[host.ID]
	if !ok {
		return ErrNotFound
	}
	host.CreatedAt = existing.CreatedAt
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
	existing, ok := s.profiles[profile.ID]
	if !ok {
		return ErrNotFound
	}
	if profile.IsDefault {
		for _, p := range s.profiles {
			if p.ID != profile.ID {
				p.IsDefault = false
			}
		}
	}
	profile.CreatedAt = existing.CreatedAt
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

func (s *memoryStore) ListScriptVersions(_ context.Context, profileID string) ([]models.ProfileScriptVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.ProfileScriptVersion
	for _, v := range s.scriptVersions {
		if v.ProfileID == profileID {
			list = append(list, *v)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) GetScriptVersion(_ context.Context, id uint) (*models.ProfileScriptVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.scriptVersions[id]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}

func (s *memoryStore) CreateScriptVersion(_ context.Context, v *models.ProfileScriptVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	s.svIdx++
	v.ID = s.svIdx
	cp := *v
	s.scriptVersions[cp.ID] = &cp
	return nil
}

func (s *memoryStore) DeleteScriptVersionsByProfile(_ context.Context, profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, v := range s.scriptVersions {
		if v.ProfileID == profileID {
			delete(s.scriptVersions, id)
		}
	}
	return nil
}

func (s *memoryStore) GetLatestScriptVersion(_ context.Context, profileID string) (*models.ProfileScriptVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest *models.ProfileScriptVersion
	for _, v := range s.scriptVersions {
		if v.ProfileID == profileID {
			if latest == nil || v.CreatedAt.After(latest.CreatedAt) {
				latest = v
			}
		}
	}
	if latest == nil {
		return nil, ErrNotFound
	}
	return latest, nil
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

// ── DNS Records ──

func (s *memoryStore) ListDNSRecords(_ context.Context) ([]models.DNSRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.DNSRecord
	for _, r := range s.dnsRecords {
		list = append(list, *r)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) GetDNSRecord(_ context.Context, id uint) (*models.DNSRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.dnsRecords[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *memoryStore) CreateDNSRecord(_ context.Context, r *models.DNSRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dnsRecIdx++
	r.ID = s.dnsRecIdx
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	s.dnsRecords[r.ID] = r
	return nil
}

func (s *memoryStore) UpdateDNSRecord(_ context.Context, r *models.DNSRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.dnsRecords[r.ID]
	if !ok {
		return ErrNotFound
	}
	r.CreatedAt = existing.CreatedAt
	r.UpdatedAt = time.Now()
	s.dnsRecords[r.ID] = r
	return nil
}

func (s *memoryStore) DeleteDNSRecord(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.dnsRecords[id]; !ok {
		return ErrNotFound
	}
	delete(s.dnsRecords, id)
	return nil
}

func (s *memoryStore) FindDNSRecords(_ context.Context, name string, recordType string, subnet string) ([]models.DNSRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []models.DNSRecord
	for _, r := range s.dnsRecords {
		if r.Name == name && r.Type == recordType && r.Enabled {
			if subnet == "" || r.Subnet == "" || r.Subnet == subnet {
				result = append(result, *r)
			}
		}
	}
	return result, nil
}

// ── Blacklist ──

func (s *memoryStore) ListBlacklist(_ context.Context) ([]models.BlacklistEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.BlacklistEntry
	for _, e := range s.blacklist {
		list = append(list, *e)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) CreateBlacklist(_ context.Context, entry *models.BlacklistEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blIdx++
	entry.ID = s.blIdx
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	s.blacklist[entry.MAC] = entry
	return nil
}

func (s *memoryStore) DeleteBlacklist(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for mac, e := range s.blacklist {
		if e.ID == id {
			delete(s.blacklist, mac)
			return nil
		}
	}
	return ErrNotFound
}

func (s *memoryStore) IsBlacklisted(_ context.Context, mac string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.blacklist[mac]
	return ok, nil
}

// ── Whitelist ──

func (s *memoryStore) ListWhitelist(_ context.Context) ([]models.WhitelistEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.WhitelistEntry
	for _, e := range s.whitelist {
		list = append(list, *e)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) CreateWhitelist(_ context.Context, entry *models.WhitelistEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wlIdx++
	entry.ID = s.wlIdx
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	key := entry.MAC + "|" + entry.SubnetCIDR
	s.whitelist[key] = entry
	return nil
}

func (s *memoryStore) DeleteWhitelist(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, e := range s.whitelist {
		if e.ID == id {
			delete(s.whitelist, key)
			return nil
		}
	}
	return ErrNotFound
}

func (s *memoryStore) IsWhitelisted(_ context.Context, mac, subnetCIDR string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := mac + "|" + subnetCIDR
	allKey := mac + "|"
	if _, ok := s.whitelist[key]; ok {
		return true, nil
	}
	_, ok := s.whitelist[allKey]
	return ok, nil
}

// ── Unauthorized Devices ──

func (s *memoryStore) ListUnauthorizedDevices(_ context.Context) ([]models.UnauthorizedDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.UnauthorizedDevice
	for _, d := range s.unauthDevices {
		list = append(list, *d)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].LastSeen.After(list[j].LastSeen)
	})
	return list, nil
}

func (s *memoryStore) UpsertUnauthorizedDevice(_ context.Context, mac, subnetCIDR, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := mac + "|" + subnetCIDR
	existing, ok := s.unauthDevices[key]
	if ok {
		existing.Count++
		existing.LastSeen = time.Now()
		existing.Reason = reason
		return nil
	}
	s.uaIdx++
	s.unauthDevices[key] = &models.UnauthorizedDevice{
		ID:         s.uaIdx,
		MAC:        mac,
		SubnetCIDR: subnetCIDR,
		Reason:     reason,
		Count:      1,
		LastSeen:   time.Now(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	return nil
}

func (s *memoryStore) DeleteUnauthorizedDevice(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, d := range s.unauthDevices {
		if d.ID == id {
			delete(s.unauthDevices, key)
			return nil
		}
	}
	return ErrNotFound
}

func (s *memoryStore) DeleteUnauthorizedDeviceByMAC(_ context.Context, mac, subnetCIDR string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := mac + "|" + subnetCIDR
	if _, ok := s.unauthDevices[key]; ok {
		delete(s.unauthDevices, key)
		return nil
	}
	return ErrNotFound
}

func (s *memoryStore) DeleteAllUnauthorizedDevices(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unauthDevices = make(map[string]*models.UnauthorizedDevice)
	return nil
}

// ── BMC Config ──

func (s *memoryStore) ListBMCConfigs(_ context.Context) ([]models.BMCConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.BMCConfig
	for _, cfg := range s.bmcConfigs {
		list = append(list, *cfg)
	}
	return list, nil
}

func (s *memoryStore) GetBMCConfig(_ context.Context, id int64) (*models.BMCConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.bmcConfigs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cfg, nil
}

func (s *memoryStore) CreateBMCConfig(_ context.Context, cfg *models.BMCConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bmcIdx++
	cfg.ID = s.bmcIdx
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()
	s.bmcConfigs[cfg.ID] = cfg
	return nil
}

func (s *memoryStore) UpdateBMCConfig(_ context.Context, cfg *models.BMCConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.bmcConfigs[cfg.ID]
	if !ok {
		return ErrNotFound
	}
	cfg.CreatedAt = existing.CreatedAt
	cfg.UpdatedAt = time.Now()
	s.bmcConfigs[cfg.ID] = cfg
	return nil
}

func (s *memoryStore) DeleteBMCConfig(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.bmcConfigs[id]; !ok {
		return ErrNotFound
	}
	delete(s.bmcConfigs, id)
	return nil
}

// ── DHCP Reservations ──

func (s *memoryStore) ListDHCPReservations(_ context.Context, subnetCIDR string) ([]models.DHCPReservation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.DHCPReservation
	for _, r := range s.dhcpReservations {
		if subnetCIDR != "" && r.SubnetCIDR != subnetCIDR {
			continue
		}
		list = append(list, *r)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) GetDHCPReservation(_ context.Context, id uint) (*models.DHCPReservation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.dhcpReservations[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}

func (s *memoryStore) GetDHCPReservationByMAC(_ context.Context, subnetCIDR, mac string) (*models.DHCPReservation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.dhcpReservations {
		if r.SubnetCIDR == subnetCIDR && r.MAC == mac {
			return r, nil
		}
	}
	return nil, ErrNotFound
}

func (s *memoryStore) GetDHCPReservationByIP(_ context.Context, subnetCIDR, ip string) (*models.DHCPReservation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.dhcpReservations {
		if r.SubnetCIDR == subnetCIDR && r.IP == ip {
			return r, nil
		}
	}
	return nil, ErrNotFound
}

func (s *memoryStore) CreateDHCPReservation(_ context.Context, r *models.DHCPReservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dhcpResIdx++
	r.ID = s.dhcpResIdx
	r.CreatedAt = time.Now()
	r.UpdatedAt = time.Now()
	s.dhcpReservations[r.ID] = r
	return nil
}

func (s *memoryStore) UpdateDHCPReservation(_ context.Context, r *models.DHCPReservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.dhcpReservations[r.ID]
	if !ok {
		return ErrNotFound
	}
	r.CreatedAt = existing.CreatedAt
	r.UpdatedAt = time.Now()
	s.dhcpReservations[r.ID] = r
	return nil
}

func (s *memoryStore) DeleteDHCPReservation(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.dhcpReservations[id]; !ok {
		return ErrNotFound
	}
	delete(s.dhcpReservations, id)
	return nil
}

// ── WOL History ──

func (s *memoryStore) ListWOLHistory(_ context.Context, page, size int) ([]models.WOLHistory, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := int64(len(s.wolHistory))
	start := (page - 1) * size
	if start >= len(s.wolHistory) {
		return nil, total, nil
	}
	end := start + size
	if end > len(s.wolHistory) {
		end = len(s.wolHistory)
	}
	out := make([]models.WOLHistory, end-start)
	for i, h := range s.wolHistory[start:end] {
		out[len(out)-1-i] = h
	}
	return out, total, nil
}

func (s *memoryStore) ListWOLHistoryByMAC(_ context.Context, mac string, limit int) ([]models.WOLHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var results []models.WOLHistory
	for i := len(s.wolHistory) - 1; i >= 0 && len(results) < limit; i-- {
		if s.wolHistory[i].MAC == mac {
			results = append(results, s.wolHistory[i])
		}
	}
	return results, nil
}

func (s *memoryStore) CreateWOLHistory(_ context.Context, h *models.WOLHistory) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wolHistIdx++
	h.ID = s.wolHistIdx
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}
	s.wolHistory = append(s.wolHistory, *h)
	return nil
}

func (s *memoryStore) DeleteWOLHistory(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, h := range s.wolHistory {
		if h.ID == id {
			s.wolHistory = append(s.wolHistory[:i], s.wolHistory[i+1:]...)
			return nil
		}
	}
	return nil
}

func (s *memoryStore) DeleteAllWOLHistory(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wolHistory = make([]models.WOLHistory, 0)
	return nil
}

func (s *memoryStore) PruneWOLHistory(_ context.Context, before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var kept []models.WOLHistory
	for _, h := range s.wolHistory {
		if h.CreatedAt.After(before) {
			kept = append(kept, h)
		}
	}
	s.wolHistory = kept
	return nil
}

func (s *memoryStore) ListWOLSchedules(_ context.Context) ([]models.WOLSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []models.WOLSchedule
	for _, sc := range s.wolSchedules {
		out = append(out, *sc)
	}
	return out, nil
}

func (s *memoryStore) GetWOLSchedule(_ context.Context, id uint) (*models.WOLSchedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.wolSchedules[id]
	if !ok {
		return nil, ErrNotFound
	}
	return sc, nil
}

func (s *memoryStore) CreateWOLSchedule(_ context.Context, sc *models.WOLSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wolSchedIdx++
	sc.ID = s.wolSchedIdx
	sc.CreatedAt = time.Now()
	sc.UpdatedAt = time.Now()
	s.wolSchedules[sc.ID] = sc
	return nil
}

func (s *memoryStore) UpdateWOLSchedule(_ context.Context, sc *models.WOLSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.wolSchedules[sc.ID]
	if !ok {
		return ErrNotFound
	}
	sc.CreatedAt = existing.CreatedAt
	sc.UpdatedAt = time.Now()
	s.wolSchedules[sc.ID] = sc
	return nil
}

func (s *memoryStore) DeleteWOLSchedule(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.wolSchedules[id]; !ok {
		return ErrNotFound
	}
	delete(s.wolSchedules, id)
	return nil
}

func (s *memoryStore) ListOSImages(_ context.Context) ([]models.OSImage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	imgs := make([]models.OSImage, 0, len(s.osImages))
	for _, img := range s.osImages {
		imgs = append(imgs, *img)
	}
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].CreatedAt.After(imgs[j].CreatedAt) })
	return imgs, nil
}

func (s *memoryStore) GetOSImage(_ context.Context, id uint) (*models.OSImage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	img, ok := s.osImages[id]
	if !ok {
		return nil, ErrNotFound
	}
	return img, nil
}

func (s *memoryStore) CreateOSImage(_ context.Context, img *models.OSImage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.osImgIdx++
	img.ID = s.osImgIdx
	img.CreatedAt = time.Now()
	img.UpdatedAt = time.Now()
	s.osImages[img.ID] = img
	return nil
}

func (s *memoryStore) UpdateOSImage(_ context.Context, img *models.OSImage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.osImages[img.ID]
	if !ok {
		return ErrNotFound
	}
	img.CreatedAt = existing.CreatedAt
	img.UpdatedAt = time.Now()
	s.osImages[img.ID] = img
	return nil
}

func (s *memoryStore) DeleteOSImage(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.osImages[id]; !ok {
		return ErrNotFound
	}
	delete(s.osImages, id)
	return nil
}

// ── AuditLog ──

func (s *memoryStore) CreateAuditLog(_ context.Context, log *models.AuditLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.auditLogIdx++
	log.ID = fmt.Sprintf("%d", s.auditLogIdx)
	s.auditLogs = append([]models.AuditLog{*log}, s.auditLogs...)
	return nil
}

func (s *memoryStore) ListAuditLogs(_ context.Context, filter AuditLogFilter) ([]models.AuditLog, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var filtered []models.AuditLog
	for _, l := range s.auditLogs {
		if filter.Action != "" && string(l.Action) != filter.Action {
			continue
		}
		if filter.Resource != "" && l.Resource != filter.Resource {
			continue
		}
		if filter.ResourceID != "" && l.ResourceID != filter.ResourceID {
			continue
		}
		if filter.RemoteIP != "" && l.RemoteIP != filter.RemoteIP {
			continue
		}
		if !filter.From.IsZero() && l.Timestamp.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && l.Timestamp.After(filter.To) {
			continue
		}
		filtered = append(filtered, l)
	}
	total := int64(len(filtered))
	start := (filter.Page - 1) * filter.Size
	if start >= len(filtered) {
		return []models.AuditLog{}, total, nil
	}
	end := start + filter.Size
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total, nil
}

func (s *memoryStore) PruneAuditLogs(_ context.Context, before time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, l := range s.auditLogs {
		if !l.Timestamp.Before(before) {
			s.auditLogs[n] = l
			n++
		}
	}
	s.auditLogs = s.auditLogs[:n]
	return nil
}

// ── Baseline ──

type memoryBaseline struct {
	Baseline    models.Baseline
	Assignments []models.BaselineScriptAssignment
}

func (s *memoryStore) ListBaselines(_ context.Context) ([]models.Baseline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.Baseline
	for _, bl := range s.baselines {
		list = append(list, bl.Baseline)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) GetBaseline(_ context.Context, id string) (*models.Baseline, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bl, ok := s.baselines[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := bl.Baseline
	return &cp, nil
}

func (s *memoryStore) CreateBaseline(_ context.Context, b *models.Baseline) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b.CreatedAt = time.Now()
	b.UpdatedAt = time.Now()
	s.baselines[b.ID] = &memoryBaseline{Baseline: *b}
	return nil
}

func (s *memoryStore) UpdateBaseline(_ context.Context, b *models.Baseline) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.baselines[b.ID]
	if !ok {
		return ErrNotFound
	}
	b.CreatedAt = existing.Baseline.CreatedAt
	b.UpdatedAt = time.Now()
	existing.Baseline = *b
	return nil
}

func (s *memoryStore) DeleteBaseline(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.baselines[id]; !ok {
		return ErrNotFound
	}
	delete(s.baselines, id)
	return nil
}

// ── Baseline ↔ Script Associations (many-to-many) ──

func (s *memoryStore) ListBaselineScripts(_ context.Context, baselineID string) ([]models.BaselineScriptAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bl, ok := s.baselines[baselineID]
	if !ok {
		return nil, ErrNotFound
	}
	out := make([]models.BaselineScriptAssignment, len(bl.Assignments))
	copy(out, bl.Assignments)
	return out, nil
}

func (s *memoryStore) SetBaselineScripts(_ context.Context, baselineID string, assignments []models.BaselineScriptAssignment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	bl, ok := s.baselines[baselineID]
	if !ok {
		return ErrNotFound
	}
	cp := make([]models.BaselineScriptAssignment, len(assignments))
	for i, a := range assignments {
		a.BaselineID = baselineID
		cp[i] = a
	}
	bl.Assignments = cp
	return nil
}

// ── Script ──

func (s *memoryStore) ListScripts(_ context.Context) ([]models.Script, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []models.Script
	for _, sc := range s.scripts {
		list = append(list, *sc)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].CreatedAt.After(list[j].CreatedAt)
	})
	return list, nil
}

func (s *memoryStore) GetScript(_ context.Context, id uint) (*models.Script, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.scripts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return sc, nil
}

func (s *memoryStore) CreateScript(_ context.Context, sc *models.Script) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scIdx++
	sc.ID = s.scIdx
	sc.CreatedAt = time.Now()
	sc.UpdatedAt = time.Now()
	s.scripts[sc.ID] = sc
	return nil
}

func (s *memoryStore) UpdateScript(_ context.Context, sc *models.Script) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.scripts[sc.ID]
	if !ok {
		return ErrNotFound
	}
	sc.CreatedAt = existing.CreatedAt
	sc.UpdatedAt = time.Now()
	s.scripts[sc.ID] = sc
	return nil
}

func (s *memoryStore) DeleteScript(_ context.Context, id uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.scripts[id]; !ok {
		return ErrNotFound
	}
	delete(s.scripts, id)
	// Clean up junction references.
	for _, bl := range s.baselines {
		filtered := make([]models.BaselineScriptAssignment, 0, len(bl.Assignments))
		for _, a := range bl.Assignments {
			if a.ScriptID != id {
				filtered = append(filtered, a)
			}
		}
		bl.Assignments = filtered
	}
	return nil
}
