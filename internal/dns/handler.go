package dns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/metrics"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

var dnsMetrics = metrics.DefaultRegistry.GetOrCreate("dns")

const dnsTimeout = 5 * time.Second

var dnsTypeMap = map[uint16]string{
	dns.TypeA:     "A",
	dns.TypeAAAA:  "AAAA",
	dns.TypeCNAME: "CNAME",
	dns.TypeTXT:   "TXT",
	dns.TypeMX:    "MX",
	dns.TypePTR:   "PTR",
}

type Handler struct {
	store         store.Interface
	localDomain   string
	serverName    string
	upstreams     []string
	client        *dns.Client
	eventBus      *eventbus.Bus
	defaultRecord bool
	subnetCIDRs   []string          // all known subnet CIDRs
	subnetIPMap   map[string]string // subnet CIDR → 服务器 IP
	fallbackIP    string            // 非 loopback 兜底 IP
}

func NewHandler(cfg *config.Config, st store.Interface, bus *eventbus.Bus) *Handler {
	localDomain := cfg.DNS.LocalDomain
	if localDomain == "" {
		localDomain = "pxelab.local"
	}

	serverName := cfg.Global.ServerName
	if serverName == "" {
		serverName = "PxeLab"
	}

	subnetIPMap := make(map[string]string)
	var subnetCIDRs []string
	var fallbackIP string
	for _, iface := range cfg.Interfaces {
		ip := net.ParseIP(iface.IP)
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if fallbackIP == "" {
			fallbackIP = iface.IP
		}
		for _, sn := range iface.Subnets {
			if sn.CIDR != "" {
				subnetIPMap[sn.CIDR] = iface.IP
				subnetCIDRs = append(subnetCIDRs, sn.CIDR)
			}
		}
	}
	// 优先使用保存的 DefaultRecordIP（用户可能手动配过）
	if cfg.DNS.DefaultRecordIP != "" {
		fallbackIP = cfg.DNS.DefaultRecordIP
	}

	return &Handler{
		store:         st,
		localDomain:   localDomain,
		serverName:    serverName,
		upstreams:     parseUpstreams(cfg.DNS.Upstream),
		client:        &dns.Client{ReadTimeout: dnsTimeout},
		eventBus:      bus,
		defaultRecord: cfg.DNS.DefaultRecord,
		subnetCIDRs:   subnetCIDRs,
		subnetIPMap:   subnetIPMap,
		fallbackIP:    fallbackIP,
	}
}

// defaultRecordIP 根据客户端子网返回匹配的服务器 IP，无匹配时返回兜底 IP
func (h *Handler) defaultRecordIP(clientSubnet string) string {
	if clientSubnet != "" {
		if ip, ok := h.subnetIPMap[clientSubnet]; ok {
			return ip
		}
		// 尝试 CIDR 前缀匹配（/24 范围内可能有细微差异）
		for cidr, ip := range h.subnetIPMap {
			_, network, err := net.ParseCIDR(cidr)
			if err == nil {
				clientIP, _, _ := net.ParseCIDR(clientSubnet + "/" + strings.Split(cidr, "/")[1])
				if clientIP != nil && network.Contains(clientIP) {
					return ip
				}
			}
		}
	}
	return h.fallbackIP
}

// parseUpstreams splits an upstream string by comma or whitespace.
// Each address is normalized to include a port (default :53 if missing).
func parseUpstreams(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	for i, addr := range parts {
		_, _, err := net.SplitHostPort(addr)
		if err != nil {
			parts[i] = net.JoinHostPort(addr, "53")
		}
	}
	return parts
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	dnsMetrics.RecordRequest()
	m := new(dns.Msg)
	m.SetReply(r)

	if len(r.Question) == 0 {
		w.WriteMsg(m)
		return
	}

	q := r.Question[0]
	qName := strings.TrimSuffix(q.Name, ".")
	typeStr := dnsTypeMap[q.Qtype]
	clientIP := w.RemoteAddr().String()

	slog.Info("DNS 查询", "service", "DNS", "name", qName, "type", typeStr, "client", clientIP)

	// 确定客户端所属子网
	clientSubnet := h.matchClientSubnet(clientIP)

	// Try local resolution when local domain is configured
	if h.localDomain != "" && typeStr != "" {
		if answered := h.resolveLocal(m, qName, typeStr, q, clientSubnet); answered {
			slog.Info("DNS 本地解析成功", "service", "DNS", "name", qName, "type", typeStr, "client", clientIP)
			h.publishEvent(qName)
			w.WriteMsg(m)
			return
		}

		// 泛域名解析：本地未命中且查询在本地域内时，返回匹配子网的 IP
		if h.defaultRecord && typeStr == "A" {
			if strings.HasSuffix(qName, "."+h.localDomain) || strings.EqualFold(qName, h.localDomain) {
				ip := h.defaultRecordIP(clientSubnet)
				if ip == "" {
					return
				}
				slog.Info("DNS 泛域名解析", "service", "DNS", "name", qName, "ip", ip, "client", clientIP)
				m.Authoritative = true
				rr, err := dns.NewRR(fmt.Sprintf("%s 60 IN A %s", q.Name, ip))
				if err == nil {
					m.Answer = append(m.Answer, rr)
				}
				h.publishEvent(qName)
				w.WriteMsg(m)
				return
			}
		}
	}

	// PTR 反向查询：对已知子网内的 IP 本地应答，避免私有网段
	// 反向解析转发到上游（上游对 RFC1918 段 PTR 不响应，导致超时报错）
	if q.Qtype == dns.TypePTR {
		if answered := h.answerLocalPTR(m, qName, clientIP); answered {
			h.publishEvent(qName)
			w.WriteMsg(m)
			return
		}
	}

	// Fallback to upstreams (try in order, first success wins)
	if len(h.upstreams) > 0 {
		m = h.forwardToUpstream(r, qName)
		if m.Rcode != dns.RcodeServerFailure {
			slog.Info("DNS 上游解析成功", "service", "DNS", "name", qName, "type", typeStr, "client", clientIP)
		}
	} else {
		m.Rcode = dns.RcodeNameError
	}

	h.publishEvent(qName)
	w.WriteMsg(m)
}

