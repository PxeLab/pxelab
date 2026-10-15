package smb

import (
	"net"
	"testing"
	"time"

	"github.com/pxelab/pxelab/internal/config"
)

// ── parseAllowIPs ──

func TestParseAllowIPs(t *testing.T) {
	nets := parseAllowIPs([]string{
		"192.168.1.10",      // 单 IP
		"10.0.0.0/8",        // CIDR
		"  172.16.0.0/12  ", // 前后空白
		"2001:db8::1",       // IPv6 单地址
		"",                  // 空串跳过
		"not-an-ip",         // 非法跳过
		"300.300.300.300",   // 非法跳过
	})
	if len(nets) != 4 {
		t.Fatalf("nets = %d, want 4（空/非法项应跳过）", len(nets))
	}

	// 单 IP 被归一为 /32 网段
	if !nets[0].Contains(net.ParseIP("192.168.1.10")) {
		t.Error("192.168.1.10 应命中")
	}
	if nets[0].Contains(net.ParseIP("192.168.1.11")) {
		t.Error("单 IP 不应命中邻居地址")
	}
	// CIDR
	if !nets[1].Contains(net.ParseIP("10.255.255.255")) {
		t.Error("10.0.0.0/8 应命中")
	}
	// IPv6 单地址
	if !nets[3].Contains(net.ParseIP("2001:db8::1")) {
		t.Error("IPv6 单地址应命中")
	}
}

func TestParseAllowIPsEmpty(t *testing.T) {
	if got := parseAllowIPs(nil); got != nil {
		t.Fatalf("nil → nil, got %v", got)
	}
	if got := parseAllowIPs([]string{"", "  ", "bogus"}); len(got) != 0 {
		t.Fatalf("全非法 → 空, got %v", got)
	}
}

// ── isAllowedAddr ──

func TestIsAllowedAddr(t *testing.T) {
	// 空列表 = 允许全部
	if !isAllowedAddr(&net.TCPAddr{IP: net.ParseIP("203.0.113.9"), Port: 12345}, nil) {
		t.Error("空列表应允许全部")
	}

	nets := parseAllowIPs([]string{"192.168.1.0/24"})
	if !isAllowedAddr(&net.TCPAddr{IP: net.ParseIP("192.168.1.50"), Port: 1}, nets) {
		t.Error("网段内应允许")
	}
	if isAllowedAddr(&net.TCPAddr{IP: net.ParseIP("192.168.2.50"), Port: 1}, nets) {
		t.Error("网段外应拒绝")
	}
	// 非 TCP 地址（无法判定）应拒绝
	if isAllowedAddr(fakeAddr{}, nets) {
		t.Error("非 TCP 地址应拒绝")
	}
	// 非 TCP 地址 + 空列表 → 允许（空列表短路）
	if !isAllowedAddr(fakeAddr{}, nil) {
		t.Error("空列表应短路允许")
	}
}

// ── filteredListener.Accept ──

func TestFilteredListenerAcceptSkipsDisallowed(t *testing.T) {
	allowed := &fakeConn{addr: &net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 1}}
	denied := &fakeConn{addr: &net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 2}}

	ln := &queueListener{conns: []net.Conn{denied, allowed}}
	fl := &filteredListener{Listener: ln, allowNets: parseAllowIPs([]string{"192.168.1.0/24"})}

	conn, err := fl.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if conn != allowed {
		t.Fatal("应返回被允许的连接")
	}
	if !denied.closed {
		t.Fatal("被拒连接应被关闭")
	}
	if !ln.exhausted() {
		t.Fatal("应已消费两个待处理连接")
	}
}

func TestFilteredListenerAcceptPropagatesError(t *testing.T) {
	fl := &filteredListener{Listener: &queueListener{}, allowNets: parseAllowIPs([]string{"192.168.1.0/24"})}
	if _, err := fl.Accept(); err == nil {
		t.Fatal("底层 listener 无更多连接时 Accept 应返回错误")
	}
}

// ── NewServer ──

func TestNewServerPortDefault(t *testing.T) {
	if s := NewServer(config.SMBConfig{}); s.port != config.DefaultPortSMB {
		t.Fatalf("默认端口 = %d, want %d", s.port, config.DefaultPortSMB)
	}
	if s := NewServer(config.SMBConfig{Port: -1}); s.port != config.DefaultPortSMB {
		t.Fatalf("负数端口应回落默认, got %d", s.port)
	}
	if s := NewServer(config.SMBConfig{Port: 1445}); s.port != 1445 {
		t.Fatalf("显式端口应保留, got %d", s.port)
	}
}

func TestNewServerParsesAllowIPsAndKeepsConfig(t *testing.T) {
	s := NewServer(config.SMBConfig{
		Port:       445,
		ServerName: "PXELAB",
		AllowIPs:   []string{"192.168.1.0/24"},
		Shares:     []config.SMBShareConfig{{Name: "isos", Path: "/tmp", ReadOnly: true}},
		Users:      []config.SMBUserConfig{{Name: "u1", Password: "p1"}},
	})
	if s.Name() != "SMB" {
		t.Fatalf("Name() = %q", s.Name())
	}
	if s.serverName != "PXELAB" {
		t.Fatalf("serverName = %q", s.serverName)
	}
	if len(s.allowNets) != 1 {
		t.Fatalf("allowNets = %d, want 1", len(s.allowNets))
	}
	if len(s.shares) != 1 || s.shares[0].Name != "isos" {
		t.Fatalf("shares = %+v", s.shares)
	}
	if len(s.users) != 1 || s.users[0].Name != "u1" {
		t.Fatalf("users = %+v", s.users)
	}
}

// ── 测试替身 ──

type fakeAddr struct{}

func (fakeAddr) Network() string { return "unix" }
func (fakeAddr) String() string  { return "fake" }

type fakeConn struct {
	addr   net.Addr
	closed bool
}

func (c *fakeConn) Read([]byte) (int, error)         { return 0, net.ErrClosed }
func (c *fakeConn) Write([]byte) (int, error)        { return 0, net.ErrClosed }
func (c *fakeConn) Close() error                     { c.closed = true; return nil }
func (c *fakeConn) LocalAddr() net.Addr              { return fakeAddr{} }
func (c *fakeConn) RemoteAddr() net.Addr             { return c.addr }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

// queueListener 按序返回预置连接，耗尽后返回错误。
type queueListener struct {
	conns []net.Conn
	next  int
}

func (l *queueListener) Accept() (net.Conn, error) {
	if l.next >= len(l.conns) {
		return nil, net.ErrClosed
	}
	c := l.conns[l.next]
	l.next++
	return c, nil
}

func (l *queueListener) Close() error   { return nil }
func (l *queueListener) Addr() net.Addr { return fakeAddr{} }
func (l *queueListener) exhausted() bool {
	return l.next >= len(l.conns)
}
