package models

import "time"

type WOLHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MAC       string    `gorm:"size:17;index" json:"mac"`
	HostName  string    `gorm:"size:255" json:"host_name"`
	Broadcast string    `gorm:"size:43" json:"broadcast"`
	SourceIP  string    `gorm:"size:45" json:"source_ip"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `gorm:"size:255" json:"error_msg,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type WOLSchedule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	MAC         string    `gorm:"size:17;index" json:"mac"`
	HostName    string    `gorm:"size:255" json:"host_name"`
	ScheduleAt  time.Time `json:"schedule_at"`
	CronExpr    string    `gorm:"size:64" json:"cron_expr,omitempty"`
	RepeatType  string    `gorm:"size:16;default:'once'" json:"repeat_type"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
