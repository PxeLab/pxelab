package httpd

import (
	"testing"

	"github.com/insomniacslk/dhcp/iana"
	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
)

// chainTestConfig 构造 chainToIPXEFallback 测试用的最小配置：
// 一个接口 + 开启 ChainToIPXE 的子网 10.0.0.0/24。
func chainTestConfig() *config.Config {
	return &config.Config{
		Boot: config.BootConfig{
			PXEConfigFile:  "pxelinux.cfg/default",
			GRUBConfigFile: "grub.cfg",
		},
		Interfaces: []config.InterfaceConfig{
			{
				Subnets: []config.SubnetConfig{
					{CIDR: "10.0.0.0/24", ChainToIPXE: true},
				},
			},
		},
	}
}

func TestChainToIPXEFallback(t *testing.T) {
	cfg := chainTestConfig()

	// 架构级测试数据：x86_64 配置 NBP=pxelinux + ChainLoad=true
	t.Cleanup(func() { boot.InitArchMap(nil) })
	boot.InitArchMap(map[int]config.ArchEntry{
		int(iana.EFI_X86_64): {NBP: "pxelinux", ChainLoad: true, PXELinux: "pxelinux.efi", IPXE: "ipxe.efi"},
	})

	tests := []struct {
		name     string
		filePath string
		clientIP string
		archName string
		want     bool
	}{
		{
			name:     "命中接口级：子网 ChainToIPXE + grub 配置文件",
			filePath: "grub.cfg",
			clientIP: "10.0.0.50",
			archName: "x86_64",
			want:     true,
		},
		{
			name:     "命中接口级：子网 ChainToIPXE + pxelinux 配置文件（不再要求类型匹配）",
			filePath: "pxelinux.cfg/default",
			clientIP: "10.0.0.50",
			archName: "arm64",
			want:     true,
		},
		{
			name:     "命中架构级：ArchMap chain_load=true（子网外 IP 也生效）",
			filePath: "pxelinux.cfg/default",
			clientIP: "192.168.1.50",
			archName: "x86_64",
			want:     true,
		},
		{
			name:     "架构级 NBP 不匹配：grub 配置文件但条目 NBP=pxelinux",
			filePath: "grub.cfg",
			clientIP: "192.168.1.50",
			archName: "x86_64",
			want:     false,
		},
		{
			name:     "都不命中：arm64 默认 NBP=ipxe，且 IP 在子网外",
			filePath: "pxelinux.cfg/default",
			clientIP: "192.168.1.50",
			archName: "arm64",
			want:     false,
		},
		{
			name:     "架构未知：不在反查表中的架构名不 chain",
			filePath: "pxelinux.cfg/default",
			clientIP: "192.168.1.50",
			archName: "sparc",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := chainToIPXEFallback(cfg, tc.filePath, tc.clientIP, tc.archName); got != tc.want {
				t.Errorf("chainToIPXEFallback(%q, %q, %q) = %v; want %v",
					tc.filePath, tc.clientIP, tc.archName, got, tc.want)
			}
		})
	}
}
