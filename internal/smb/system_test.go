package smb

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/config"
)

// fakeRunner 模拟 PowerShell：按脚本内容返回预置输出，并记录调用顺序。
// 共享以 name -> path 记录，便于验证 Stop/幂等行为。
type fakeRunner struct {
	calls            []string
	admin            string
	lanmanStatus     string
	failStartService bool
	shares           map[string]string
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{admin: "True", lanmanStatus: "Running", shares: map[string]string{}}
}

func (f *fakeRunner) run(script string) (string, error) {
	f.calls = append(f.calls, script)
	switch {
	case strings.Contains(script, "IsInRole"):
		return f.admin, nil
	case strings.Contains(script, "Start-Service"):
		if !f.failStartService {
			f.lanmanStatus = "Running"
		}
		return "", nil
	case strings.Contains(script, "Get-Service"):
		return f.lanmanStatus, nil
	case strings.Contains(script, "New-SmbShare"):
		f.shares[between(script, "-Name '", "'")] = between(script, "-Path '", "'")
		return "", nil
	case strings.Contains(script, "Remove-SmbShare"):
		delete(f.shares, between(script, "-Name '", "'"))
		return "", nil
	case strings.Contains(script, "Get-SmbShare"):
		name := between(script, "-Name '", "'")
		if p, ok := f.shares[name]; ok {
			return p, nil
		}
		return "__NONE__", nil
	}
	return "", nil
}

func (f *fakeRunner) joined() string { return strings.Join(f.calls, "\n") }

func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	s = s[i+len(start):]
	if j := strings.Index(s, end); j >= 0 {
		return s[:j]
	}
	return ""
}

func newTestSystemServer(f *fakeRunner, sharePath string) *systemServer {
	return &systemServer{
		cfg: config.SMBConfig{
			Backend:       config.SMBBackendSystem,
			SystemAccount: "darre",
			Shares:        []config.SMBShareConfig{{Name: "isos", Path: sharePath, ReadOnly: true}},
		},
		run: f.run,
	}
}

// TestSystemServerStartCreatesShareAndACL：目录不存在时自动创建，按只读创建共享，
// 并授予 NTFS 读权限；Stop 时删除共享。
func TestSystemServerStartCreatesShareAndACL(t *testing.T) {
	sharePath := filepath.Join(t.TempDir(), "boot", "isos") // 故意不存在
	f := newFakeRunner()
	s := newTestSystemServer(f, sharePath)

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if fi, err := os.Stat(sharePath); err != nil || !fi.IsDir() {
		t.Fatalf("共享目录应被自动创建: %v", err)
	}
	clean := filepath.Clean(sharePath)
	got := f.joined()
	if !strings.Contains(got, "New-SmbShare -Name 'isos' -Path '"+clean+"' -ReadAccess 'darre'") {
		t.Fatalf("应按只读创建共享:\n%s", got)
	}
	if !strings.Contains(got, "icacls '"+clean+"' /grant 'darre:(OI)(CI)(RX)'") {
		t.Fatalf("应授予 NTFS 读权限:\n%s", got)
	}
	if f.shares["isos"] != clean {
		t.Fatalf("共享路径 = %q, want %q", f.shares["isos"], clean)
	}

	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, ok := f.shares["isos"]; ok {
		t.Fatal("Stop 应删除本次创建的共享")
	}
}

// TestSystemServerWritableUsesChangeAccess：非只读共享用 -ChangeAccess + Modify ACL。
func TestSystemServerWritableUsesChangeAccess(t *testing.T) {
	sharePath := t.TempDir()
	f := newFakeRunner()
	s := newTestSystemServer(f, sharePath)
	s.cfg.Shares[0].ReadOnly = false

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	got := f.joined()
	if !strings.Contains(got, "-ChangeAccess 'darre'") {
		t.Fatalf("可写共享应用 -ChangeAccess:\n%s", got)
	}
	if !strings.Contains(got, "'darre:(OI)(CI)(M)'") {
		t.Fatalf("可写共享应授予 Modify:\n%s", got)
	}
}

func TestSystemServerRequiresElevation(t *testing.T) {
	f := newFakeRunner()
	f.admin = "False"
	s := newTestSystemServer(f, t.TempDir())

	err := s.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "管理员") {
		t.Fatalf("非管理员应给出明确提示, got %v", err)
	}
	if strings.Contains(f.joined(), "New-SmbShare") {
		t.Fatal("未提权时不应尝试创建共享")
	}
}

func TestSystemServerRequiresAccount(t *testing.T) {
	f := newFakeRunner()
	s := newTestSystemServer(f, t.TempDir())
	s.cfg.SystemAccount = "  "
	err := s.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "system_account") {
		t.Fatalf("缺少账号应报错, got %v", err)
	}
}

// TestSystemServerReplacesForeignShare：同名共享指向别处时先删再按我们的路径重建。
func TestSystemServerReplacesForeignShare(t *testing.T) {
	sharePath := filepath.Join(t.TempDir(), "isos")
	f := newFakeRunner()
	f.shares["isos"] = `D:\somewhere\else`
	s := newTestSystemServer(f, sharePath)

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !strings.Contains(f.joined(), "Remove-SmbShare -Name 'isos' -Force") {
		t.Fatalf("应替换指向别处的同名共享:\n%s", f.joined())
	}
	if f.shares["isos"] != filepath.Clean(sharePath) {
		t.Fatalf("共享路径应被改写, got %q", f.shares["isos"])
	}
}

// TestSystemServerStopKeepsForeignShare：路径不属于我们时 Stop 不得删除该共享。
func TestSystemServerStopKeepsForeignShare(t *testing.T) {
	f := newFakeRunner()
	f.shares["isos"] = `D:\somewhere\else`
	s := newTestSystemServer(f, filepath.Join(t.TempDir(), "isos"))

	if err := s.Stop(context.Background()); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if strings.Contains(f.joined(), "Remove-SmbShare") {
		t.Fatalf("不应删除路径不匹配的同名共享:\n%s", f.joined())
	}
	if _, ok := f.shares["isos"]; !ok {
		t.Fatal("外部共享应保留")
	}
}

// TestSystemServerStartsLanmanServer：LanmanServer 未运行时尝试启动。
func TestSystemServerStartsLanmanServer(t *testing.T) {
	f := newFakeRunner()
	f.lanmanStatus = "Stopped"
	s := newTestSystemServer(f, t.TempDir())

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !strings.Contains(f.joined(), "Start-Service -Name LanmanServer") {
		t.Fatalf("应尝试启动 LanmanServer:\n%s", f.joined())
	}
}

func TestSystemServerLanmanStillStopped(t *testing.T) {
	f := newFakeRunner()
	f.lanmanStatus = "Stopped"
	f.failStartService = true
	s := newTestSystemServer(f, t.TempDir())

	err := s.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "LanmanServer") {
		t.Fatalf("LanmanServer 未运行应报错, got %v", err)
	}
}

func TestPSQuote(t *testing.T) {
	if got := psQuote(`D:\a b\it's`); got != `'D:\a b\it''s'` {
		t.Fatalf("psQuote = %s", got)
	}
}
