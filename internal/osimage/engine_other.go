//go:build !windows

package osimage

import "os/exec"

// hideWindow 非 Windows 平台无需处理
func hideWindow(cmd *exec.Cmd) {}
