package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Profile struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	Name           string    `json:"name" gorm:"uniqueIndex"`
	Description    string    `json:"description"`
	MenuJSON       string    `json:"-" gorm:"column:menu"`
	IsDefault      bool      `json:"is_default" gorm:"index"`
	Arch           string    `json:"arch,omitempty"`
	BaselineIDs    string    `json:"-" gorm:"type:text"` // JSON: ["bl-sec","bl-mon"]
	Variables      string    `json:"-" gorm:"type:text"` // JSON: {"NTP_SERVER":"ntp.internal"}
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GetBaselines 将 BaselineIDs JSON 解析为字符串切片
func (p *Profile) GetBaselines() ([]string, error) {
	if p.BaselineIDs == "" {
		return nil, nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(p.BaselineIDs), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// SetBaselines 将字符串切片转为 JSON 存入 BaselineIDs
func (p *Profile) SetBaselines(ids []string) error {
	if ids == nil {
		p.BaselineIDs = ""
		return nil
	}
	data, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	p.BaselineIDs = string(data)
	return nil
}

// GetVariablesMap 解析 Variables JSON 为 map[string]string
func (p *Profile) GetVariablesMap() (map[string]string, error) {
	if p.Variables == "" {
		return nil, nil
	}
	vars := make(map[string]string)
	if err := json.Unmarshal([]byte(p.Variables), &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

// SetVariablesMap 将 map[string]string 转为 JSON 存入 Variables
func (p *Profile) SetVariablesMap(vars map[string]string) error {
	if vars == nil {
		p.Variables = ""
		return nil
	}
	data, err := json.Marshal(vars)
	if err != nil {
		return err
	}
	p.Variables = string(data)
	return nil
}

func (p *Profile) GetMenu() (*BootMenu, error) {
	if p.MenuJSON == "" {
		return &BootMenu{}, nil
	}
	var menu BootMenu
	if err := json.Unmarshal([]byte(p.MenuJSON), &menu); err != nil {
		return nil, err
	}
	return &menu, nil
}

func (p *Profile) SetMenu(menu *BootMenu) error {
	data, err := json.Marshal(menu)
	if err != nil {
		return err
	}
	p.MenuJSON = string(data)
	return nil
}

// MarshalJSON converts MenuJSON (DB string) to menu (JSON object) for API output.
func (p *Profile) MarshalJSON() ([]byte, error) {
	type Alias Profile
	menu, _ := p.GetMenu()
	baselines, _ := p.GetBaselines()
	if baselines == nil {
		baselines = []string{}
	}
	vars, _ := p.GetVariablesMap()
	if vars == nil {
		vars = map[string]string{}
	}
	return json.Marshal(&struct {
		Menu      BootMenu           `json:"menu"`
		Baselines []string           `json:"baselines"`
		Variables map[string]string  `json:"variables"`
		Alias
	}{
		Menu:      *menu,
		Baselines: baselines,
		Variables: vars,
		Alias:     Alias(*p),
	})
}

// UnmarshalJSON converts menu (JSON object) to MenuJSON (DB string) from API input.
func (p *Profile) UnmarshalJSON(data []byte) error {
	type Alias Profile
	aux := &struct {
		Menu      *BootMenu          `json:"menu"`
		Baselines []string           `json:"baselines"`
		Variables map[string]string  `json:"variables"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.Menu != nil {
		if err := p.SetMenu(aux.Menu); err != nil {
			return err
		}
	}
	if err := p.SetBaselines(aux.Baselines); err != nil {
		return err
	}
	return p.SetVariablesMap(aux.Variables)
}

type BootMenu struct {
	Entries []MenuEntry `json:"entries"`
}

type MenuEntry struct {
	Label     string  `json:"label"`
	Type      string  `json:"type"` // local | direct | chain | sanboot | wds | custom
	Kernel    *string `json:"kernel,omitempty"`
	Initrd    *string `json:"initrd,omitempty"`
	Cmdline   *string `json:"cmdline,omitempty"`
	URL       *string `json:"url,omitempty"`
	WIM       *string `json:"wim,omitempty"`
	Script    *string `json:"script,omitempty"` // raw iPXE script for "custom" type
	IsDefault bool    `json:"is_default,omitempty"`

	// SAN boot options (used when Type is "sanboot")
	SANAction     string `json:"san_action,omitempty"`      // "boot" | "hook" | "zap" | "unhook"
	SANNoDescribe bool   `json:"san_no_describe,omitempty"` // --no-describe flag
	SANDrive      string `json:"san_drive,omitempty"`       // --drive flag, e.g. "0x80"
	SANKeepSAN    bool   `json:"san_keep_san,omitempty"`    // set keep-san 1
}

func (bm *BootMenu) Scan(value any) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), bm)
}

func (bm BootMenu) Value() (driver.Value, error) {
	return json.Marshal(bm)
}
