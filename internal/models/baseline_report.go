package models

import "time"

// BaselineReport 基线脚本执行回执。
// 装机后首次启动执行聚合基线（pull.sh / pull.ps1）时逐条上报：
// 哪条脚本、第几个、退出码、耗时与输出尾部，用于主机详情页排障。
type BaselineReport struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	HostID     string    `json:"host_id" gorm:"index;not null"`
	ScriptName string    `json:"script_name" gorm:"not null"`
	Seq        int       `json:"seq" gorm:"not null;default:0"`
	ExitCode   int       `json:"exit_code" gorm:"not null;default:0"`
	DurationMs int64     `json:"duration_ms" gorm:"not null;default:0"`
	OutputTail string    `json:"output_tail" gorm:"type:text"` // ≤4KB，服务端截断
	CreatedAt  time.Time `json:"created_at"`
}
