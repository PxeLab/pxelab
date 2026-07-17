package models

import "time"

type OSImage struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Name         string    `json:"name" gorm:"not null"`
	Filename     string    `json:"filename" gorm:"not null"`
	Size         int64     `json:"size"`
	Distro       string    `json:"distro"`
	Version      string    `json:"version"`
	Arch         string    `json:"arch"`
	Status       string    `json:"status" gorm:"not null;default:uploading"`
	MountPoint   string    `json:"mount_point"`
	ExtractedTo  string    `json:"extracted_to"`
	Checksum     string    `json:"checksum"`
	ErrorMessage string    `json:"error_message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
