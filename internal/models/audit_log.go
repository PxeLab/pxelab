package models

import "time"

type AuditAction string

const (
	AuditCreate AuditAction = "CREATE"
	AuditUpdate AuditAction = "UPDATE"
	AuditDelete AuditAction = "DELETE"
)

type AuditLog struct {
	ID         string      `json:"id" gorm:"primaryKey"`
	Action     AuditAction `json:"action" gorm:"index"`
	Resource   string      `json:"resource" gorm:"index"`
	ResourceID string      `json:"resource_id"`
	RemoteIP   string      `json:"remote_ip" gorm:"index"`
	Detail     string      `json:"detail"`
	Timestamp  time.Time   `json:"timestamp" gorm:"index"`
}
