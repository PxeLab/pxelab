package store

import (
	"context"
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
	)
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
	return s.db.WithContext(ctx).Create(lease).Error
}

func (s *sqliteStore) DeleteLease(ctx context.Context, mac string) error {
	return s.db.WithContext(ctx).Delete(&models.Lease{}, "mac = ?", mac).Error
}

func (s *sqliteStore) PruneLeases(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&models.Lease{}).Error
}
