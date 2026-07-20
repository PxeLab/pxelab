package boot

import "github.com/insomniacslk/dhcp/iana"

// LoongArch IANA 架构常量
// insomniacslk/dhcp 库尚未包含这些常量，需要手动定义
// 参考：https://www.iana.org/assignments/dhcpv6-parameters/dhcpv6-parameters.xhtml#processor-architecture
//       https://github.com/ipxe/ipxe/blob/master/src/include/ipxe/dhcp.h
const (
	EFI_LOONGARCH32 iana.Arch = 0x25 // 37
	EFI_LOONGARCH64 iana.Arch = 0x27 // 39
)

// ArchName 返回 IANA 架构编号对应的可读名称。
// 覆盖 iana.Arch.String() 以支持 LoongArch 等库未定义的架构。
func ArchName(code int) string {
	switch iana.Arch(code) {
	case EFI_LOONGARCH32:
		return "EFI_LOONGARCH32"
	case EFI_LOONGARCH64:
		return "EFI_LOONGARCH64"
	default:
		return iana.Arch(code).String()
	}
}
