//go:build windows

package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pxelab/pxelab/internal/config"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func init() {
	// Override the stubs from service.go
}

func isWindowsService() bool {
	ok, _ := svc.IsWindowsService()
	return ok
}

func runAsService() (bool, error) {
	if !isWindowsService() {
		return false, nil
	}
	slog.Info("以 Windows 服务模式运行")
	err := svc.Run(serviceName, &pxelabService{})
	if err != nil {
		return true, fmt.Errorf("Windows 服务运行失败: %w", err)
	}
	return true, nil
}

type pxelabService struct{}

func (s *pxelabService) Execute(args []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

	go func() {
		cfg, err := config.LoadConfig("")
		if err != nil {
			slog.Error("服务加载配置失败", "error", err)
			return
		}
		if err := run(cfg, cfg.Global.AppMode, context.Background()); err != nil {
			slog.Error("服务运行错误", "error", err)
		}
	}()

	for req := range requests {
		switch req.Cmd {
		case svc.Interrogate:
			changes <- req.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		default:
			continue
		}
	}
	return false, 0
}

func serviceInstall(cfgFile string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接服务管理器失败: %w", err)
	}
	defer m.Disconnect()

	exe := binPath()
	config := mgr.Config{
		DisplayName: serviceDisplayName,
		Description: serviceDesc,
		StartType:   windows.SERVICE_AUTO_START,
	}

	// Append config flag if specified
	var cmdLine string
	if cfgFile != "" {
		cmdLine = fmt.Sprintf("%s --config %s", exe, cfgFile)
	} else {
		cmdLine = exe
	}

	s, err := m.CreateService(serviceName, cmdLine, config)
	if err != nil {
		return fmt.Errorf("创建服务失败: %w", err)
	}
	defer s.Close()

	slog.Info("Windows 服务已安装", "name", serviceName)
	return nil
}

func serviceRemove() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接服务管理器失败: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("服务 %s 不存在: %w", serviceName, err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("删除服务失败: %w", err)
	}
	slog.Info("Windows 服务已移除", "name", serviceName)
	return nil
}

func serviceStart() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接服务管理器失败: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("服务 %s 不存在: %w", serviceName, err)
	}
	defer s.Close()

	if err := s.Start(); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}
	slog.Info("Windows 服务已启动", "name", serviceName)
	return nil
}

func serviceStop() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("连接服务管理器失败: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("服务 %s 不存在: %w", serviceName, err)
	}
	defer s.Close()

	_, err = s.Control(svc.Stop)
	if err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}
	slog.Info("Windows 服务已停止", "name", serviceName)
	return nil
}
