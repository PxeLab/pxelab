package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/pxego/pxego/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sqliteStore struct {
	db *gorm.DB
}

func NewSQLite(dsn string) (Interface, error) {
	// 确保数据库目录存在
	if dir := filepath.Dir(dsn); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1) // SQLite 不支持并发写入
	sqlDB.SetMaxIdleConns(1)
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) Migrate() error {
	return s.db.AutoMigrate(
		&models.Host{},
		&models.Profile{},
		&models.Event{},
		&models.Lease{},
		&models.NetbootOverlay{},
		&models.AnswerTemplate{},
		&models.AnswerTemplateVersion{},
		&models.InstallTask{},
			&models.DNSRecord{},
		&models.BlacklistEntry{},
		&models.WhitelistEntry{},
	)
}

func (s *sqliteStore) Seed() error {
	var count int64
	if err := s.db.Model(&models.Profile{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
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
		if err := profile.SetMenu(&models.BootMenu{Entries: []models.MenuEntry{localEntry}}); err != nil {
			return err
		}
		if err := s.db.Create(profile).Error; err != nil {
			return err
		}
	}

	// 迁移旧版多条目 Profile → 单条目
	return s.migrateMultiEntryProfiles()
}

func (s *sqliteStore) migrateMultiEntryProfiles() error {
	var profiles []models.Profile
	if err := s.db.Find(&profiles).Error; err != nil {
		return err
	}
	for i := range profiles {
		p := &profiles[i]
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
		if err := s.db.Model(&models.Profile{}).Where("id = ?", p.ID).Update("menu", string(data)).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *sqliteStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Host 操作

func (s *sqliteStore) ListHosts(ctx context.Context, search string, page, size int) ([]models.Host, int64, error) {
	var hosts []models.Host
	var total int64
	query := s.db.WithContext(ctx).Model(&models.Host{})
	if search != "" {
		query = query.Where("name LIKE ? OR mac LIKE ? OR ip LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	query.Count(&total)
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&hosts).Error; err != nil {
		return nil, 0, err
	}
	return hosts, total, nil
}

func (s *sqliteStore) GetHost(ctx context.Context, id string) (*models.Host, error) {
	var host models.Host
	if err := s.db.WithContext(ctx).First(&host, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *sqliteStore) GetHostByMAC(ctx context.Context, mac string) (*models.Host, error) {
	var host models.Host
	if err := s.db.WithContext(ctx).First(&host, "mac = ?", mac).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *sqliteStore) CreateHost(ctx context.Context, host *models.Host) error {
	return s.db.WithContext(ctx).Create(host).Error
}

func (s *sqliteStore) UpdateHost(ctx context.Context, host *models.Host) error {
	return s.db.WithContext(ctx).Save(host).Error
}

func (s *sqliteStore) DeleteHost(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Host{}, "id = ?", id).Error
}

// Profile 操作

func (s *sqliteStore) ListProfiles(ctx context.Context) ([]models.Profile, error) {
	var profiles []models.Profile
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func (s *sqliteStore) GetDefaultProfile(ctx context.Context) (*models.Profile, error) {
	var profile models.Profile
	if err := s.db.WithContext(ctx).First(&profile, "is_default = ?", true).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *sqliteStore) GetProfile(ctx context.Context, id string) (*models.Profile, error) {
	var profile models.Profile
	if err := s.db.WithContext(ctx).First(&profile, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *sqliteStore) CreateProfile(ctx context.Context, profile *models.Profile) error {
	if profile.IsDefault {
		s.db.WithContext(ctx).Model(&models.Profile{}).Where("is_default = ?", true).Update("is_default", false)
	}
	return s.db.WithContext(ctx).Create(profile).Error
}

func (s *sqliteStore) UpdateProfile(ctx context.Context, profile *models.Profile) error {
	if profile.IsDefault {
		s.db.WithContext(ctx).Model(&models.Profile{}).Where("is_default = ?", true).Update("is_default", false)
	}
	return s.db.WithContext(ctx).Save(profile).Error
}

func (s *sqliteStore) DeleteProfile(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Profile{}, "id = ?", id).Error
}

// Event 操作

func (s *sqliteStore) ListEvents(ctx context.Context, filter EventFilter) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64
	query := s.db.WithContext(ctx).Model(&models.Event{})
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Level != "" {
		query = query.Where("level = ?", filter.Level)
	}
	query.Count(&total)
	if err := query.Order("timestamp DESC").Offset((filter.Page - 1) * filter.Size).Limit(filter.Size).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func (s *sqliteStore) CreateEvent(ctx context.Context, event *models.Event) error {
	return s.db.WithContext(ctx).Create(event).Error
}

func (s *sqliteStore) PruneEvents(ctx context.Context, before int64) error {
	return s.db.WithContext(ctx).Where("timestamp < ?", time.Unix(before, 0)).Delete(&models.Event{}).Error
}

// Lease 操作

func (s *sqliteStore) ListLeases(ctx context.Context) ([]models.Lease, error) {
	var leases []models.Lease
	if err := s.db.WithContext(ctx).Find(&leases).Error; err != nil {
		return nil, err
	}
	return leases, nil
}

func (s *sqliteStore) CreateLease(ctx context.Context, lease *models.Lease) error {
	return s.db.WithContext(ctx).Save(lease).Error
}

func (s *sqliteStore) DeleteLease(ctx context.Context, mac string) error {
	return s.db.WithContext(ctx).Delete(&models.Lease{}, "mac = ?", mac).Error
}

func (s *sqliteStore) PruneLeases(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&models.Lease{}).Error
}

// ── Netboot Overlays ──

func (s *sqliteStore) ListNetbootOverlays(ctx context.Context) ([]models.NetbootOverlay, error) {
	var overlays []models.NetbootOverlay
	if err := s.db.WithContext(ctx).Order("distro_name ASC").Find(&overlays).Error; err != nil {
		return nil, err
	}
	return overlays, nil
}

func (s *sqliteStore) GetNetbootOverlay(ctx context.Context, distroName string) (*models.NetbootOverlay, error) {
	var overlay models.NetbootOverlay
	if err := s.db.WithContext(ctx).First(&overlay, "distro_name = ?", distroName).Error; err != nil {
		return nil, err
	}
	return &overlay, nil
}

func (s *sqliteStore) UpsertNetbootOverlay(ctx context.Context, o *models.NetbootOverlay) error {
	var existing models.NetbootOverlay
	result := s.db.WithContext(ctx).First(&existing, "distro_name = ?", o.DistroName)
	if result.Error != nil {
		return s.db.WithContext(ctx).Create(o).Error
	}
	o.ID = existing.ID
	o.CreatedAt = existing.CreatedAt
	return s.db.WithContext(ctx).Save(o).Error
}

func (s *sqliteStore) DeleteNetbootOverlay(ctx context.Context, distroName string) error {
	return s.db.WithContext(ctx).Delete(&models.NetbootOverlay{}, "distro_name = ?", distroName).Error
}

// ── Answer Templates ──

func (s *sqliteStore) ListAnswerTemplates(ctx context.Context) ([]models.AnswerTemplate, error) {
	var templates []models.AnswerTemplate
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func (s *sqliteStore) GetAnswerTemplate(ctx context.Context, id uint) (*models.AnswerTemplate, error) {
	var t models.AnswerTemplate
	if err := s.db.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *sqliteStore) CreateAnswerTemplate(ctx context.Context, t *models.AnswerTemplate) error {
	return s.db.WithContext(ctx).Create(t).Error
}

func (s *sqliteStore) UpdateAnswerTemplate(ctx context.Context, t *models.AnswerTemplate) error {
	return s.db.WithContext(ctx).Save(t).Error
}

func (s *sqliteStore) DeleteAnswerTemplate(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.AnswerTemplate{}, "id = ?", id).Error
}

// ── Answer Template Versions ──

func (s *sqliteStore) ListAnswerTemplateVersions(ctx context.Context, templateID uint) ([]models.AnswerTemplateVersion, error) {
	var versions []models.AnswerTemplateVersion
	if err := s.db.WithContext(ctx).Where("template_id = ?", templateID).Order("version DESC").Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

func (s *sqliteStore) GetAnswerTemplateVersion(ctx context.Context, templateID uint, version int) (*models.AnswerTemplateVersion, error) {
	var v models.AnswerTemplateVersion
	if err := s.db.WithContext(ctx).Where("template_id = ? AND version = ?", templateID, version).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *sqliteStore) CreateAnswerTemplateVersion(ctx context.Context, v *models.AnswerTemplateVersion) error {
	return s.db.WithContext(ctx).Create(v).Error
}

func (s *sqliteStore) DeleteAnswerTemplateVersions(ctx context.Context, templateID uint) error {
	return s.db.WithContext(ctx).Delete(&models.AnswerTemplateVersion{}, "template_id = ?", templateID).Error
}

// ── Install Tasks ──

func (s *sqliteStore) ListInstallTasks(ctx context.Context) ([]models.InstallTask, error) {
	var tasks []models.InstallTask
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (s *sqliteStore) GetInstallTask(ctx context.Context, id string) (*models.InstallTask, error) {
	var task models.InstallTask
	if err := s.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *sqliteStore) CreateInstallTask(ctx context.Context, task *models.InstallTask) error {
	return s.db.WithContext(ctx).Create(task).Error
}

func (s *sqliteStore) UpdateInstallTask(ctx context.Context, task *models.InstallTask) error {
	return s.db.WithContext(ctx).Save(task).Error
}

func (s *sqliteStore) DeleteInstallTask(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.InstallTask{}, "id = ?", id).Error
}

func (s *sqliteStore) GetInstallTaskByHostMAC(ctx context.Context, mac string) (*models.InstallTask, error) {
	// Join install_tasks → hosts to find task by MAC
	var task models.InstallTask
	err := s.db.WithContext(ctx).
		Joins("JOIN hosts ON hosts.id = install_tasks.host_id").
		Where("hosts.mac = ?", mac).
		Where("install_tasks.status IN ?", []string{"pending", "installing"}).
		First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// ── DNS Records ──

func (s *sqliteStore) ListDNSRecords(ctx context.Context) ([]models.DNSRecord, error) {
	var records []models.DNSRecord
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (s *sqliteStore) GetDNSRecord(ctx context.Context, id uint) (*models.DNSRecord, error) {
	var r models.DNSRecord
	if err := s.db.WithContext(ctx).First(&r, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *sqliteStore) CreateDNSRecord(ctx context.Context, r *models.DNSRecord) error {
	return s.db.WithContext(ctx).Create(r).Error
}

func (s *sqliteStore) UpdateDNSRecord(ctx context.Context, r *models.DNSRecord) error {
	return s.db.WithContext(ctx).Save(r).Error
}

func (s *sqliteStore) DeleteDNSRecord(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.DNSRecord{}, "id = ?", id).Error
}

func (s *sqliteStore) FindDNSRecords(ctx context.Context, name string, recordType string) ([]models.DNSRecord, error) {
	var records []models.DNSRecord
	if err := s.db.WithContext(ctx).
		Where("name = ? AND type = ? AND enabled = ?", name, recordType, true).
		Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}
