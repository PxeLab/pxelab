//go:build windows

package portcheck

import (
	"os/exec"
	"testing"
)

// TestHideWindowSetsNoWindowFlags 保证探测端口占用时不会闪 cmd 窗口：
// 服务启动失败会依次 exec netstat 与 tasklist，未隐藏时用户会看到两个窗口闪烁。
func TestHideWindowSetsNoWindowFlags(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo")
	hideWindow(cmd)
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr 应被设置")
	}
	if !cmd.SysProcAttr.HideWindow {
		t.Fatal("HideWindow 应为 true")
	}
	const createNoWindow = 0x08000000
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CreationFlags 应包含 CREATE_NO_WINDOW, got 0x%x", cmd.SysProcAttr.CreationFlags)
	}
}
