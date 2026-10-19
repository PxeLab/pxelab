//go:build !windows

package smb

import (
	"log/slog"

	"github.com/pxelab/pxelab/internal/app"
	"github.com/pxelab/pxelab/internal/config"
)

// newSystemServer 非 Windows 不支持系统 SMB 后端：回落内置实现并告警
// （配置可能从 Windows 机器同步过来）。
func newSystemServer(cfg config.SMBConfig) app.Server {
	slog.Warn("system SMB 后端仅 Windows 支持，已回落内置 SMB", "service", "SMB")
	return NewServer(cfg)
}
