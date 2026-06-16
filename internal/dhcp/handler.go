package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

type Handler struct {
	config   *config.Config
	store    store.Interface
	leaseMgr *LeaseManager
	eventBus *eventbus.Bus
}

func NewHandler(cfg *config.Config, st store.Interface, bus *eventbus.Bus, leaseMgr *LeaseManager) *Handler {
	return &Handler{
		config:   cfg,
		store:    st,
		leaseMgr: leaseMgr,
		eventBus: bus,
	}
}

func (h *Handler) InitSubnets() {
	for _, iface := range h.config.Interfaces {
		for _, subnet := range iface.Subnets {
			var pools []*IPRange

			// 优先使用多地址池 Pools 字段
			if len(subnet.Pools) > 0 {
				for _, p := range subnet.Pools {
					poolParts := strings.SplitN(p, "-", 2)
					if len(poolParts) != 2 {
						slog.Warn("地址池格式无效，跳过", "pool", p)
						continue
					}
					ipRange, err := NewIPRange(strings.TrimSpace(poolParts[0]), strings.TrimSpace(poolParts[1]))
					if err != nil {
						slog.Warn("创建 IP 范围失败，跳过", "pool", p, "error", err)
						continue
					}
					pools = append(pools, ipRange)
				}
			} else if subnet.Pool != "" {
				// 兼容旧格式：单个地址池字符串
				poolParts := strings.SplitN(subnet.Pool, "-", 2)
				if len(poolParts) != 2 {
					slog.Warn("地址池格式无效，跳过", "cidr", subnet.CIDR, "pool", subnet.Pool)
					continue
				}
				ipRange, err := NewIPRange(strings.TrimSpace(poolParts[0]), strings.TrimSpace(poolParts[1]))
				if err != nil {
					slog.Warn("创建 IP 范围失败，跳过", "cidr", subnet.CIDR, "error", err)
					continue
				}
				pools = append(pools, ipRange)
			}

			if len(pools) == 0 {
				slog.Warn("子网无有效地址池，跳过", "cidr", subnet.CIDR)
				continue
			}

			gateway := net.ParseIP(subnet.Gateway)
			if gateway == nil {
				slog.Warn("网关地址无效，跳过", "cidr", subnet.CIDR, "gateway", subnet.Gateway)
				continue
			}
			h.leaseMgr.AddSubnet(subnet.CIDR, pools, gateway)
			slog.Info("子网已注册", "cidr", subnet.CIDR, "pools", subnet.Pools)
		}
	}
}

// ReloadSubnets 清空并重新加载子网配置，用于热重载
func (h *Handler) ReloadSubnets() {
	slog.Info("重载 DHCP 子网配置")
	h.leaseMgr.ClearSubnets()
	h.InitSubnets()
}

func (h *Handler) Handle(ctx context.Context, conn net.PacketConn, peer net.Addr, pkt *dhcpv4.DHCPv4) {
	if pkt == nil {
		return
	}
	mt := pkt.MessageType()
	if mt != dhcpv4.MessageTypeDiscover && mt != dhcpv4.MessageTypeRequest {
		return
	}

	mac := pkt.ClientHWAddr.String()
	isPXE := IsPXEClient(pkt)

	slog.Debug("收到 DHCP 包",
		"mac", mac,
		"type", mt,
		"isPXE", isPXE,
	)

	msgType := "DHCP"
	if isPXE {
		msgType = "PXE"
	}

	h.eventBus.Publish("event", models.Event{
		Type:    models.EventDHCP,
		Level:   models.EventInfo,
		Message: fmt.Sprintf("%s 请求: %s", msgType, mac),
		MAC:     &mac,
	})

	// 确定 DHCP 模式
	dhcpMode := "hybrid"
	for _, iface := range h.config.Interfaces {
		if isPXE && (iface.DHCP == "proxy" || iface.DHCP == "hybrid") {
			dhcpMode = "proxy"
		} else if iface.DHCP == "full" {
			dhcpMode = "full"
		}
	}

	// 查找匹配子网（通过 peer IP）
	var subnetCfg *config.SubnetConfig
	for _, iface := range h.config.Interfaces {
		for _, subnet := range iface.Subnets {
			_, cidrNet, err := net.ParseCIDR(subnet.CIDR)
			if err == nil {
				peerIP := peer.(*net.UDPAddr).IP
				if cidrNet.Contains(peerIP) {
					subnetCfg = &subnet
					break
				}
			}
		}
		if subnetCfg != nil {
			break
		}
	}

	var nextServer net.IP
	if subnetCfg != nil && subnetCfg.NextServer != "" {
		nextServer = net.ParseIP(subnetCfg.NextServer)
	}

	var reply *dhcpv4.DHCPv4
	switch mt {
	case dhcpv4.MessageTypeDiscover:
		reply = h.handleDiscover(pkt, dhcpMode, nextServer, subnetCfg)
	case dhcpv4.MessageTypeRequest:
		reply = h.handleRequest(pkt, dhcpMode, nextServer, subnetCfg)
	}

	if reply != nil {
		if nextServer != nil {
			reply.UpdateOption(dhcpv4.OptServerIdentifier(nextServer))
		}
		if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
			slog.Error("发送 DHCP 响应失败", "error", err)
		}
	}
}

func (h *Handler) handleDiscover(pkt *dhcpv4.DHCPv4, mode string, nextServer net.IP, subnetCfg *config.SubnetConfig) *dhcpv4.DHCPv4 {
	reply, err := dhcpv4.NewReplyFromRequest(pkt)
	if err != nil {
		return nil
	}

	switch mode {
	case "proxy":
		reply.YourIPAddr = net.IP{0, 0, 0, 0}
		if nextServer != nil {
			reply.ServerIPAddr = nextServer
		}
		if arch, ok := DetectClientArch(pkt); ok {
			reply.BootFileName = boot.BootFileForArch(arch)
		}

	case "full":
		cidr := ""
		if subnetCfg != nil {
			cidr = subnetCfg.CIDR
		}
		if ip, err := h.leaseMgr.Allocate(cidr, pkt.ClientHWAddr.String()); err == nil {
			reply.YourIPAddr = ip
		} else {
			slog.Warn("IP 分配失败", "mac", pkt.ClientHWAddr.String(), "error", err)
			return nil
		}
		if nextServer != nil {
			reply.ServerIPAddr = nextServer
		}
		if arch, ok := DetectClientArch(pkt); ok {
			reply.BootFileName = boot.BootFileForArch(arch)
		}

	case "hybrid":
		cidr := ""
		if subnetCfg != nil {
			cidr = subnetCfg.CIDR
		}
		if ip, err := h.leaseMgr.Allocate(cidr, pkt.ClientHWAddr.String()); err == nil {
			reply.YourIPAddr = ip
		} else {
			slog.Warn("IP 分配失败", "mac", pkt.ClientHWAddr.String(), "error", err)
			return nil
		}
		if nextServer != nil {
			reply.ServerIPAddr = nextServer
		}
		if arch, ok := DetectClientArch(pkt); ok {
			reply.BootFileName = boot.BootFileForArch(arch)
		}
	}

	return reply
}

func (h *Handler) handleRequest(pkt *dhcpv4.DHCPv4, mode string, nextServer net.IP, subnetCfg *config.SubnetConfig) *dhcpv4.DHCPv4 {
	_ = mode
	_ = subnetCfg
	reply, err := dhcpv4.NewReplyFromRequest(pkt)
	if err != nil {
		return nil
	}
	if nextServer != nil {
		reply.ServerIPAddr = nextServer
	}
	return reply
}
