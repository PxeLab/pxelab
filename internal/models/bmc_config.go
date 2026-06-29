package models

import "time"

type BMCConfig struct {
	ID        int64     `json:"id" gorm:"primaryKey"`
	Host      string    `json:"host" gorm:"not null"`
	Port      int       `json:"port" gorm:"default:623"`
	Username  string    `json:"username" gorm:"not null"`
	Password  string    `json:"-" gorm:"not null"` // hidden from JSON
	Protocol  string    `json:"protocol" gorm:"default:ipmi"` // ipmi | redfish
	Vendor    string    `json:"vendor" gorm:"default:''"`
	Model     string    `json:"model" gorm:"default:''"`
	Serial    string    `json:"serial" gorm:"default:''"`
	MAC       string    `json:"mac" gorm:"default:''"`
	Name      string    `json:"name" gorm:"default:''"`
	BootMode  string    `json:"boot_mode" gorm:"default:auto"` // auto | uefi | legacy
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