func (h *Handler) resolveLocal(m *dns.Msg, qName string, typeStr string, q dns.Question, clientSubnet string) bool {
	// Check if query matches local domain or is a subdomain
	if !strings.HasSuffix(qName, "."+h.localDomain) && !strings.EqualFold(qName, h.localDomain) {
		return false
	}

	// Extract hostname: "pxe-server.pxelab.local" → "pxe-server"
	hostname := qName
	if strings.EqualFold(qName, h.localDomain) {
		hostname = "@" // root record for the domain itself
	} else {
		hostname = strings.TrimSuffix(qName, "."+h.localDomain)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()
	records, err := h.store.FindDNSRecords(ctx, hostname, typeStr, clientSubnet)
	if err != nil || len(records) == 0 {
		slog.Debug("DNS 本地记录未匹配", "service", "DNS", "name", qName, "type", typeStr)
		return false
	}

	// 分离子网匹配记录和全局记录
	var subnetRecords, globalRecords []models.DNSRecord
	for _, rec := range records {
		if clientSubnet != "" && rec.Subnet == clientSubnet {
			subnetRecords = append(subnetRecords, rec)
		} else if rec.Subnet == "" {
			globalRecords = append(globalRecords, rec)
		}
	}

	// 优先使用子网匹配的记录
	matched := subnetRecords
	if len(matched) == 0 {
		matched = globalRecords
	}
	if len(matched) == 0 {
		return false
	}

	m.Authoritative = true
	for _, rec := range matched {
		rr, err := dns.NewRR(fmt.Sprintf("%s %d IN %s %s", q.Name, rec.TTL, rec.Type, rec.Value))
		if err != nil {
			slog.Error("DNS 构造应答失败", "service", "DNS", "name", qName, "type", typeStr, "error", err)
			continue
		}
		m.Answer = append(m.Answer, rr)
	}
	return true
}

// matchClientSubnet 根据客户端 IP 匹配所属的子网 CIDR
func (h *Handler) matchClientSubnet(clientAddr string) string {
	host, _, err := net.SplitHostPort(clientAddr)
	if err != nil {
		host = clientAddr
	}
	clientIP := net.ParseIP(host)
	if clientIP == nil {
		return ""
	}
	for _, cidr := range h.subnetCIDRs {
		_, cidrNet, err := net.ParseCIDR(cidr)
		if err == nil && cidrNet.Contains(clientIP) {
			return cidr
		}
	}
	return ""
}

// answerLocalPTR 处理 PTR 反向查询。对已知子网内的 IP 本地应答：
//   - 服务器自身 IP → 返回 PTR 指向 serverName.localDomain
//   - DHCP 租约内 IP → 返回 PTR 指向 hostname.localDomain
//   - 其余已知子网 IP → 权威 NXDOMAIN（不转发上游，避免私有网段超时）
//
// 返回 true 表示已应答（调用方应直接返回）；false 表示应继续转发上游。
func (h *Handler) answerLocalPTR(m *dns.Msg, qName string, clientIP string) bool {
	ip := parseReverseIP(qName)
	if ip == nil {
		return false
	}

	// 仅对已知子网内的 IP 本地应答
	if !h.ipInKnownSubnet(ip) {
		return false
	}

	// 服务器自身 IP：返回 serverName.localDomain
	if h.isServerIP(ip) {
		ptrName := fmt.Sprintf("%s.%s.", h.serverName, h.localDomain)
		rr, err := dns.NewRR(fmt.Sprintf("%s 60 IN PTR %s", m.Question[0].Name, ptrName))
		if err == nil {
			m.Answer = append(m.Answer, rr)
			m.Authoritative = true
			slog.Info("DNS PTR 本地应答(服务器)", "service", "DNS", "name", qName, "ptr", ptrName, "client", clientIP)
			return true
		}
	}

	// DHCP 租约内 IP：返回租约主机名
	if hostname := h.leaseHostname(ip); hostname != "" {
		ptrName := fmt.Sprintf("%s.%s.", hostname, h.localDomain)
		rr, err := dns.NewRR(fmt.Sprintf("%s 60 IN PTR %s", m.Question[0].Name, ptrName))
		if err == nil {
			m.Answer = append(m.Answer, rr)
			m.Authoritative = true
			slog.Info("DNS PTR 本地应答(租约)", "service", "DNS", "name", qName, "ptr", ptrName, "client", clientIP)
			return true
		}
	}

	// 已知子网内但无对应记录：权威 NXDOMAIN，不打扰上游
	m.Authoritative = true
	m.Rcode = dns.RcodeNameError
	slog.Info("DNS PTR 本地应答(NXDOMAIN)", "service", "DNS", "name", qName, "client", clientIP)
	return true
}

// parseReverseIP 从 PTR 查询名解析出 IP。
// 支持 in-addr.arpa（IPv4）和 ip6.arpa（IPv6）两种格式，解析失败返回 nil。
func parseReverseIP(qName string) net.IP {
	name := strings.TrimSuffix(qName, ".")
	lower := strings.ToLower(name)

	if ipv4, ok := strings.CutSuffix(lower, ".in-addr.arpa"); ok {
		parts := strings.Split(ipv4, ".")
		if len(parts) != 4 {
			return nil
		}
		// 反转字节序: 11.77.77.77.in-addr.arpa → 77.77.77.11
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		ip := net.ParseIP(strings.Join(parts, "."))
		if ip == nil || ip.To4() == nil {
			return nil
		}
		return ip.To4()
	}

	if ipv6, ok := strings.CutSuffix(lower, ".ip6.arpa"); ok {
		// ip6.arpa 是 nibble 反转: 每个十六进制半字节一个标签
		parts := strings.Split(ipv6, ".")
		if len(parts) != 32 {
			return nil
		}
		for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
			parts[i], parts[j] = parts[j], parts[i]
		}
		// 每 4 个 nibble 组成一个 16-bit 组
		var groups []string
		for i := 0; i < 32; i += 4 {
			groups = append(groups, strings.Join(parts[i:i+4], ""))
		}
		ip := net.ParseIP(strings.Join(groups, ":"))
		if ip == nil {
			return nil
		}
		return ip
	}

	return nil
}

