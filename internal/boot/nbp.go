package boot

import (
	"github.com/insomniacslk/dhcp/iana"
	"github.com/pxelab/pxelab/internal/config"
)

// ResolveNBP 根据客户端架构和 ArchEntry 配置，解析最终的引导文件和链式加载目标。
// 返回值：
//   - bootFile: DHCP 响应中的引导文件名
//   - chainLoadTarget: 如果需要链式加载，返回 iPXE 文件名；否则为空
//
// 决策逻辑：
//  1. Secure Boot 开启且有 shim → 返回 shim，由 shim 自动链到对应二进制
//  2. NBP="ipxe" → 直接返回 IPXE 文件，无链式加载
//  3. NBP="pxelinux" 且 ChainLoad=true → 返回 PXELinux 文件，chainLoadTarget=IPXE 文件
//  4. NBP="pxelinux" 且 ChainLoad=false → 仅返回 PXELinux 文件
//  5. NBP="grub2" 同理
//
// Secure Boot 说明：
// DHCP 无法检测客户端是否开启 Secure Boot（没有对应 DHCP option），
// 因此采用行业标准做法：当架构配置了 Secure Boot 时，始终下发 shim 作为引导文件。
// shim 在 Secure Boot 关闭时也能正常工作（直接透传），因此不会影响非 SB 客户端。
func ResolveNBP(arch iana.Arch, entry config.ArchEntry) (bootFile string, chainLoadTarget string) {
	nbp := entry.NBP
	if nbp == "" {
		nbp = "ipxe" // 默认 iPXE
	}

	// Secure Boot: 根据 NBP 类型选择对应的 shim，由 shim 自动加载对应的签名二进制
	// shim 在 Secure Boot 关闭时也能正常工作（直接透传）
	// 仅 x86_64 (7) 和 ARM64 (11) 支持 Secure Boot
	if entry.SecureBoot && (arch == iana.EFI_X86_64 || arch == iana.EFI_ARM64) {
		switch nbp {
		case "grub2":
			if entry.ShimGRUB != "" {
				return entry.ShimGRUB, ""
			}
		default: // "ipxe" 或未知类型
			if entry.ShimIPXE != "" {
				return entry.ShimIPXE, ""
			}
		}
		// pxelinux 不支持 Secure Boot（无 shim），继续正常流程
	}

	switch nbp {
	case "pxelinux":
		bootFile = entry.PXELinux
		if bootFile == "" {
			bootFile = pxelinuxFile(arch) // 从内置默认值获取
		}
		if entry.ChainLoad && entry.IPXE != "" {
			chainLoadTarget = entry.IPXE
		}
	case "grub2":
		bootFile = entry.GRUB
		if bootFile == "" {
			bootFile = grubFile(arch) // 从内置默认值获取
		}
		if entry.ChainLoad && entry.IPXE != "" {
			chainLoadTarget = entry.IPXE
		}
	default: // "ipxe" 或未知类型
		bootFile = entry.IPXE
		if bootFile == "" {
			bootFile = BootFileForArch(arch) // 从内置默认值获取
		}
	}

	return bootFile, chainLoadTarget
}

// NBPFilename 根据架构和引导加载器类型返回 NBP 文件名（兼容旧接口）
// bootloader: "ipxe"（默认）, "pxelinux", "grub2", "undionly"
//
// Deprecated: 新代码应使用 ResolveNBP()。
func NBPFilename(arch iana.Arch, bootloader string) string {
	switch bootloader {
	case "pxelinux":
		return pxelinuxFile(arch)
	case "grub2":
		return grubFile(arch)
	case "undionly":
		if arch == iana.INTEL_X86PC {
			return "undionly.kpxe"
		}
		return BootFileForArch(arch)
	default:
		return BootFileForArch(arch)
	}
}

// pxelinuxFile 从 ArchMap 读取 PXELinux 文件名。
// 当该架构没有 PXELinux 文件时（如 ARM64），尝试回退到 GRUB2；
// 都没有时返回空字符串，DHCP 不下发启动文件。
func pxelinuxFile(arch iana.Arch) string {
	if entry, ok := archEntry(arch); ok && entry.PXELinux != "" {
		return entry.PXELinux
	}
	// 没有 PXELinux 时尝试 GRUB fallback（如 ARM64 → grubaa64.efi）
	if entry, ok := archEntry(arch); ok && entry.GRUB != "" {
		return entry.GRUB
	}
	switch arch {
	case iana.INTEL_X86PC:
		return "pxelinux.bios"
	case iana.EFI_IA32, iana.EFI_X86_64:
		return "pxelinux.efi"
	default:
		return ""
	}
}

// grubFile 从 ArchMap 读取 GRUB2 文件名，缺失时回退到内置默认值。
func grubFile(arch iana.Arch) string {
	if entry, ok := archEntry(arch); ok && entry.GRUB != "" {
		return entry.GRUB
	}
	switch arch {
	case iana.EFI_X86_64, iana.EFI_BC:
		return "grubx64.efi"
	case iana.EFI_ARM64:
		return "grubaa64.efi"
	default:
		return ""
	}
}


