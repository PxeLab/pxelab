package store

import (
	"context"
	"time"

	"github.com/pxego/pxego/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ── Blacklist ──

func (s *sqliteStore) ListBlacklist(ctx context.Context) ([]models.BlacklistEntry, error) {
	var entries []models.BlacklistEntry
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *sqliteStore) CreateBlacklist(ctx context.Context, entry *models.BlacklistEntry) error {
	return s.db.WithContext(ctx).Create(entry).Error
}

func (s *sqliteStore) DeleteBlacklist(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.BlacklistEntry{}, id).Error
}

func (s *sqliteStore) IsBlacklisted(ctx context.Context, mac string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.BlacklistEntry{}).
		Where("mac = ?", mac).Count(&count).Error
	return count > 0, err
}

// ── Whitelist ──

func (s *sqliteStore) ListWhitelist(ctx context.Context) ([]models.WhitelistEntry, error) {
	var entries []models.WhitelistEntry
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *sqliteStore) CreateWhitelist(ctx context.Context, entry *models.WhitelistEntry) error {
	return s.db.WithContext(ctx).Create(entry).Error
}

func (s *sqliteStore) DeleteWhitelist(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.WhitelistEntry{}, id).Error
}

func (s *sqliteStore) IsWhitelisted(ctx context.Context, mac, subnetCIDR string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.WhitelistEntry{}).
		Where("mac = ? AND (subnet_cidr = ? OR subnet_cidr = '')", mac, subnetCIDR).Count(&count).Error
	return count > 0, err
}

// ── Unauthorized Devices ──

func (s *sqliteStore) ListUnauthorizedDevices(ctx context.Context) ([]models.UnauthorizedDevice, error) {
	var entries []models.UnauthorizedDevice
	if err := s.db.WithContext(ctx).Order("last_seen DESC").Find(&entries).Error; err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *sqliteStore) UpsertUnauthorizedDevice(ctx context.Context, mac, subnetCIDR, reason string) error {
	// Use GORM native upsert with OnConflict — works reliably across all drivers
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "mac"}, {Name: "subnet_cidr"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":     gorm.Expr("count + 1"),
			"last_seen": time.Now(),
			"reason":    reason,
		}),
	}).Create(&models.UnauthorizedDevice{
		MAC:        mac,
		SubnetCIDR: subnetCIDR,
		Reason:     reason,
		Count:      1,
		LastSeen:   time.Now(),
	}).Error
}

func (s *sqliteStore) DeleteUnauthorizedDevice(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&models.UnauthorizedDevice{}, id).Error
}

func (s *sqliteStore) DeleteUnauthorizedDeviceByMAC(ctx context.Context, mac, subnetCIDR string) error {
	return s.db.WithContext(ctx).
		Where("mac = ? AND subnet_cidr = ?", mac, subnetCIDR).
		Delete(&models.UnauthorizedDevice{}).Error
}

func (s *sqliteStore) DeleteAllUnauthorizedDevices(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&models.UnauthorizedDevice{}).Error
}
