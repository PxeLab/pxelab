package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

type Handler struct {
	config          *config.Config
	store           store.Interface
	leaseMgr        *LeaseManager
	eventBus        *eventbus.Bus
	interfaceFilter int // -1 = 全部接口, >=0 = 只处理指定接口
}

func NewHandler(cfg *config.Config, st store.Interface, bus *eventbus.Bus, leaseMgr *LeaseManager) *Handler {
	return &Handler{
		config:          cfg,
		store:           st,
		leaseMgr:        leaseMgr,
		eventBus:        bus,
		interfaceFilter: -1,
	}
}

// WithInterfaceFilter 创建一个只处理指定接口的派生 Handler
func (h *Handler) WithInterfaceFilter(idx int) *Handler {
	return &Handler{
		config:          h.config,
		store:           h.store,
		leaseMgr:        h.leaseMgr,
		eventBus:        h.eventBus,
		interfaceFilter: idx,
	}
}

func (h *Handler) filteredInterfaces() []config.InterfaceConfig {
	if h.interfaceFilter >= 0 && h.interfaceFilter < len(h.config.Interfaces) {
		return h.config.Interfaces[h.interfaceFilter : h.interfaceFilter+1]
	}
	return h.config.Interfaces
}

func (h *Handler) InitSubnets() {
	for _, iface := range h.config.Interfaces {
		for _, subnet := range iface.Subnets {
			// 确定 DHCP 模式
			dhcpMode := subnet.DHCP
			if dhcpMode == "" {
				dhcpMode = "full"
			}

			var pools []*IPRange

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

			// Proxy 子网：即使无地址池也注册（使用 MAC 派生 IP）
			if dhcpMode == "proxy" {
				gateway := net.ParseIP(subnet.Gateway)
				h.leaseMgr.AddSubnet(subnet.CIDR, pools, gateway)
				if len(pools) == 0 {
					slog.Info("代理子网已注册（无地址池，使用 MAC 派生 IP）", "cidr", subnet.CIDR)
				} else {
					slog.Info("代理子网已注册", "cidr", subnet.CIDR, "pools", subnet.Pools)
				}
				continue
			}

			// Full: 必须有地址池和网关
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

	// 加载 DHCP 预留
	h.loadReservations()
}

func (h *Handler) loadReservations() {
	reservations, err := h.store.ListDHCPReservations(context.Background(), "")
	if err != nil {
		slog.Error("加载 DHCP 预留失败", "error", err)
		return
	}
	for _, r := range reservations {
		h.leaseMgr.AddReservation(r.SubnetCIDR, r.MAC, r.IP)
	}
	if len(reservations) > 0 {
		slog.Info("DHCP 预留已加载", "count", len(reservations))
	}
}

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
	isIPXE := IsIPXEClient(pkt)
	isPXE := IsPXEClient(pkt) || isIPXE

	slog.Info("DHCP 请求",
		"service", "DHCP",
		"mac", mac,
		"type", mt.String(),
		"isPXE", isPXE,
		"vci", pkt.ClassIdentifier(),
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


	// 查找匹配子网
	var subnetCfg *config.SubnetConfig
	var serverIP net.IP
	peerIP := peer.(*net.UDPAddr).IP

	// 1) 通过 giaddr 匹配（中继场景）
	if !pkt.GatewayIPAddr.IsUnspecified() && !pkt.GatewayIPAddr.IsLoopback() {
		slog.Info("通过 giaddr 匹配子网", "giaddr", pkt.GatewayIPAddr)
		for _, iface := range h.filteredInterfaces() {
			for _, subnet := range iface.Subnets {
				_, cidrNet, err := net.ParseCIDR(subnet.CIDR)
				if err == nil && cidrNet.Contains(pkt.GatewayIPAddr) {
					subnetCfg = &subnet
					serverIP = net.ParseIP(iface.IP)
					break
				}
			}
			if subnetCfg != nil {
				break
			}
		}
	}

	// 2) 通过 ciaddr 匹配（续租场景）
	if subnetCfg == nil && !pkt.ClientIPAddr.IsUnspecified() {
		slog.Info("通过 ciaddr 匹配子网", "ciaddr", pkt.ClientIPAddr)
		for _, iface := range h.filteredInterfaces() {
			for _, subnet := range iface.Subnets {
				_, cidrNet, err := net.ParseCIDR(subnet.CIDR)
				if err == nil && cidrNet.Contains(pkt.ClientIPAddr) {
					subnetCfg = &subnet
					serverIP = net.ParseIP(iface.IP)
					break
				}
			}
			if subnetCfg != nil {
				break
			}
		}
	}

	// 3) 通过 peer IP 匹配（中继 / 非广播场景）
	if subnetCfg == nil && !peerIP.IsUnspecified() {
		slog.Info("通过 peer IP 匹配子网", "peer", peerIP)
		for _, iface := range h.filteredInterfaces() {
			for _, subnet := range iface.Subnets {
				_, cidrNet, err := net.ParseCIDR(subnet.CIDR)
				if err == nil && cidrNet.Contains(peerIP) {
					subnetCfg = &subnet
					serverIP = net.ParseIP(iface.IP)
					break
				}
			}
			if subnetCfg != nil {
				break
			}
		}
	}

	// 4) 客户端从 0.0.0.0 广播且无中继：取唯一子网
	if subnetCfg == nil && peerIP.IsUnspecified() && pkt.GatewayIPAddr.IsUnspecified() {
		var allSubnets []config.SubnetConfig
		for _, iface := range h.filteredInterfaces() {
			allSubnets = append(allSubnets, iface.Subnets...)
			if serverIP == nil && len(iface.Subnets) > 0 {
				serverIP = net.ParseIP(iface.IP)
			}
		}
		if len(allSubnets) == 1 {
			subnetCfg = &allSubnets[0]
			slog.Info("广播请求匹配唯一子网", "cidr", subnetCfg.CIDR, "mac", mac)
		} else if len(allSubnets) > 1 {
			slog.Warn("广播请求含多个子网，无法确定子网",
				"mac", mac, "subnet_count", len(allSubnets))
			return
		} else {
			slog.Warn("广播请求无可用子网", "mac", mac)
			return
		}
	}

	if subnetCfg == nil {
		slog.Warn("未找到匹配子网", "mac", mac)
		return
	}

	// ── 黑白名单检查 ──

	// 1. 全局黑名单
	blacklisted, err := h.store.IsBlacklisted(ctx, mac)
	if err != nil {
		slog.Error("黑名单查询失败，已拒绝", "mac", mac, "error", err)
		return
	}
	if blacklisted {
		slog.Info("黑名单 MAC 已拒绝", "mac", mac, "cidr", subnetCfg.CIDR)
		h.eventBus.Publish("event", models.Event{
			Type:    models.EventDHCP,
			Level:   models.EventWarn,
			Message: fmt.Sprintf("黑名单 MAC 已拒绝: %s", mac),
			MAC:     &mac,
		})
		return
	}

	// 2. 全局白名单
	if h.config.Global.WhitelistEnabled {
		whitelisted, err := h.store.IsWhitelisted(ctx, mac, subnetCfg.CIDR)
		if err != nil {
			slog.Error("白名单查询失败，已拒绝", "mac", mac, "error", err)
			return
		}
		if !whitelisted {
			slog.Info("全局白名单未命中，已拒绝", "mac", mac, "cidr", subnetCfg.CIDR)
			h.eventBus.Publish("event", models.Event{
				Type:    models.EventDHCP,
				Level:   models.EventWarn,
				Message: fmt.Sprintf("全局白名单拒绝: %s (subnet %s)", mac, subnetCfg.CIDR),
				MAC:     &mac,
			})
			return
		}
	}

	// 3. 子网级白名单
	if subnetCfg.WhitelistEnabled {
		whitelisted, err := h.store.IsWhitelisted(ctx, mac, subnetCfg.CIDR)
		if err != nil {
			slog.Error("白名单查询失败，已拒绝", "mac", mac, "error", err)
			return
		}
		if !whitelisted {
			slog.Info("子网白名单未命中，已拒绝", "mac", mac, "cidr", subnetCfg.CIDR)
			h.eventBus.Publish("event", models.Event{
				Type:    models.EventDHCP,
				Level:   models.EventWarn,
				Message: fmt.Sprintf("子网白名单拒绝: %s (subnet %s)", mac, subnetCfg.CIDR),
				MAC:     &mac,
			})
			return
		}
	}

	slog.Info("子网匹配成功",
		"mac", mac,
		"cidr", subnetCfg.CIDR,
		"gateway", subnetCfg.Gateway,
		"server_ip", serverIP,
	)

	var nextServer net.IP
	if subnetCfg.NextServer != "" {
		nextServer = net.ParseIP(subnetCfg.NextServer)
	}
	if nextServer == nil && serverIP != nil {
		nextServer = serverIP
	}

	bootloader := h.bootloaderForSubnet(subnetCfg)

	// 从匹配的子网确定 DHCP 模式
	dhcpMode := subnetCfg.DHCP
	if dhcpMode == "" {
		dhcpMode = "full"
	}

	// Proxy subnet: skip non-PXE clients
	if dhcpMode == "proxy" && !isPXE {
		slog.Debug("Proxy skip non-PXE client", "service", "DHCP", "mac", mac)
		return
	}
	slog.Info("DHCP 模式", "service", "DHCP", "mode", dhcpMode, "mac", mac)

	var reply *dhcpv4.DHCPv4
	switch mt {
	case dhcpv4.MessageTypeDiscover:
		reply = h.handleDiscover(pkt, dhcpMode, serverIP, nextServer, subnetCfg, bootloader, isIPXE)
	case dhcpv4.MessageTypeRequest:
		reply = h.handleRequest(pkt, dhcpMode, serverIP, nextServer, subnetCfg, bootloader, isIPXE)
	}

	if reply != nil {
		dst := peer
		if peerIP.IsUnspecified() {
			dst = &net.UDPAddr{IP: net.IPv4bcast, Port: peer.(*net.UDPAddr).Port}
		}
		if _, err := conn.WriteTo(reply.ToBytes(), dst); err != nil {
			slog.Error("发送 DHCP 响应失败", "service", "DHCP", "error", err)
		} else {
			slog.Info("DHCP 响应已发送",
				"service", "DHCP",
				"mac", mac,
				"yiaddr", reply.YourIPAddr,
				"type", mt.String(),
			)
		}
	}
}

// appendProxyPXEOptions 填充 ProxyDHCP 专属选项（yiaddr=0 全程，无网关/DNS/租期）
// Option 60 = PXEClient 是关键：iPXE dhcp_offer() 据此 + yiaddr=0 识别为 ProxyDHCP
func appendProxyPXEOptions(reply *dhcpv4.DHCPv4, serverIP, nextServer net.IP) {
	if serverIP != nil {
		reply.UpdateOption(dhcpv4.OptServerIdentifier(serverIP))
	}
	if nextServer != nil {
		reply.ServerIPAddr = nextServer
		reply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionTFTPServerName,
			[]byte(nextServer.String())))
	}
	reply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation,
		[]byte{6, 1, 0x0C}))
	reply.UpdateOption(dhcpv4.OptClassIdentifier("PXEClient"))
}

