package boot

import "github.com/insomniacslk/dhcp/iana"

// BootFileForArch 根据 PXE 客户端架构返回引导文件
func BootFileForArch(arch iana.Arch) string {
	switch arch {
	case iana.INTEL_X86PC:
		return "undionly.kpxe"
	case iana.EFI_IA32:
		return "ipxe32.efi"
	case iana.EFI_X86_64, iana.EFI_BC:
		return "ipxe.efi"
	case iana.EFI_ARM64:
		return "ipxe-arm64.efi"
	case iana.EFI_RISCV64:
		return "ipxe-riscv64.efi"
	default:
		return "undionly.kpxe"
	}
}
