package boot

import "github.com/insomniacslk/dhcp/iana"

// NBPFilename 根据架构和引导加载器类型返回 NBP 文件名
// bootloader: "ipxe"（默认）, "pxelinux", "grub2"
func NBPFilename(arch iana.Arch, bootloader string) string {
	switch bootloader {
	case "pxelinux":
		return pxelinuxFile(arch)
	case "grub2":
		return grubFile(arch)
	default:
		return BootFileForArch(arch)
	}
}

func pxelinuxFile(arch iana.Arch) string {
	switch arch {
	case iana.INTEL_X86PC:
		return "pxelinux.bios"
	case iana.EFI_IA32, iana.EFI_X86_64, iana.EFI_BC:
		return "pxelinux.efi"
	default:
		return "pxelinux.efi"
	}
}

func grubFile(arch iana.Arch) string {
	switch arch {
	case iana.EFI_X86_64, iana.EFI_BC:
		return "grubx64.efi"
	case iana.EFI_ARM64:
		return "grubaa64.efi"
	default:
		// GRUB2 不支持 BIOS，返回空字符串
		return ""
	}
}
