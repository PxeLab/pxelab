package boot

import (
	"testing"

	"github.com/insomniacslk/dhcp/iana"
)

func TestBootFileForArch(t *testing.T) {
	tests := []struct {
		arch iana.Arch
		want string
	}{
		{iana.INTEL_X86PC, "undionly.kpxe"},
		{iana.EFI_IA32, "ipxe32.efi"},
		{iana.EFI_X86_64, "ipxe.efi"},
		{iana.EFI_BC, "ipxe.efi"},
		{iana.EFI_ARM64, "ipxe-arm64.efi"},
		{iana.EFI_RISCV64, "ipxe-riscv64.efi"},
		{iana.Arch(255), "undionly.kpxe"}, // unknown → default
	}

	for _, tc := range tests {
		got := BootFileForArch(tc.arch)
		if got != tc.want {
			t.Errorf("BootFileForArch(%d) = %s; want %s", tc.arch, got, tc.want)
		}
	}
}
