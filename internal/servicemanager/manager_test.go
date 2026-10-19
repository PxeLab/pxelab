package servicemanager

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
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
