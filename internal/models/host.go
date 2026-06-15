package models

import "time"

type Host struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	Name         string     `json:"name" gorm:"uniqueIndex"`
	MAC          string     `json:"mac" gorm:"uniqueIndex"`
	IP           string     `json:"ip"`
	ProfileID    *string    `json:"profile_id"`
	BMCAddr      string     `json:"bmc_addr"`
	BMCUser      string     `json:"bmc_user"`
	BMCPass      string     `json:"-" gorm:"column:bmc_pass"` // 敏感字段不序列化
	MenuOverride *string    `json:"menu_override"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastOnline   *time.Time `json:"last_online"`
	BootCount    int        `json:"boot_count"`
}
