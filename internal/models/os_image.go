package models

import "time"

type OSImage struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Name         string    `json:"name" gorm:"not null"`
	Filename     string    `json:"filename" gorm:"not null"`
	SourcePath   string    `json:"source_path"` // 导入的外部 ISO 绝对路径；空 = 上传托管在 isos/ 下
	Size         int64     `json:"size"`
	Distro       string    `json:"distro"`
	Version      string    `json:"version"`
	Arch         string    `json:"arch"`
	Status       string    `json:"status" gorm:"not null;default:uploading"`
	MountPoint   string    `json:"mount_point"`
	ExtractedTo  string    `json:"extracted_to"`
	KernelPath   string    `json:"kernel_path"` // 相对挂载点的 kernel 路径（自动探测）
	InitrdPath   string    `json:"initrd_path"` // 相对挂载点的 initrd 路径（自动探测）
	Checksum     string    `json:"checksum"`
	ErrorMessage string    `json:"error_message"`
	FilePath     string    `json:"file_path" gorm:"-"` // ISO 在服务器上的完整路径（响应时计算，不落库）
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
