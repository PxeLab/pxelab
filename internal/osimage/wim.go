package osimage

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ErrWIMUnsupported 非 Windows 平台不支持离线 WIM 操作：Linux 上 wimlib
// 无法注册 Windows driver store，真正的离线驱动注入需要 Windows 主机上的
// dism。非 Windows 主机请改用 PE 阶段运行时驱动加载（winpeshl install.bat
// 注入，见 internal/api/driver_packages.go 的 buildPeDriverInstallBlock）。
var ErrWIMUnsupported = fmt.Errorf("wim: offline WIM operations require a Windows host (dism); use PE-stage runtime driver loading on other platforms")

// WIMIndex 描述 WIM 内的一个镜像索引（GetWIMInfo 的返回项）。
type WIMIndex struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Size        int64  `json:"size"`
}

// WIMDriver 描述离线镜像中已注入的第三方（OEM）驱动（ListWIMDrivers 的返回项，
// 用于注入结果验证与 UI 展示）。
type WIMDriver struct {
	ProviderName string `json:"provider_name"`
	ClassName    string `json:"class_name"`
	Driver       string `json:"driver"` // 如 oem3.inf
	Version      string `json:"version"`
	BootCritical bool   `json:"boot_critical"`
}

// 平台实现（wim_windows.go / wim_other.go）：
//
//	MountWIM(wimPath, mountDir string, index int) error
//	AddDriverToWIM(mountDir, driverDir string, recurse bool) error
//	UnmountWIM(mountDir string, commit bool) error
//	ListWIMDrivers(mountDir string) ([]WIMDriver, error)
//	CleanupWIM() error
//	GetWIMInfo(wimPath string) ([]WIMIndex, error)

// psWindowsImage 对应 Get-WindowsImage | Select-Object ImageIndex,ImageName,
// ImageDescription,ImageSize | ConvertTo-Json 的输出（ConvertTo-Json 单对象时
// 不产生数组，parseWIMIndexes 做了兼容）。
type psWindowsImage struct {
	ImageIndex       int    `json:"ImageIndex"`
	ImageName        string `json:"ImageName"`
	ImageDescription string `json:"ImageDescription"`
	ImageSize        int64  `json:"ImageSize"`
}

// psWindowsDriver 对应 Get-WindowsDriver | Select-Object ProviderName,ClassName,
// Driver,Version,BootCritical | ConvertTo-Json 的输出。
type psWindowsDriver struct {
	ProviderName string `json:"ProviderName"`
	ClassName    string `json:"ClassName"`
	Driver       string `json:"Driver"`
	Version      string `json:"Version"`
	BootCritical bool   `json:"BootCritical"`
}

// parseWIMIndexes 解析 Get-WindowsImage 的 ConvertTo-Json 输出为索引列表。
// 空输出（WIM 无镜像）返回空切片；单对象输出兼容为单元素列表。
func parseWIMIndexes(out string) ([]WIMIndex, error) {
	s := strings.TrimSpace(out)
	if s == "" {
		return nil, nil
	}
	var raw []psWindowsImage
	if err := json.Unmarshal([]byte(s), &raw); err == nil {
		return wimIndexesFrom(raw), nil
	}
	var one psWindowsImage
	if err := json.Unmarshal([]byte(s), &one); err == nil {
		return wimIndexesFrom([]psWindowsImage{one}), nil
	}
	return nil, fmt.Errorf("wim: cannot parse Get-WindowsImage output: %.200s", s)
}

func wimIndexesFrom(raw []psWindowsImage) []WIMIndex {
	out := make([]WIMIndex, 0, len(raw))
	for _, img := range raw {
		out = append(out, WIMIndex{
			Index:       img.ImageIndex,
			Name:        img.ImageName,
			Description: img.ImageDescription,
			Size:        img.ImageSize,
		})
	}
	return out
}

// parseWIMDrivers 解析 Get-WindowsDriver 的 ConvertTo-Json 输出为驱动列表。
// 空输出（镜像内无第三方驱动）返回空切片；单对象输出兼容为单元素列表。
func parseWIMDrivers(out string) ([]WIMDriver, error) {
	s := strings.TrimSpace(out)
	if s == "" {
		return nil, nil
	}
	var raw []psWindowsDriver
	if err := json.Unmarshal([]byte(s), &raw); err == nil {
		return wimDriversFrom(raw), nil
	}
	var one psWindowsDriver
	if err := json.Unmarshal([]byte(s), &one); err == nil {
		return wimDriversFrom([]psWindowsDriver{one}), nil
	}
	return nil, fmt.Errorf("wim: cannot parse Get-WindowsDriver output: %.200s", s)
}

func wimDriversFrom(raw []psWindowsDriver) []WIMDriver {
	out := make([]WIMDriver, 0, len(raw))
	for _, d := range raw {
		out = append(out, WIMDriver{
			ProviderName: d.ProviderName,
			ClassName:    d.ClassName,
			Driver:       d.Driver,
			Version:      d.Version,
			BootCritical: d.BootCritical,
		})
	}
	return out
}
