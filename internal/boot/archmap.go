package boot

import (
	"github.com/insomniacslk/dhcp/iana"
	"github.com/pxelab/pxelab/internal/config"
)

// globalArchMap 是运行时架构映射表，由 InitArchMap 设置，可在运行时热更新。
// 查找时优先读取此表，缺失的架构回退到硬编码默认值。
var globalArchMap map[int]config.ArchEntry

// InitArchMap 设置全局架构映射表。传入 nil 时清空。
func InitArchMap(m map[int]config.ArchEntry) {
	globalArchMap = m
}

// GetArchMap 返回当前全局架构映射表的副本。
// 未设置时返回内置默认值，以提供完整的可编辑基线。
func GetArchMap() map[int]config.ArchEntry {
	m := make(map[int]config.ArchEntry)
	// 从内置默认值开始
	for code, entry := range defaultArchMap() {
		m[code] = entry
	}
	// 用户配置覆盖默认值
	if globalArchMap != nil {
		for k, v := range globalArchMap {
			m[k] = v
		}
	}
	return m
}

// ArchName 返回 IANA 架构编号对应的可读名称。
func ArchName(code int) string {
	return iana.Arch(code).String()
}

// DefaultArchMap 返回内置默认的架构→启动文件映射表（纯默认值，不含用户覆盖）。
func DefaultArchMap() map[int]config.ArchEntry {
	return defaultArchMap()
}

// defaultArchMap 返回内置默认的架构→启动文件映射表。
// 当用户没有在 config.yaml 中配置 arch_map 时使用。
func defaultArchMap() map[int]config.ArchEntry {
	return map[int]config.ArchEntry{
		int(iana.INTEL_X86PC):       {IPXE: "ipxe.pxe", PXELinux: "pxelinux.bios"},
		int(iana.EFI_IA32):          {IPXE: "ipxe32.efi", PXELinux: "pxelinux32.efi"},
		int(iana.EFI_X86_64):        {IPXE: "ipxe.efi", PXELinux: "pxelinux.efi", GRUB: "grubx64.efi"},
		int(iana.EFI_BC):            {IPXE: "ipxe.efi", GRUB: "grubx64.efi"},
		int(iana.EFI_ARM64):         {IPXE: "ipxe-arm64.efi", GRUB: "grubaa64.efi"},
		int(iana.EFI_RISCV64):       {IPXE: "ipxe-riscv64.efi"},
	}
}

// archEntry 读取指定架构的 ArchEntry，优先从全局映射，缺失时返回零值。
func archEntry(arch iana.Arch) (config.ArchEntry, bool) {
	if globalArchMap != nil {
		if entry, ok := globalArchMap[int(arch)]; ok {
			return entry, true
		}
	}
	return config.ArchEntry{}, false
}

// BootFileForArch 根据 PXE 客户端架构返回默认的 iPXE 引导文件。
// 优先从 ArchMap 读取，缺失时回退到内置默认值。
func BootFileForArch(arch iana.Arch) string {
	if entry, ok := archEntry(arch); ok && entry.IPXE != "" {
		return entry.IPXE
	}
	switch arch {
	case iana.INTEL_X86PC:
		return "ipxe.pxe"
	case iana.EFI_IA32:
		return "ipxe32.efi"
	case iana.EFI_X86_64, iana.EFI_BC:
		return "ipxe.efi"
	case iana.EFI_ARM64:
		return "ipxe-arm64.efi"
	case iana.EFI_RISCV64:
		return "ipxe-riscv64.efi"
	default:
		return "ipxe.pxe"
	}
}
