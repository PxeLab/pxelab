package models

import "time"

type Lease struct {
	MAC       string    `json:"mac" gorm:"primaryKey"`
	IP        string    `json:"ip"`
	SubnetID  string    `json:"subnet_id" gorm:"index"`
	Hostname  *string   `json:"hostname"`
	ExpiresAt time.Time `json:"expires_at" gorm:"index"`
	CreatedAt time.Time `json:"created_at"`
}
