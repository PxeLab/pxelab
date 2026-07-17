package models

import "time"

type WOLHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	MAC       string    `gorm:"size:17;index" json:"mac"`
	HostName  string    `gorm:"size:255" json:"host_name"`
	Broadcast string    `gorm:"size:43" json:"broadcast"`
	SourceIP  string    `gorm:"size:45" json:"source_ip"`
	Interface string    `gorm:"size:45" json:"interface"`
	Success   bool      `json:"success"`
	ErrorMsg  string    `gorm:"size:255" json:"error_msg,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type WOLSchedule struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	MAC             string     `gorm:"size:17;index" json:"mac"`
	HostName        string     `gorm:"size:255" json:"host_name"`
	ScheduleAt      time.Time  `json:"schedule_at"`
	CronExpr        string     `gorm:"size:64" json:"cron_expr,omitempty"`
	RepeatType      string     `gorm:"size:16;default:'once'" json:"repeat_type"`
	Weekday         int        `gorm:"default:0" json:"weekday"`
	ScheduleTime    string     `gorm:"size:5" json:"schedule_time"`
	CustomBroadcast string     `gorm:"size:43" json:"custom_broadcast,omitempty"`
	Enabled         bool       `gorm:"default:true" json:"enabled"`
	LastRun         *time.Time `json:"last_run,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