// appendDHCPOptions 填充 DHCP 回复的必备选项
func appendDHCPOptions(reply *dhcpv4.DHCPv4, serverIP, nextServer net.IP, subnetCfg *config.SubnetConfig) {
	if serverIP != nil {
		reply.UpdateOption(dhcpv4.OptServerIdentifier(serverIP))
	}
	if nextServer != nil {
		reply.ServerIPAddr = nextServer
	}

	if subnetCfg.CIDR != "" {
		_, ipnet, err := net.ParseCIDR(subnetCfg.CIDR)
		if err == nil {
			reply.UpdateOption(dhcpv4.OptSubnetMask(ipnet.Mask))
		}
	}

	if subnetCfg.Gateway != "" {
		if gw := net.ParseIP(subnetCfg.Gateway); gw != nil {
			reply.UpdateOption(dhcpv4.OptRouter(gw))
		}
	}

	if subnetCfg.DNSServers != "" {
		var dnsIPs []net.IP
		for _, s := range strings.Split(subnetCfg.DNSServers, ",") {
			if ip := net.ParseIP(strings.TrimSpace(s)); ip != nil {
				dnsIPs = append(dnsIPs, ip)
			}
		}
		if len(dnsIPs) > 0 {
			reply.UpdateOption(dhcpv4.OptDNS(dnsIPs...))
		}
	}

	leaseTime := subnetCfg.LeaseTime
	if leaseTime <= 0 {
		leaseTime = 3600
	}
	reply.UpdateOption(dhcpv4.OptIPAddressLeaseTime(time.Duration(leaseTime) * time.Second))

	// Option 43 · PXE Vendor Specific — Discovery Control
	// 子选项 6 值 0x0C = 禁用 PXE 服务器发现(bit2) + 禁止用户提示(bit3)
	reply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation,
		[]byte{6, 1, 0x0C}))
}

