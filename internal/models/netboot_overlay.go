package models

import (
	"encoding/json"
	"time"
)

type NetbootOverlay struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	DistroName       string    `json:"distro_name" gorm:"uniqueIndex;not null"`
	Enabled          bool      `json:"enabled" gorm:"default:false"`
	Mirror           string    `json:"mirror"`
	LocalBase        string    `json:"local_base"`
	KernelParams     string    `json:"kernel_params"`
	VersionOverrides string    `json:"-" gorm:"type:text"` // JSON
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type VersionOverride struct {
	Codename     string `json:"codename"`
	Arch         string `json:"arch"`
	Enabled      *bool  `json:"enabled,omitempty"`
	RemoteKernel string `json:"remote_kernel,omitempty"`
	RemoteInitrd string `json:"remote_initrd,omitempty"`
	Cmdline      string `json:"cmdline,omitempty"`
	AnswerParam  string `json:"answer_param,omitempty"`
	AnswerType   string `json:"answer_type,omitempty"`
}

func (o *NetbootOverlay) GetVersionOverrides() ([]VersionOverride, error) {
	if o.VersionOverrides == "" {
		return nil, nil
	}
	var ovs []VersionOverride
	if err := json.Unmarshal([]byte(o.VersionOverrides), &ovs); err != nil {
		return nil, err
	}
	return ovs, nil
}

func (o *NetbootOverlay) SetVersionOverrides(ovs []VersionOverride) error {
	data, err := json.Marshal(ovs)
	if err != nil {
		return err
	}
	o.VersionOverrides = string(data)
	return nil
}
