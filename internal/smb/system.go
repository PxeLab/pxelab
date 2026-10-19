package smb

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pxelab/pxelab/internal/app"
	"github.com/pxelab/pxelab/internal/config"
)

// runFunc 执行一段 PowerShell 脚本并返回 stdout；可注入以便单测。
type runFunc func(script string) (string, error)

// systemServer 用 Windows 自带的 SMB 服务（LanmanServer）提供共享：PxeLab 不监听
// 444/445，只负责创建/删除共享并把共享级 + NTFS 权限授予配置的 Windows 账号。
//
// 适用场景：Windows 上 445 已被系统 SMB 占用（默认即如此），内置后端无法监听。
type systemServer struct {
	cfg     config.SMBConfig
	run     runFunc
	created []string // 本次启动接管的共享名
}

// NewFromConfig 按配置选择 SMB 后端（builtin / system）。
// system 后端在非 Windows 上会回落内置实现（见 newSystemServer）。
func NewFromConfig(cfg config.SMBConfig) app.Server {
	if config.NormalizeSMBBackend(cfg.Backend) == config.SMBBackendSystem {
		return newSystemServer(cfg)
	}
	return NewServer(cfg)
}

func (s *systemServer) Name() string { return "SMB" }

func (s *systemServer) account() string { return strings.TrimSpace(s.cfg.SystemAccount) }

func (s *systemServer) Start(ctx context.Context) error {
	if err := s.validate(); err != nil {
		return err
	}
	if err := s.ensureElevated(); err != nil {
		return err
	}
	if err := s.ensureLanmanRunning(); err != nil {
		return err
	}
	s.created = s.created[:0]
	for _, sh := range s.cfg.Shares {
		if err := s.ensureShare(sh); err != nil {
			return err
		}
	}
	slog.Info("系统 SMB 共享已就绪", "service", "SMB", "shares", len(s.created), "account", s.account())
	return nil
}

func (s *systemServer) Stop(ctx context.Context) error {
	// 只删除「路径与配置一致」的共享，避免误删管理员手工创建的同名共享。
	// 不停止 LanmanServer（它是系统服务，可能还承载其它共享）。
	for _, sh := range s.cfg.Shares {
		if err := s.removeShareIfOurs(sh); err != nil {
			return err
		}
	}
	s.created = nil
	slog.Info("系统 SMB 共享已移除", "service", "SMB")
	return nil
}

func (s *systemServer) validate() error {
	if len(s.cfg.Shares) == 0 {
		return fmt.Errorf("SMB 共享未配置")
	}
	if s.account() == "" {
		return fmt.Errorf("system SMB 后端需要配置 system_account（WinPE 连接共享所用的 Windows 账号）")
	}
	for _, sh := range s.cfg.Shares {
		if strings.TrimSpace(sh.Name) == "" || strings.TrimSpace(sh.Path) == "" {
			return fmt.Errorf("SMB 共享缺少 name 或 path")
		}
	}
	return nil
}

// ensureElevated 系统后端创建共享需要管理员权限（内置后端监听端口不需要）。
func (s *systemServer) ensureElevated() error {
	out, err := s.run(`([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)`)
	if err != nil {
		return fmt.Errorf("检测管理员权限失败: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(out), "True") {
		return fmt.Errorf("system SMB 后端需要管理员权限：请以管理员身份运行 PxeLab（New-SmbShare 需要提权）")
	}
	return nil
}

// ensureLanmanRunning 系统后端依赖 LanmanServer（Windows 的 SMB 服务）。
func (s *systemServer) ensureLanmanRunning() error {
	out, err := s.run(`(Get-Service -Name LanmanServer).Status`)
	if err != nil {
		return fmt.Errorf("查询 LanmanServer 状态失败: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(out), "Running") {
		return nil
	}
	if _, err := s.run(`Start-Service -Name LanmanServer`); err != nil {
		return fmt.Errorf("启动 LanmanServer 失败: %w", err)
	}
	out, err = s.run(`(Get-Service -Name LanmanServer).Status`)
	if err != nil || !strings.EqualFold(strings.TrimSpace(out), "Running") {
		return fmt.Errorf("LanmanServer 未运行，系统 SMB 无法提供共享")
	}
	return nil
}

func (s *systemServer) ensureShare(sh config.SMBShareConfig) error {
	path := filepath.Clean(sh.Path)
	// 目录必须存在：boot/isos 往往要等第一次提取 ISO 才创建（用户手工试时踩过
	// “系统找不到指定的路径”），这里主动补齐。
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("创建共享目录失败 %s: %w", path, err)
	}

	existing, err := s.sharePath(sh.Name)
	if err != nil {
		return err
	}
	if existing != "" && !samePath(existing, path) {
		// 同名共享指向别处：先删再按我们的路径重建。
		if _, err := s.run(fmt.Sprintf(`Remove-SmbShare -Name %s -Force`, psQuote(sh.Name))); err != nil {
			return fmt.Errorf("替换同名共享 %s 失败: %w", sh.Name, err)
		}
		existing = ""
	}
	if existing == "" {
		access := "-ChangeAccess"
		if sh.ReadOnly {
			access = "-ReadAccess"
		}
		if _, err := s.run(fmt.Sprintf(`New-SmbShare -Name %s -Path %s %s %s`,
			psQuote(sh.Name), psQuote(path), access, psQuote(s.account()))); err != nil {
			return fmt.Errorf("创建共享 %s 失败: %w", sh.Name, err)
		}
	}

	// 共享级权限只是第一层：NTFS ACL 不放行时客户端能连上但读文件会 Access denied。
	perm := "(OI)(CI)(RX)"
	if !sh.ReadOnly {
		perm = "(OI)(CI)(M)"
	}
	grant := fmt.Sprintf(`icacls %s /grant %s | Out-Null; if ($LASTEXITCODE -ne 0) { throw "icacls exited $LASTEXITCODE" }`,
		psQuote(path), psQuote(s.account()+":"+perm))
	if _, err := s.run(grant); err != nil {
		return fmt.Errorf("授予目录权限失败 %s: %w", path, err)
	}

	s.created = append(s.created, sh.Name)
	slog.Info("系统 SMB 共享已创建", "service", "SMB", "share", sh.Name, "path", path, "read_only", sh.ReadOnly)
	return nil
}

// sharePath 返回同名共享的路径；不存在时返回空串。
func (s *systemServer) sharePath(name string) (string, error) {
	out, err := s.run(fmt.Sprintf(
		`$s = Get-SmbShare -Name %s -ErrorAction SilentlyContinue; if ($s) { $s.Path } else { '__NONE__' }`,
		psQuote(name)))
	if err != nil {
		return "", fmt.Errorf("查询共享 %s 失败: %w", name, err)
	}
	p := strings.TrimSpace(out)
	if p == "" || p == "__NONE__" {
		return "", nil
	}
	return p, nil
}

func (s *systemServer) removeShareIfOurs(sh config.SMBShareConfig) error {
	existing, err := s.sharePath(sh.Name)
	if err != nil || existing == "" {
		return err
	}
	if !samePath(existing, filepath.Clean(sh.Path)) {
		return nil
	}
	if _, err := s.run(fmt.Sprintf(`Remove-SmbShare -Name %s -Force`, psQuote(sh.Name))); err != nil {
		return fmt.Errorf("移除共享 %s 失败: %w", sh.Name, err)
	}
	slog.Info("系统 SMB 共享已移除", "service", "SMB", "share", sh.Name)
	return nil
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// psQuote 把字符串嵌入 PowerShell 单引号字面量（' → ”）。
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