func iPXEScriptURL(serverIP net.IP, mac string) string {
	return fmt.Sprintf("http://%s:8080/boot/ipxe/script?mac=%s", serverIP, mac)
}

func (h *Handler) handleDiscover(pkt *dhcpv4.DHCPv4, mode string, serverIP, nextServer net.IP, subnetCfg *config.SubnetConfig, bootloader string, isIPXE bool) *dhcpv4.DHCPv4 {
	reply, err := dhcpv4.NewReplyFromRequest(pkt)
	if err != nil {
		return nil
	}
	reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))

	// iPXE 第二阶段：返回脚本 URL 而非启动文件
	if isIPXE {
		scriptURL := iPXEScriptURL(serverIP, pkt.ClientHWAddr.String())
		reply.BootFileName = scriptURL
		reply.UpdateOption(BuildIPXEScriptOption(scriptURL))
		if mode == "proxy" {
				// yiaddr=0 → iPXE 识别为 ProxyDHCP，存入 proxydhcp scope
				appendProxyPXEOptions(reply, serverIP, nextServer)
				} else {
			archStr, platformStr := ArchAndPlatform(pkt)
			ip, err := h.leaseMgr.AllocateWithInfo(subnetCfg.CIDR, pkt.ClientHWAddr.String(), archStr, platformStr)
			if err != nil {
				slog.Warn("IP 分配失败", "mac", pkt.ClientHWAddr.String(), "error", err)
				return nil
			}
			reply.YourIPAddr = ip
			appendDHCPOptions(reply, serverIP, nextServer, subnetCfg)
		}
		slog.Info("iPXE 脚本 Offer", "mac", pkt.ClientHWAddr.String(), "url", scriptURL, "mode", mode)
		return reply
	}

	// PXE ROM 客户端：提供启动文件 + Option 175.178（iPXE 启动后读取缓存）
	scriptURL := iPXEScriptURL(serverIP, pkt.ClientHWAddr.String())

	switch mode {
	case "proxy":
	appendProxyPXEOptions(reply, serverIP, nextServer)
	reply.YourIPAddr = net.IP{0, 0, 0, 0}
	if arch, ok := DetectClientArch(pkt); ok {
		reply.BootFileName = boot.NBPFilename(arch, bootloader)
	}
	reply.UpdateOption(BuildIPXEScriptOption(scriptURL))
	// Subnet Mask — 部分 PXE 客户端需要此选项
	if subnetCfg.CIDR != "" {
		if _, ipnet, err := net.ParseCIDR(subnetCfg.CIDR); err == nil {
			reply.UpdateOption(dhcpv4.OptSubnetMask(ipnet.Mask))
		}
	}
	case "full":
		archStr, platformStr := ArchAndPlatform(pkt)
		ip, err := h.leaseMgr.AllocateWithInfo(subnetCfg.CIDR, pkt.ClientHWAddr.String(), archStr, platformStr)
		if err != nil {
			slog.Warn("IP 分配失败", "mac", pkt.ClientHWAddr.String(), "error", err)
			return nil
		}
		reply.YourIPAddr = ip
		appendDHCPOptions(reply, serverIP, nextServer, subnetCfg)
		if arch, ok := DetectClientArch(pkt); ok {
			reply.BootFileName = boot.NBPFilename(arch, bootloader)
		}
		reply.UpdateOption(BuildIPXEScriptOption(scriptURL))
		slog.Info("DHCP Offer", "service", "DHCP", "mac", pkt.ClientHWAddr.String(), "ip", ip, "mode", mode, "bootfile", reply.BootFileName, "bootloader", bootloader)
	}

	return reply
}

