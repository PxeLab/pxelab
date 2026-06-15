package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Profile struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex"`
	Description string    `json:"description"`
	MenuJSON    string    `json:"-" gorm:"column:menu"`
	IsDefault   bool      `json:"is_default" gorm:"index"`
	Arch        *uint16   `json:"arch,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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

type BootMenu struct {
	Entries []MenuEntry `json:"entries"`
}

type MenuEntry struct {
	Label   string  `json:"label"`
	Type    string  `json:"type"` // local | direct | chain | sanboot | wds
	Kernel  *string `json:"kernel,omitempty"`
	Initrd  *string `json:"initrd,omitempty"`
	Cmdline *string `json:"cmdline,omitempty"`
	URL     *string `json:"url,omitempty"`
	WIM     *string `json:"wim,omitempty"`
}

// Scan 和 Value 实现 sql.Scanner 和 driver.Valuer 以便 GORM 存储
func (bm *BootMenu) Scan(value any) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), bm)
}

func (bm BootMenu) Value() (driver.Value, error) {
	return json.Marshal(bm)
}
