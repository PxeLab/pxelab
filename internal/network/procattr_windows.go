//go:build windows

package network

import (
	"os/exec"
	"syscall"
)

// hideWindow 让子进程在 Windows 上不弹出控制台窗口。
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
