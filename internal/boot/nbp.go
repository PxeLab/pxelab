package boot

import "github.com/insomniacslk/dhcp/iana"

// NBPFilename 根据架构和引导加载器类型返回 NBP 文件名
// bootloader: "ipxe"（默认）, "pxelinux", "grub2", "undionly"
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


