package dhcp

import (
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

// DetectClientArch 从 DHCP 包读取架构类型
func DetectClientArch(pkt *dhcpv4.DHCPv4) (iana.Arch, bool) {
	if pkt == nil {
		return 0, false
	}
	// 读取 Option 93
	archs := pkt.ClientArch()
	if len(archs) > 0 {
		return archs[0], true
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

// BuildIPXEScriptOption 构建 iPXE Option 175 子选项 178（引导脚本 URL）
func BuildIPXEScriptOption(url string) dhcpv4.Option {
	// 子选项格式: [178(1)][len(1)][url(n)]
	data := make([]byte, 0, 2+len(url))
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
