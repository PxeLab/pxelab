package models

import "time"

type BlacklistEntry struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MAC       string    `gorm:"uniqueIndex;size:17" json:"mac"`
	Reason    string    `gorm:"size:255" json:"reason"`
	Source    string    `gorm:"size:32;default:db" json:"source"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WhitelistEntry struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	MAC        string    `gorm:"index:idx_mac_subnet,unique;size:17" json:"mac"`
	SubnetCIDR string    `gorm:"index:idx_mac_subnet,unique;size:43" json:"subnet_cidr"`
	Reason     string    `gorm:"size:255" json:"reason"`
	Source     string    `gorm:"size:32;default:db" json:"source"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
