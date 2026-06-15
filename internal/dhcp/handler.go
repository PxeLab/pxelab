package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"

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

func NewHandler(cfg *config.Config, st store.Interface, bus *eventbus.Bus) *Handler {
	return &Handler{
		config:   cfg,
		store:    st,
		leaseMgr: NewLeaseManager(st),
		eventBus: bus,
	}
}

func (h *Handler) InitSubnets() {
	for _, iface := range h.config.Interfaces {
		for _, subnet := range iface.Subnets {
			r, _ := NewIPRange(subnet.Pool, subnet.Pool)
			_ = r
			// TODO: 解析 pool 字符串为 start-end
		}
	}
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
	if isPXE {
		for _, iface := range h.config.Interfaces {
			if iface.DHCP == "proxy" || iface.DHCP == "hybrid" {
				dhcpMode = "proxy"
			} else if iface.DHCP == "full" {
				dhcpMode = "full"
			}
		}
	}

	// 获取 NextServer
	var nextServer net.IP
	for _, iface := range h.config.Interfaces {
		for _, subnet := range iface.Subnets {
			if subnet.NextServer != "" {
				nextServer = net.ParseIP(subnet.NextServer)
			}
		}
	}

	var reply *dhcpv4.DHCPv4
	switch mt {
	case dhcpv4.MessageTypeDiscover:
		reply = h.handleDiscover(pkt, dhcpMode, nextServer)
	case dhcpv4.MessageTypeRequest:
		reply = h.handleRequest(pkt, dhcpMode, nextServer)
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

func (h *Handler) handleDiscover(pkt *dhcpv4.DHCPv4, mode string, nextServer net.IP) *dhcpv4.DHCPv4 {
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
		if ip, err := h.leaseMgr.Allocate("", pkt.ClientHWAddr.String()); err == nil {
			reply.YourIPAddr = ip
		}
		if nextServer != nil {
			reply.ServerIPAddr = nextServer
		}
	}

	return reply
}

func (h *Handler) handleRequest(pkt *dhcpv4.DHCPv4, mode string, nextServer net.IP) *dhcpv4.DHCPv4 {
	reply, err := dhcpv4.NewReplyFromRequest(pkt)
	if err != nil {
		return nil
	}
	if nextServer != nil {
		reply.ServerIPAddr = nextServer
	}
	return reply
}
