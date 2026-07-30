package models

import (
	"encoding/json"
	"time"
)

// Baseline 操作系统基线，定义一组初始化脚本
type Baseline struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Description string    `json:"description"`
	OSFilter    string    `json:"os_filter" gorm:"type:text"` // JSON: ["rhel9","rocky9","ubuntu22"]
	Variables   string    `json:"-" gorm:"type:text"`         // JSON: {"NTP_SERVER":"ntp.internal"}
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Script is a standalone executable script (shell/bat/powershell).
type Script struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null"`
	Type        string    `json:"type" gorm:"not null;default:shell"` // shell / bat / powershell
	Content     string    `json:"content" gorm:"type:text;not null"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BaselineScriptAssignment is the join table for Baseline ↔ Script many-to-many.
type BaselineScriptAssignment struct {
	BaselineID string `json:"baseline_id" gorm:"primaryKey;index"`
	ScriptID   uint   `json:"script_id" gorm:"primaryKey;index"`
	Seq        int    `json:"seq" gorm:"not null;default:0"`
}

func (BaselineScriptAssignment) TableName() string { return "baseline_script_assignments" }

// GetOSFilter 解析 OSFilter JSON 为字符串切片
func (b *Baseline) GetOSFilter() ([]string, error) {
	if b.OSFilter == "" {
		return nil, nil
	}
	var filters []string
	if err := json.Unmarshal([]byte(b.OSFilter), &filters); err != nil {
		return nil, err
	}
	return filters, nil
}

// SetOSFilter 将字符串切片转为 JSON 存入 OSFilter
func (b *Baseline) SetOSFilter(filters []string) error {
	if filters == nil {
		b.OSFilter = ""
		return nil
	}
	data, err := json.Marshal(filters)
	if err != nil {
		return err
	}
	b.OSFilter = string(data)
	return nil
}

// GetVariablesMap 解析 Variables JSON 为 map[string]string
func (b *Baseline) GetVariablesMap() (map[string]string, error) {
	if b.Variables == "" {
		return nil, nil
	}
	vars := make(map[string]string)
	if err := json.Unmarshal([]byte(b.Variables), &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

// SetVariablesMap 将 map[string]string 转为 JSON 存入 Variables
func (b *Baseline) SetVariablesMap(vars map[string]string) error {
	if vars == nil {
		b.Variables = ""
		return nil
	}
	data, err := json.Marshal(vars)
	if err != nil {
		return err
	}
	b.Variables = string(data)
	return nil
}
