package dhcp

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

// BuildPXEOptions 构建 PXE 相关的 DHCP 选项
func BuildPXEOptions(arch iana.Arch, nextServer string, bootFile string, menuData []byte) []dhcpv4.Option {
	options := []dhcpv4.Option{
		dhcpv4.OptTFTPServerName(nextServer),
		dhcpv4.OptBootFileName(bootFile),
	}

	// Option 43 - Vendor Specific Information
	// PXE sub-option 6: Discovery Control
	discoveryCtrl := []byte{6, 1, 0x08}
	_ = discoveryCtrl

	// PXE sub-option 8: Boot Menu
	if menuData != nil {
		bootMenu := buildSubOption(8, menuData)
		option43 := dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation, bootMenu)
		options = append(options, option43)
	}

	return options
}

func buildSubOption(subOpt byte, data []byte) []byte {
	// 子选项编码：子选项代码(1)+长度(1)+数据(n)
	length := len(data)
	result := make([]byte, 0, 2+length)
	result = append(result, subOpt)
	result = append(result, byte(length))
	result = append(result, data...)
	return result
}

// DetectClientArch 从 DHCP 包读取架构类型（Option 93 → Option 60 兜底）
func DetectClientArch(pkt *dhcpv4.DHCPv4) (iana.Arch, bool) {
	if pkt == nil {
		return 0, false
	}
	archs := pkt.ClientArch()
	if len(archs) > 0 {
		return archs[0], true
	}
	// 兜底：从 Option 60 解析 "PXEClient:Arch:xxxxx:UNDI:..." 中的架构码
	vci := pkt.ClassIdentifier()
	if idx := strings.Index(vci, ":Arch:"); idx >= 0 {
		rest := vci[idx+6:]
		if end := strings.IndexByte(rest, ':'); end == 5 {
			var arch uint16
			if _, err := fmt.Sscanf(rest[:5], "%x", &arch); err == nil {
				slog.Debug("从 Option 60 兜底解析架构", "service", "DHCP", "vci", vci, "arch", arch)
				return iana.Arch(arch), true
			}
		}
	}
	return 0, false
}

// IsPXEClient 检测客户端是否为 PXE 客户端（Option 60 以 "PXEClient" 开头）
// Intel PXE ROM 会发送 "PXEClient:Arch:xxxxx:UNDI:xxxxx" 格式
func IsPXEClient(pkt *dhcpv4.DHCPv4) bool {
	if pkt == nil {
		return false
	}
	return strings.HasPrefix(pkt.ClassIdentifier(), "PXEClient")
}

// BuildIPXEScriptOption 构建 iPXE Option 175 子选项（177=特征标志 + 178=脚本 URL）
func BuildIPXEScriptOption(url string) dhcpv4.Option {
	// 子选项 177: 特征标志, 值 0x01 = BootFileName 走 HTTP 而非 TFTP
	// 子选项 178: 引导脚本 URL
	data := make([]byte, 0, 6+len(url))
	data = append(data, 177, 1, 0x01)  // feature flags: HTTP
	data = append(data, 178, byte(len(url)))
	data = append(data, []byte(url)...)
	return dhcpv4.OptGeneric(dhcpv4.GenericOptionCode(175), data)
}

// IsIPXEClient 检测客户端是否已经是 iPXE 在运行（Option 60 精确等于 "iPXE"）
func IsIPXEClient(pkt *dhcpv4.DHCPv4) bool {
	if pkt == nil {
		return false
	}
	return pkt.ClassIdentifier() == "iPXE"
}

// ArchString 将 PXE 架构枚举转为 iPXE 兼容的架构字符串
func ArchString(arch iana.Arch) string {
	switch arch {
	case iana.INTEL_X86PC:
		return "x86"
	case iana.EFI_IA32:
		return "i386"
	case iana.EFI_X86_64, iana.EFI_BC:
		return "x86_64"
	case iana.EFI_ARM64:
		return "arm64"
	case iana.EFI_RISCV64:
		return "riscv64"
	default:
		return "x86"
	}
}

// PlatformString 根据 PXE 架构返回启动平台类型
func PlatformString(arch iana.Arch) string {
	switch arch {
	case iana.INTEL_X86PC:
		return "pc" // legacy BIOS
	default:
		return "efi"
	}
}

// isUEFIArch 判断架构是否为 UEFI
func isUEFIArch(arch iana.Arch) bool {
	switch arch {
	case iana.INTEL_X86PC:
		return false
	default:
		return true
	}
}

// ArchAndPlatform 返回 (arch, platform) 字符串对
func ArchAndPlatform(pkt *dhcpv4.DHCPv4) (string, string) {
	arch, ok := DetectClientArch(pkt)
	if !ok {
		return "x86", "pc"
	}
	return ArchString(arch), PlatformString(arch)
}

