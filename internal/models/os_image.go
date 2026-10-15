package models

import (
	"encoding/json"
	"time"
)

type OSImage struct {
	ID           uint   `json:"id" gorm:"primaryKey"`
	Name         string `json:"name" gorm:"not null"`
	Filename     string `json:"filename" gorm:"not null"`
	SourcePath   string `json:"source_path"` // 导入的外部 ISO 绝对路径；空 = 上传托管在 isos/ 下
	Size         int64  `json:"size"`
	Distro       string `json:"distro"`
	Version      string `json:"version"`
	Arch         string `json:"arch"`
	Status       string `json:"status" gorm:"not null;default:uploading"`
	MountPoint   string `json:"mount_point"`
	ExtractedTo  string `json:"extracted_to"`
	KernelPath   string `json:"kernel_path"` // 相对挂载点的 kernel 路径（自动探测）
	InitrdPath   string `json:"initrd_path"` // 相对挂载点的 initrd 路径（自动探测）
	Checksum     string `json:"checksum"`
	ErrorMessage string `json:"error_message"`
	// InjectedDrivers / DriverInjectedAt：离线 WIM 驱动注入记录（借鉴项 1/5）。
	// InjectedDrivers 是已注入驱动包名的 JSON 数组（见 Get/Set 与 Marshal/Unmarshal）。
	InjectedDrivers  string     `json:"-" gorm:"type:text"` // JSON: ["nic-intel","raid-lsi"]
	DriverInjectedAt *time.Time `json:"driver_injected_at,omitempty"`
	// DriverInjectCount 累计离线注入次数（叠加注入会累积；可经 restore-wim 还原并清零）。
	DriverInjectCount int       `json:"driver_inject_count"`
	FilePath          string    `json:"file_path" gorm:"-"` // ISO 在服务器上的完整路径（响应时计算，不落库）
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (o *OSImage) GetInjectedDrivers() ([]string, error) {
	if o.InjectedDrivers == "" {
		return nil, nil
	}
	var packs []string
	if err := json.Unmarshal([]byte(o.InjectedDrivers), &packs); err != nil {
		return nil, err
	}
	return packs, nil
}

func (o *OSImage) SetInjectedDrivers(packs []string) error {
	if packs == nil {
		o.InjectedDrivers = ""
		return nil
	}
	data, err := json.Marshal(packs)
	if err != nil {
		return err
	}
	o.InjectedDrivers = string(data)
	return nil
}

// MarshalJSON 把 InjectedDrivers 还原为 injected_drivers 数组暴露给前端
// （与 Host.DriverPacks 的做法一致）。
func (o *OSImage) MarshalJSON() ([]byte, error) {
	type Alias OSImage
	packs, _ := o.GetInjectedDrivers()
	if packs == nil {
		packs = []string{}
	}
	return json.Marshal(&struct {
		*Alias
		InjectedDrivers []string `json:"injected_drivers"`
	}{
		Alias:           (*Alias)(o),
		InjectedDrivers: packs,
	})
}

// UnmarshalJSON 支持请求体里的 injected_drivers 数组。
func (o *OSImage) UnmarshalJSON(data []byte) error {
	type Alias OSImage
	aux := &struct {
		InjectedDrivers []string `json:"injected_drivers"`
		*Alias
	}{Alias: (*Alias)(o)}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	return o.SetInjectedDrivers(aux.InjectedDrivers)
}
