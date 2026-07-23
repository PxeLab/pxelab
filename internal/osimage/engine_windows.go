//go:build windows

package osimage

import (
	"os/exec"
	"syscall"
)

// hideWindow 阻止子进程弹出控制台窗口（Windows 上 exec powershell 默认会闪 cmd 窗口）
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
