package models

import "time"

type InstallTask struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	HostID           string    `json:"host_id" gorm:"not null"`
	DistroName       string    `json:"distro_name" gorm:"not null"`
	VersionCodename  string    `json:"version_codename" gorm:"not null"`
	Arch             string    `json:"arch"`
	AnswerTemplateID *uint     `json:"answer_template_id"`
	ExtraCmdline     string    `json:"extra_cmdline"`
	Status           string    `json:"status" gorm:"default:pending"` // pending / installing / done / failed
	ErrorMsg         string    `json:"error_msg"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
