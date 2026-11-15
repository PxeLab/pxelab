package dhcp

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/pxelab/pxelab/internal/config"
)

// pxeSubOptions 解析序列化后的 DHCP Option 43（PXE 厂商特定信息）。
func pxeSubOptions(t *testing.T, pkt *dhcpv4.DHCPv4) map[byte][]byte {
	t.Helper()

	parsed, err := dhcpv4.FromBytes(pkt.ToBytes())
	if err != nil {
		t.Fatalf("解析 DHCP 回复失败: %v", err)
	}
	data := parsed.GetOneOption(dhcpv4.OptionVendorSpecificInformation)
	if len(data) == 0 {
		t.Fatal("回复缺少 Option 43")
	}

	subs := make(map[byte][]byte)
	for i := 0; i < len(data); {
		if data[i] == 255 { // PXE_END
			break
		}
		if i+1 >= len(data) {
			t.Fatalf("Option 43 子选项被截断: % x", data)
		}
		code, length := data[i], int(data[i+1])
		if i+2+length > len(data) {
			t.Fatalf("Option 43 子选项 %d 长度越界: % x", code, data)
		}
		subs[code] = data[i+2 : i+2+length]
		i += 2 + length
	}
	return subs
}

// PXE 2.1 Table 2-1：Discovery Control（Option 43 子选项 6）
//   - bit 2 (0x04)：只接受 PXE_BOOT_SERVERS（子选项 8）中列出的服务器；
//   - bit 3 (0x08)：offer 中带 bootfile 时直接下载，不进行 boot server 发现。
//
// 置了 bit 2 却没有子选项 8，是一个自相矛盾的约束。PxeLab 直接通过 TFTP
// 提供 NBP，不提供 boot server 列表，因此这里只应置 bit 3。
func TestPXEDiscoveryControlMatchesBootServerList(t *testing.T) {
	mac, err := net.ParseMAC("00:11:22:33:44:55")
	if err != nil {
		t.Fatal(err)
	}
	serverIP := net.ParseIP("10.0.0.10")
	subnet := &config.SubnetConfig{CIDR: "10.0.0.0/24", Gateway: "10.0.0.1", LeaseTime: 3600}

	tests := []struct {
		name  string
		apply func(*dhcpv4.DHCPv4)
	}{
		{
			name: "proxy DHCP",
			apply: func(reply *dhcpv4.DHCPv4) {
				appendProxyPXEOptions(reply, serverIP, serverIP)
			},
		},
		{
			name: "full DHCP server",
			apply: func(reply *dhcpv4.DHCPv4) {
				appendDHCPOptions(reply, serverIP, serverIP, subnet)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discover, err := dhcpv4.NewDiscovery(mac)
			if err != nil {
				t.Fatal(err)
			}
			reply, err := dhcpv4.NewReplyFromRequest(discover)
			if err != nil {
				t.Fatal(err)
			}
			reply.BootFileName = "ipxe.efi"
			tt.apply(reply)

			subs := pxeSubOptions(t, reply)
			ctrl, ok := subs[6]
			if !ok || len(ctrl) != 1 {
				t.Fatalf("Option 43 子选项 6 (Discovery Control) 缺失或长度不为 1: % x", subs[6])
			}
			if ctrl[0]&0x08 == 0 {
				t.Errorf("Discovery Control = 0x%02x，未置 bit 3；offer 已带 bootfile 时应让客户端直接下载", ctrl[0])
			}
			if ctrl[0]&0x04 != 0 {
				if _, ok := subs[8]; !ok {
					t.Errorf("Discovery Control = 0x%02x 置了 bit 2（只接受 PXE_BOOT_SERVERS 列表中的服务器），但缺少子选项 8", ctrl[0])
				}
			}
		})
	}
}
