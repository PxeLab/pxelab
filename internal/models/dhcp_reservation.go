package models

import "time"

type DHCPReservation struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	InterfaceName string    `json:"interface_name" gorm:"not null"`
	SubnetCIDR    string    `json:"subnet_cidr" gorm:"column:subnet_cidr;not null;index"`
	MAC           string    `json:"mac" gorm:"default:'';index"`
	IP            string    `json:"ip" gorm:"not null"`
	Hostname      string    `json:"hostname" gorm:"default:''"`
	Description   string    `json:"description" gorm:"default:''"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
