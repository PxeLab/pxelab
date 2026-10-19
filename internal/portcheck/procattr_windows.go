//go:build windows

package portcheck

import (
	"os/exec"
	"syscall"
)

// hideWindow 阻止子进程弹出控制台窗口。
//
// PxeLab 在 Windows 上通常以无控制台的方式运行（GUI/服务/tray），此时每次
// exec 未隐藏的 netstat / tasklist 都会闪一个 cmd 窗口。服务启动失败时会调用
// WhoOccupies 探测端口占用者（netstat + tasklist 两条命令）→ 用户会看到两个
// 窗口闪一下。
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}