func (h *Handler) handleRequest(pkt *dhcpv4.DHCPv4, mode string, serverIP, nextServer net.IP, subnetCfg *config.SubnetConfig, bootloader string, isIPXE bool) *dhcpv4.DHCPv4 {
	reply, err := dhcpv4.NewReplyFromRequest(pkt)
	if err != nil {
		return nil
	}
	reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))

	// Proxy 模式
	if mode == "proxy" {
		if isIPXE {
			// iPXE 二次 DHCP ACK：yiaddr=0, ProxyDHCP 选项
				// yiaddr=0 → iPXE 识别为 ProxyDHCP，存入 proxydhcp scope
				appendProxyPXEOptions(reply, serverIP, nextServer)
			scriptURL := iPXEScriptURL(serverIP, pkt.ClientHWAddr.String())
			reply.BootFileName = scriptURL
			reply.UpdateOption(BuildIPXEScriptOption(scriptURL))
			slog.Info("iPXE ProxyDHCP Ack", "service", "DHCP", "mac", pkt.ClientHWAddr.String(), "ns", nextServer)
			return reply
		}
		// PXE ROM 首次 ACK：只提供 PXE 选项 + 标记已引导
		appendProxyPXEOptions(reply, serverIP, nextServer)
		if arch, ok := DetectClientArch(pkt); ok {
			reply.BootFileName = boot.NBPFilename(arch, bootloader)
		}
		scriptURL := iPXEScriptURL(serverIP, pkt.ClientHWAddr.String())
		reply.UpdateOption(BuildIPXEScriptOption(scriptURL))
		slog.Info("ProxyDHCP ACK", "service", "DHCP", "mac", pkt.ClientHWAddr.String(), "bootfile", reply.BootFileName)
		return reply
	}

	appendDHCPOptions(reply, serverIP, nextServer, subnetCfg)

	// 设置 ACK 确认 IP：优先 ciaddr（续租），其次 Option 50（请求的 IP）
	if !pkt.ClientIPAddr.IsUnspecified() {
		reply.YourIPAddr = pkt.ClientIPAddr
	} else if reqIP := pkt.RequestedIPAddress(); reqIP != nil && !reqIP.IsUnspecified() {
		reply.YourIPAddr = reqIP
	}

	// 在 ACK 中也设置启动文件/脚本 URL + Option 175.178
	if isIPXE {
		scriptURL := iPXEScriptURL(serverIP, pkt.ClientHWAddr.String())
		reply.BootFileName = scriptURL
		reply.UpdateOption(BuildIPXEScriptOption(scriptURL))
	} else if arch, ok := DetectClientArch(pkt); ok {
		reply.BootFileName = boot.NBPFilename(arch, bootloader)
		scriptURL := iPXEScriptURL(serverIP, pkt.ClientHWAddr.String())
		reply.UpdateOption(BuildIPXEScriptOption(scriptURL))
	}

	slog.Info("DHCP Ack", "service", "DHCP", "mac", pkt.ClientHWAddr.String(), "yiaddr", reply.YourIPAddr, "mode", mode, "bootfile", reply.BootFileName, "bootloader", bootloader)
	return reply
}

func (h *Handler) bootloaderForSubnet(target *config.SubnetConfig) string {
	for _, iface := range h.filteredInterfaces() {
		for _, subnet := range iface.Subnets {
			// Handle 中 subnetCfg 可能指向循环变量副本（值拷贝），不能用指针比较
			// 改用 CIDR + Gateway 做值比较，CIDR 在配置中应当是唯一的
			if subnet.CIDR == target.CIDR && subnet.Gateway == target.Gateway {
				if iface.Bootloader == "" {
					return "ipxe"
				}
				return iface.Bootloader
			}
		}
	}
	return "ipxe"
}

// GetClientByIP returns the client architecture info for the given IP address.
func (h *Handler) GetClientByIP(ip string) (arch, platform string, ok bool) {
	if h.leaseMgr == nil {
		return "", "", false
	}
	info, ok := h.leaseMgr.GetClientByIP(ip)
	if !ok {
		return "", "", false
	}
	return info.Arch, info.Platform, true
}
