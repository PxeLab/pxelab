package dns

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

func TestParseUpstreams(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"8.8.8.8:53", []string{"8.8.8.8:53"}},
		{"8.8.8.8:53 1.1.1.1:53", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"8.8.8.8:53,1.1.1.1:53", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"8.8.8.8:53, 1.1.1.1:53", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"8.8.8.8", []string{"8.8.8.8:53"}},
		{"8.8.8.8 1.1.1.1", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"dns.example.com", []string{"dns.example.com:53"}},
		{"dns.example.com:5353", []string{"dns.example.com:5353"}},
		{"2001:4860:4860::8888", []string{"[2001:4860:4860::8888]:53"}},
		{"", nil},
	}
	for _, tc := range tests {
		got := parseUpstreams(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("parseUpstreams(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseUpstreams(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestNewHandler(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	cfg := &config.Config{DNS: config.DNSConfig{Upstream: "8.8.8.8:53"}}
	h := NewHandler(cfg, st, bus)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if len(h.upstreams) != 1 || h.upstreams[0] != "8.8.8.8:53" {
		t.Errorf("expected upstreams ['8.8.8.8:53'], got %v", h.upstreams)
	}
}

func TestNewHandlerNoUpstream(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	cfg := &config.Config{}
	h := NewHandler(cfg, st, bus)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if len(h.upstreams) != 0 {
		t.Errorf("expected empty upstreams, got %v", h.upstreams)
	}
}

func TestNewHandlerMultiUpstream(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	cfg := &config.Config{DNS: config.DNSConfig{Upstream: "8.8.8.8:53 1.1.1.1:53"}}
	h := NewHandler(cfg, st, bus)
	if len(h.upstreams) != 2 {
		t.Errorf("expected 2 upstreams, got %d: %v", len(h.upstreams), h.upstreams)
	}
}

// testPTRConfig 构造含已知子网(77.77.77.0/24 → 服务器 IP 77.77.77.41)的配置
func testPTRConfig() *config.Config {
	return &config.Config{
		DNS: config.DNSConfig{
			LocalDomain:   "pxelab.local",
			DefaultRecord: true,
		},
		Global: config.GlobalConfig{ServerName: "pxe-server"},
		Interfaces: []config.InterfaceConfig{
			{
				Name: "eth0",
				IP:   "77.77.77.41",
				Subnets: []config.SubnetConfig{
					{CIDR: "77.77.77.0/24"},
				},
			},
		},
	}
}

// fakeResponseWriter 实现 dns.ResponseWriter 用于单测
type fakeResponseWriter struct {
	msg *dns.Msg
}

func (f *fakeResponseWriter) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("77.77.77.41"), Port: 53}
}
func (f *fakeResponseWriter) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("77.77.77.11"), Port: 50000}
}
func (f *fakeResponseWriter) WriteMsg(m *dns.Msg) error { f.msg = m; return nil }
func (f *fakeResponseWriter) Write(b []byte) (int, error) {
	return len(b), nil
}
func (f *fakeResponseWriter) Close() error                { return nil }
func (f *fakeResponseWriter) TsigStatus() error           { return nil }
func (f *fakeResponseWriter) TsigTimersOnly(bool)         {}
func (f *fakeResponseWriter) Hijack()                     {}

// ptrQuery 构造 PTR 查询消息
func ptrQuery(qName string) *dns.Msg {
	return &dns.Msg{
		MsgHdr: dns.MsgHdr{Id: dns.Id(), RecursionDesired: true},
		Question: []dns.Question{
			{Name: qName, Qtype: dns.TypePTR, Qclass: dns.ClassINET},
		},
	}
}

func TestAnswerLocalPTRServerIP(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	h := NewHandler(testPTRConfig(), st, bus)

	m := new(dns.Msg)
	m.SetReply(ptrQuery("41.77.77.77.in-addr.arpa."))
	answered := h.answerLocalPTR(m, "41.77.77.77.in-addr.arpa", "77.77.77.11:50000")
	if !answered {
		t.Fatal("expected server IP PTR to be answered locally")
	}
	if len(m.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(m.Answer))
	}
	rr, ok := m.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("expected PTR record, got %T", m.Answer[0])
	}
	want := "pxe-server.pxelab.local."
	if rr.Ptr != want {
		t.Errorf("expected PTR %q, got %q", want, rr.Ptr)
	}
	if !m.Authoritative {
		t.Error("expected authoritative answer")
	}
}

