package boot

import "github.com/insomniacslk/dhcp/iana"

// BootFileForArch returns the boot file name for the given client architecture.
func BootFileForArch(arch iana.Arch) string {
	switch arch {
	case iana.INTEL_X86PC, iana.INTEL_LEAN_CLIENT:
		return "undionly.kpxe"
	case iana.EFI_IA32:
		return "ipxe32.efi"
	case iana.EFI_X86_64:
		return "ipxe.efi"
	case iana.EFI_BC:
		return "ipxe.efi"
	case iana.EFI_ARM32:
		return "ipxe.arm32.efi"
	case iana.EFI_ARM64:
		return "ipxe.arm64.efi"
	default:
		return "undionly.kpxe"
	}
}
