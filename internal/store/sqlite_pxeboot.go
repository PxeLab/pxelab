package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/pxelab/pxelab/internal/models"
	"gorm.io/gorm"
)

func (s *sqliteStore) UpsertPxeBootRecord(ctx context.Context, mac, loader, context, ip string) error {
	mac = strings.ToLower(strings.TrimSpace(mac))
	if mac == "" {
		return nil
	}
	now := time.Now()
	var rec models.PxeBootRecord
	err := s.db.WithContext(ctx).First(&rec, "mac = ?", mac).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.db.WithContext(ctx).Create(&models.PxeBootRecord{
			MAC: mac, IP: ip, Loader: loader, LastContext: context,
			FirstSeen: now, LastSeen: now, Count: 1,
		}).Error
	}
	if err != nil {
		return err
	}
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
	return s.db.WithContext(ctx).Save(&rec).Error
}

func (s *sqliteStore) ListPxeBootRecords(ctx context.Context) ([]models.PxeBootRecord, error) {
	var recs []models.PxeBootRecord
	if err := s.db.WithContext(ctx).Order("last_seen DESC").Find(&recs).Error; err != nil {
		return nil, err
	}
	return recs, nil
}

func (s *sqliteStore) DeletePxeBootRecord(ctx context.Context, mac string) error {
	return s.db.WithContext(ctx).Delete(&models.PxeBootRecord{}, "mac = ?", strings.ToLower(strings.TrimSpace(mac))).Error
}

func (s *sqliteStore) ClearPxeBootRecords(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("1 = 1").Delete(&models.PxeBootRecord{}).Error
}

func (s *sqliteStore) ClaimPxeBootRecord(ctx context.Context, mac, hostID string) error {
	return s.db.WithContext(ctx).Model(&models.PxeBootRecord{}).
		Where("mac = ?", strings.ToLower(strings.TrimSpace(mac))).
		Update("claimed_host_id", hostID).Error
}