func TestAnswerLocalPTRLeaseHostname(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	hostname := "pxe-client-01"
	// 租约记录：IP 77.77.77.11，主机名 pxe-client-01
	lease := &models.Lease{
		MAC:       "00:0c:29:aa:bb:cc",
		IP:        "77.77.77.11",
		SubnetID:  "77.77.77.0/24",
		Hostname:  &hostname,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := st.CreateLease(context.Background(), lease); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(testPTRConfig(), st, bus)

	m := new(dns.Msg)
	m.SetReply(ptrQuery("11.77.77.77.in-addr.arpa."))
	answered := h.answerLocalPTR(m, "11.77.77.77.in-addr.arpa", "77.77.77.11:50000")
	if !answered {
		t.Fatal("expected lease IP PTR to be answered locally")
	}
	if len(m.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(m.Answer))
	}
	rr, ok := m.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("expected PTR record, got %T", m.Answer[0])
	}
	want := "pxe-client-01.pxelab.local."
	if rr.Ptr != want {
		t.Errorf("expected PTR %q, got %q", want, rr.Ptr)
	}
}

func TestAnswerLocalPTRKnownSubnetNoRecord(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	h := NewHandler(testPTRConfig(), st, bus)

	m := new(dns.Msg)
	m.SetReply(ptrQuery("12.77.77.77.in-addr.arpa."))
	answered := h.answerLocalPTR(m, "12.77.77.77.in-addr.arpa", "77.77.77.11:50000")
	if !answered {
		t.Fatal("expected known-subnet PTR to be answered locally")
	}
	if m.Rcode != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN, got rcode %d", m.Rcode)
	}
	if !m.Authoritative {
		t.Error("expected authoritative NXDOMAIN")
	}
	if len(m.Answer) != 0 {
		t.Errorf("expected no answers, got %d", len(m.Answer))
	}
}

func TestAnswerLocalPTRUnknownSubnet(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	h := NewHandler(testPTRConfig(), st, bus)

	// 未知子网（8.8.8.0/24 不在已知子网内）应返回 false，走上游转发
	m := new(dns.Msg)
	m.SetReply(ptrQuery("8.8.8.8.in-addr.arpa."))
	answered := h.answerLocalPTR(m, "8.8.8.8.in-addr.arpa", "77.77.77.11:50000")
	if answered {
		t.Error("expected unknown-subnet PTR NOT to be answered locally")
	}
}

func TestParseReverseIP(t *testing.T) {
	tests := []struct {
		qName string
		want  string
	}{
		{"11.77.77.77.in-addr.arpa.", "77.77.77.11"},
		{"1.0.0.127.in-addr.arpa", "127.0.0.1"},
		{"8.8.8.8.in-addr.arpa.", "8.8.8.8"},
		// IPv6: 2001:db8::1 的 nibble 反转
		{"1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa.", "2001:db8::1"},
		// 非法输入
		{"not-an-ip.in-addr.arpa.", ""},
		{"11.77.77.in-addr.arpa.", ""},
		{"example.com.", ""},
		{"", ""},
	}
	for _, tc := range tests {
		got := parseReverseIP(tc.qName)
		if tc.want == "" {
			if got != nil {
				t.Errorf("parseReverseIP(%q) = %v, want nil", tc.qName, got)
			}
			continue
		}
		if got == nil || got.String() != tc.want {
			t.Errorf("parseReverseIP(%q) = %v, want %s", tc.qName, got, tc.want)
		}
	}
}

func TestServeDNSPTRKnownSubnet(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	h := NewHandler(testPTRConfig(), st, bus)
	w := &fakeResponseWriter{}

	// 服务器 IP 的 PTR 查询：应本地应答且不产生上游转发
	h.ServeDNS(w, ptrQuery("41.77.77.77.in-addr.arpa."))
	if w.msg == nil {
		t.Fatal("expected response message")
	}
	if len(w.msg.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(w.msg.Answer))
	}
	rr, ok := w.msg.Answer[0].(*dns.PTR)
	if !ok {
		t.Fatalf("expected PTR record, got %T", w.msg.Answer[0])
	}
	if rr.Ptr != "pxe-server.pxelab.local." {
		t.Errorf("expected PTR pxe-server.pxelab.local., got %q", rr.Ptr)
	}
}

func TestServeDNSPTRNoUpstreamUnrelatedQuery(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	// 无上游配置
	cfg := testPTRConfig()
	cfg.DNS.Upstream = ""
	h := NewHandler(cfg, st, bus)
	w := &fakeResponseWriter{}

	// 未知子网的 PTR：无上游配置时返回 NXDOMAIN（不 panic，不产生上游报错）
	h.ServeDNS(w, ptrQuery("8.8.8.8.in-addr.arpa."))
	if w.msg == nil {
		t.Fatal("expected response message")
	}
	if w.msg.Rcode != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN for unknown subnet without upstream, got rcode %d", w.msg.Rcode)
	}
}