// ipInKnownSubnet 判断 IP 是否属于任一已知子网
func (h *Handler) ipInKnownSubnet(ip net.IP) bool {
	for _, cidr := range h.subnetCIDRs {
		_, cidrNet, err := net.ParseCIDR(cidr)
		if err == nil && cidrNet.Contains(ip) {
			return true
		}
	}
	return false
}

// isServerIP 判断 IP 是否为任一子网对应的服务器自身 IP 或兜底 IP
func (h *Handler) isServerIP(ip net.IP) bool {
	ipStr := ip.String()
	if h.fallbackIP != "" && ipStr == h.fallbackIP {
		return true
	}
	for _, srvIP := range h.subnetIPMap {
		if srvIP == ipStr {
			return true
		}
	}
	return false
}

// leaseHostname 查询 IP 对应的 DHCP 租约主机名（含主机名合法性校验）
func (h *Handler) leaseHostname(ip net.IP) string {
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()
	leases, err := h.store.ListLeases(ctx)
	if err != nil {
		return ""
	}
	ipStr := ip.String()
	for _, l := range leases {
		if l.IP == ipStr && l.Hostname != nil {
			if validHostname(*l.Hostname) {
				return *l.Hostname
			}
		}
	}
	return ""
}

// validHostname 校验主机名只能包含字母、数字、连字符和下划线
func validHostname(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// forwardToUpstream tries each upstream in order, returning the first successful response.
// If all fail, returns a SERVFAIL response.
func (h *Handler) forwardToUpstream(r *dns.Msg, qName string) *dns.Msg {
	for _, addr := range h.upstreams {
		resp, _, err := h.client.Exchange(r, addr)
		if err != nil {
			slog.Warn("DNS 转发失败", "service", "DNS", "query", qName, "upstream", addr, "error", err)
			continue
		}
		return resp
	}
	dnsMetrics.RecordError()
	slog.Error("DNS 所有上游均不可用", "service", "DNS", "query", qName, "upstreams", h.upstreams)
	m := new(dns.Msg)
	m.SetReply(r)
	m.Rcode = dns.RcodeServerFailure
	return m
}

func (h *Handler) publishEvent(name string) {
	h.eventBus.Publish("event", models.Event{
		Type:    models.EventDNS,
		Level:   models.EventInfo,
		Message: fmt.Sprintf("DNS 查询: %s", name),
	})
}
