package servicemanager

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

type fakeServer struct {
	name     string
	startErr error
	started  int
	stopped  int
}

func (f *fakeServer) Name() string                { return f.name }
func (f *fakeServer) Start(context.Context) error { f.started++; return f.startErr }
func (f *fakeServer) Stop(context.Context) error  { f.stopped++; return nil }

// TestStartReturnsErrorSynchronously 启动失败必须同步返回错误：旧实现把 Start
// 丢进 goroutine，API 先返回 200，前端以为成功、故障只落在日志里。
func TestStartReturnsErrorSynchronously(t *testing.T) {
	m := New()
	m.Register("smb", "SMB", &fakeServer{name: "smb", startErr: errors.New("boom")}, false, false, 0, "tcp")

	err := m.Start("smb")
	if err == nil {
		t.Fatal("启动失败应同步返回错误")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("错误应包含原始原因: %v", err)
	}
	info, ok := m.Get("smb")
	if !ok {
		t.Fatal("Get 应返回已注册服务")
	}
	if info.Status != StatusError {
		t.Fatalf("status = %q, want %q", info.Status, StatusError)
	}
	if info.ErrorMsg == "" {
		t.Fatal("errorMsg 应被记录")
	}
}

func TestStartSetsRunningOnSuccess(t *testing.T) {
	m := New()
	m.Register("http", "HTTP", &fakeServer{name: "http"}, false, false, 0, "tcp")

	if err := m.Start("http"); err != nil {
		t.Fatalf("start: %v", err)
	}
	info, _ := m.Get("http")
	if info.Status != StatusRunning {
		t.Fatalf("status = %q, want running", info.Status)
	}
	if info.ErrorMsg != "" {
		t.Fatalf("errorMsg 应为空: %q", info.ErrorMsg)
	}
	if info.StartedAt == nil {
		t.Fatal("startedAt 应被设置")
	}
}

// TestRestartAfterStopReportsStartFailure 回归：Stop() 会 close(stopCh)，旧实现
// 复用同一 channel，重启后的启动失败会被误判为“主动停止”而静默吞掉。
func TestRestartAfterStopReportsStartFailure(t *testing.T) {
	m := New()
	srv := &fakeServer{name: "smb"}
	m.Register("smb", "SMB", srv, false, false, 0, "tcp")

	if err := m.Start("smb"); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if err := m.Stop("smb"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	srv.startErr = errors.New("after-restart-failure")
	if err := m.Start("smb"); err == nil {
		t.Fatal("Stop 之后再启动失败必须被报告（stopCh 需按次重建）")
	}
}

func TestStartUnknownService(t *testing.T) {
	m := New()
	if err := m.Start("nope"); err == nil {
		t.Fatal("未知服务应报错")
	}
}

// blockingServer 模拟 DHCP/ProxyDHCP：Start 在前台 serve，永不返回，
// 直到 ctx 取消。
type blockingServer struct {
	name    string
	started chan struct{}
}

func (b *blockingServer) Name() string { return b.name }
func (b *blockingServer) Start(ctx context.Context) error {
	close(b.started)
	<-ctx.Done()
	return nil
}
func (b *blockingServer) Stop(ctx context.Context) error { return nil }

func freeTCPPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("无法取空闲端口: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestStartDoesNotHangOnForegroundServe 回归：Manager.Start 曾改为同步调用，
// 遇到前台 serve 型服务（ProxyDHCP）会永久卡住，导致其后的 HTTP/DNS 等不启动。
func TestStartDoesNotHangOnForegroundServe(t *testing.T) {
	m := New()
	b := &blockingServer{name: "proxy", started: make(chan struct{})}
	m.Register("proxy", "ProxyDHCP", b, false, false, freeTCPPort(t), "tcp")

	done := make(chan error, 1)
	go func() { done <- m.Start("proxy") }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("前台 serve 型服务应视为启动成功: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start 被前台 serve 循环卡住（回归）")
	}
	select {
	case <-b.started:
	default:
		t.Fatal("Start 未真正被执行")
	}
}

// TestForegroundServiceDoesNotBlockOthers 模拟 Run 的顺序自动启动：阻塞型服务
// 之后的服务必须仍能启动。
func TestForegroundServiceDoesNotBlockOthers(t *testing.T) {
	m := New()
	m.Register("proxy", "ProxyDHCP", &blockingServer{name: "proxy", started: make(chan struct{})},
		false, false, freeTCPPort(t), "tcp")
	httpSrv := &fakeServer{name: "http"}
	m.Register("http", "HTTP", httpSrv, false, false, 0, "tcp")

	if err := m.Start("proxy"); err != nil {
		t.Fatalf("proxy start: %v", err)
	}
	if err := m.Start("http"); err != nil {
		t.Fatalf("http start: %v", err)
	}
	if httpSrv.started == 0 {
		t.Fatal("阻塞型服务不应阻止后续服务启动")
	}
}

// TestStartFailsFastWhenPortBusy 所有服务启动前都做端口预检：端口被占用时
// 直接返回可读原因，且不会调用 server.Start。
func TestStartFailsFastWhenPortBusy(t *testing.T) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("无法绑定测试端口: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	m := New()
	srv := &fakeServer{name: "smb"}
	m.Register("smb", "SMB", srv, false, false, port, "tcp")

	err = m.Start("smb")
	if err == nil {
		t.Fatal("端口被占用时启动应失败")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(port)) {
		t.Fatalf("错误应包含端口号: %v", err)
	}
	if srv.started != 0 {
		t.Fatalf("预检失败不应调用 server.Start, started=%d", srv.started)
	}

	info, ok := m.Get("smb")
	if !ok {
		t.Fatal("Get 应返回服务")
	}
	if info.Status != StatusError {
		t.Fatalf("status = %q, want error", info.Status)
	}
	if info.PortAvailable == nil || *info.PortAvailable {
		t.Fatalf("PortAvailable 应为 false, got %v", info.PortAvailable)
	}
}
