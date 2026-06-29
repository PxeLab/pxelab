package models

import "time"

type DNSRecord struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"not null;index:idx_name_type"`
	Type      string    `json:"type" gorm:"not null;index:idx_name_type"` // A, AAAA, CNAME, TXT, MX
	Value     string    `json:"value" gorm:"not null"`
	TTL       uint32    `json:"ttl" gorm:"default:300"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	Subnet    string    `json:"subnet" gorm:"default:'';index"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
