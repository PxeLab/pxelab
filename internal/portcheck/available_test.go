package portcheck

import (
	"net"
	"strconv"
	"testing"
)

func TestAvailableDetectsBusyPort(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("无法绑定测试端口: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port

	if ok, _ := Available(port, "tcp"); ok {
		_ = l.Close()
		t.Fatalf("端口 %d 正被占用，Available 应为 false", port)
	}

	_ = l.Close()
	// 释放后应可用（极端情况下可能被其它进程抢走，跳过而非误报）
	if ok, _ := Available(port, "tcp"); !ok {
		t.Skipf("端口 %d 释放后被占用（环境相关）", port)
	}
}

func TestAvailableUDPBusyPort(t *testing.T) {
	c, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Skipf("无法绑定测试 UDP 端口: %v", err)
	}
	defer c.Close()
	port := c.LocalAddr().(*net.UDPAddr).Port
	if ok, _ := Available(port, "udp"); ok {
		t.Fatalf("UDP 端口 %s 正被占用，Available 应为 false", strconv.Itoa(port))
	}
}

func TestAvailableInvalidInputs(t *testing.T) {
	// 非法端口 / 未知协议：视为可用（不做判断）
	for _, p := range []int{0, -1, 70000} {
		if ok, _ := Available(p, "tcp"); !ok {
			t.Fatalf("端口 %d 应视为可用", p)
		}
	}
	if ok, _ := Available(12345, "sctp"); !ok {
		t.Fatal("未知协议应视为可用")
	}
}

func TestDescribeList(t *testing.T) {
	if got := DescribeList(nil); got != nil {
		t.Fatalf("nil 应返回 nil, got %v", got)
	}
	got := DescribeList([]Process{{PID: 4, Name: "System"}, {PID: 99}})
	if len(got) != 2 || got[0] != "System (PID 4)" || got[1] != "PID 99 (PID 99)" {
		t.Fatalf("DescribeList = %v", got)
	}
}
