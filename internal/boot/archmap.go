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

// DefaultArchMap 返回内置默认的架构→启动文件映射表（纯默认值，不含用户覆盖）。
func DefaultArchMap() map[int]config.ArchEntry {
	return defaultArchMap()
}

// defaultArchMap 返回内置默认的架构→启动文件映射表。
// 当用户没有在 config.yaml 中配置 arch_map 时使用。
// 默认所有架构使用 iPXE，NBP 类型决定使用哪种引导加载器。
// 完整支持 iPXE v2.0.0 发布包中的所有架构。
func defaultArchMap() map[int]config.ArchEntry {
	return map[int]config.ArchEntry{
		// BIOS 架构
		int(iana.INTEL_X86PC): {NBP: "ipxe", IPXE: "ipxe.pxe", PXELinux: "pxelinux.bios"},

		// EFI 架构
		int(iana.EFI_IA32):    {NBP: "ipxe", IPXE: "ipxe32.efi", PXELinux: "pxelinux32.efi"},
		int(iana.EFI_X86_64):  {NBP: "ipxe", IPXE: "ipxe.efi", PXELinux: "pxelinux.efi", GRUB: "grubx64.efi", SecureBoot: false, IPXESB: "ipxe-x86_64-sb.efi", ShimIPXE: "shim-x86_64.efi", GRUBSB: "grubx64.efi", ShimGRUB: "shimx64.efi"},
		int(iana.EFI_BC):      {NBP: "ipxe", IPXE: "snponly.efi", GRUB: "grubx64.efi"},
		int(iana.EFI_ARM32):   {NBP: "ipxe", IPXE: "ipxe-arm32.efi"},
		int(iana.EFI_ARM64):   {NBP: "ipxe", IPXE: "ipxe-arm64.efi", GRUB: "grubaa64.efi", SecureBoot: false, IPXESB: "ipxe-arm64-sb.efi", ShimIPXE: "shim-arm64.efi", GRUBSB: "grubaa64.efi", ShimGRUB: "shim-arm64.efi"},
		int(iana.EFI_RISCV32): {NBP: "ipxe", IPXE: "ipxe-riscv32.efi"},
		int(iana.EFI_RISCV64): {NBP: "ipxe", IPXE: "ipxe-riscv64.efi"},

		// LoongArch 架构（insomniacslk/dhcp 库未定义，使用自定义常量）
		int(EFI_LOONGARCH32): {NBP: "ipxe", IPXE: "ipxe-loong64.efi"}, // LoongArch32 使用 64 位二进制
		int(EFI_LOONGARCH64): {NBP: "ipxe", IPXE: "ipxe-loong64.efi"},
	}
}

// archNameToCodes 将 dhcp.ArchString 输出的架构名反查回 IANA 架构码。
// 租约中只保存架构字符串（见 dhcp.ArchAndPlatform），HTTP 侧判断时需要反查。
// 一对多的情况（x86_64 同时覆盖 EFI_X86_64/EFI_BC，loong64 覆盖 LoongArch32/64）
// 保留全部候选码，由 ChainLoadForArch 逐一枚举。
var archNameToCodes = map[string][]int{
	"x86":     {int(iana.INTEL_X86PC)},
	"i386":    {int(iana.EFI_IA32)},
	"x86_64":  {int(iana.EFI_X86_64), int(iana.EFI_BC)},
	"arm32":   {int(iana.EFI_ARM32)},
	"arm64":   {int(iana.EFI_ARM64)},
	"riscv32": {int(iana.EFI_RISCV32)},
	"riscv64": {int(iana.EFI_RISCV64)},
	"loong64": {int(EFI_LOONGARCH32), int(EFI_LOONGARCH64)},
}

// ChainLoadForArch 判断指定架构（dhcp.ArchString 输出的架构名）在 ArchMap 中
// 是否配置了「NBP=nbpType 且 ChainLoad=true」。
// nbpType 为 "pxelinux" 或 "grub2"。
// 架构名未知、查无条目或未开启 ChainLoad 时返回 false。
// 一个架构名对应多个候选码时，任一条目命中即返回 true。
func ChainLoadForArch(archName, nbpType string) bool {
	codes, ok := archNameToCodes[archName]
	if !ok {
		return false
	}
	archMap := GetArchMap()
	for _, code := range codes {
		if entry, ok := archMap[code]; ok && entry.NBP == nbpType && entry.ChainLoad {
			return true
		}
	}
	return false
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
