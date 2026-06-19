package boot

import "fmt"

// iPXEBinaryForArch returns the iPXE boot filename for the given architecture.
func iPXEBinaryForArch(arch, platform string) string {
	switch {
	case platform == "efi" && arch == "x86_64":
		return "ipxe.efi"
	case platform == "efi" && arch == "i386":
		return "ipxe32.efi"
	case platform == "efi" && arch == "arm64":
		return "ipxe-arm64.efi"
	default:
		// BIOS/legacy or fallback
		return "undionly.kpxe"
	}
}

// PXELinuxChainloadConfig generates a pxelinux config that chain-loads to iPXE.
func PXELinuxChainloadConfig(serverAddr, arch, platform string) string {
	ipxeBin := iPXEBinaryForArch(arch, platform)
	return fmt.Sprintf(`DEFAULT ipxe
LABEL ipxe
  KERNEL http://%s/boot/%s
`, serverAddr, ipxeBin)
}

// GRUB2ChainloadConfig generates a grub2 config that chain-loads to iPXE.
// GRUB2 supports conditional branching, so a single config handles all archs.
func GRUB2ChainloadConfig(serverAddr string) string {
	return fmt.Sprintf(`set net_default_server=%s
set prefix=(http)/boot

if [ ${grub_cpu} = x86_64 -a ${grub_platform} = efi ]; then
  chainloader (http)/boot/ipxe.efi
elif [ ${grub_cpu} = i386 -a ${grub_platform} = efi ]; then
  chainloader (http)/boot/ipxe32.efi
elif [ ${grub_cpu} = aarch64 ]; then
  chainloader (http)/boot/ipxe-arm64.efi
else
  chainloader (http)/boot/ipxe.efi
fi
boot
`, serverAddr)
}
