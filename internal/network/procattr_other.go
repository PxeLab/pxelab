//go:build !windows

package network

import "os/exec"

// hideWindow 在非 Windows 平台是 no-op（没有控制台窗口概念）。
func hideWindow(cmd *exec.Cmd) {}
