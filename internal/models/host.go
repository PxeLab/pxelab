package models

import (
	"encoding/json"
	"time"
)

type Host struct {
	ID           string  `json:"id" gorm:"primaryKey"`
	Name         string  `json:"name" gorm:"uniqueIndex"`
	MAC          string  `json:"mac" gorm:"uniqueIndex"`
	SN           string  `json:"sn" gorm:"index"` // 主板序列号（辅助身份键，按配置选择）
	IP           string  `json:"ip"`
	ProfileID    *string `json:"profile_id"`
	BMCAddr      string  `json:"bmc_addr"`
	BMCUser      string  `json:"bmc_user"`
	BMCPass      string  `json:"-" gorm:"column:bmc_pass"` // 敏感字段不序列化
	MenuOverride *string `json:"menu_override"`
	// BaselineIDs / ScriptIDs：主机在所属 Profile 继承之外“额外追加”的
	// 初始化脚本集与单脚本（JSON 数组，见 Unmarshal/MarshalJSON）。
	BaselineIDs string     `json:"-" gorm:"type:text"` // JSON: ["bl-sec","bl-mon"]
	ScriptIDs   string     `json:"-" gorm:"type:text"` // JSON: [1,3,7]
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastOnline  *time.Time `json:"last_online"`
	BootCount   int        `json:"boot_count"`
}

func (h *Host) GetBaselineIDs() ([]string, error) {
	if h.BaselineIDs == "" {
		return nil, nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(h.BaselineIDs), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (h *Host) SetBaselineIDs(ids []string) error {
	if ids == nil {
		h.BaselineIDs = ""
		return nil
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	h.BaselineIDs = string(data)
	return nil
}

func (h *Host) GetScriptIDs() ([]uint, error) {
	if h.ScriptIDs == "" {
		return nil, nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(h.ScriptIDs), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (h *Host) SetScriptIDs(ids []uint) error {
	if ids == nil {
		h.ScriptIDs = ""
		return nil
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	h.ScriptIDs = string(data)
	return nil
}

// UnmarshalJSON 支持请求体里的 baseline_ids / script_ids 数组。
func (h *Host) UnmarshalJSON(data []byte) error {
	type Alias Host
	aux := &struct {
		BaselineIDs []string `json:"baseline_ids"`
		ScriptIDs   []uint   `json:"script_ids"`
		*Alias
	}{Alias: (*Alias)(h)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if err := h.SetBaselineIDs(aux.BaselineIDs); err != nil {
		return err
	}
	if err := h.SetScriptIDs(aux.ScriptIDs); err != nil {
		return err
	}
	return nil
}

// MarshalJSON 把内部 JSON 字符串还原为数组暴露给前端。
func (h *Host) MarshalJSON() ([]byte, error) {
	type Alias Host
	bls, _ := h.GetBaselineIDs()
	scs, _ := h.GetScriptIDs()
	if bls == nil {
		bls = []string{}
	}
	if scs == nil {
		scs = []uint{}
	}
	return json.Marshal(&struct {
		*Alias
		BaselineIDs []string `json:"baseline_ids"`
		ScriptIDs   []uint   `json:"script_ids"`
	}{
		Alias:       (*Alias)(h),
		BaselineIDs: bls,
		ScriptIDs:   scs,
	})
}
