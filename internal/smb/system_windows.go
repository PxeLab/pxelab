//go:build windows

package smb

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/pxelab/pxelab/internal/app"
	"github.com/pxelab/pxelab/internal/config"
)

func newSystemServer(cfg config.SMBConfig) app.Server {
	return &systemServer{cfg: cfg, run: runPowerShell}
}

// runPowerShell 执行 PowerShell 脚本并返回 stdout（隐藏控制台窗口，避免闪 cmd）。
func runPowerShell(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"$ProgressPreference='SilentlyContinue'; $ErrorActionPreference='Stop'; "+script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return out.String(), nil
}
