package store

import (
	"context"

	"github.com/pxego/pxego/internal/models"
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
		Where("mac = ? AND subnet_cidr = ?", mac, subnetCIDR).Count(&count).Error
	return count > 0, err
}
