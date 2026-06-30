package dns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

const dnsTimeout = 5 * time.Second

var dnsTypeMap = map[uint16]string{
	dns.TypeA:     "A",
	dns.TypeAAAA:  "AAAA",
	dns.TypeCNAME: "CNAME",
	dns.TypeTXT:   "TXT",
	dns.TypeMX:    "MX",
}

type Handler struct {
	store         store.Interface
	localDomain   string
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
		localDomain = "pxego.local"
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

	// Extract hostname: "pxe-server.pxego.local" → "pxe-server"
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
