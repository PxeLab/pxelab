package main

import (
	"context"
	"log/slog"
	"net"
	"os/exec"
	"runtime"

	"github.com/getlantern/systray"
)

func startTray(ctx context.Context, cancel context.CancelFunc, httpAddr, dataDir string) {
	systray.Run(func() {
		slog.Info("系统托盘已启动")
		systray.SetTitle("PxeLab")
		systray.SetTooltip("PxeLab - all-in-one PXE server")

		icon, err := generateTrayIcon()
		if err == nil {
			systray.SetIcon(icon)
		} else {
			slog.Debug("生成托盘图标失败", "error", err)
		}

		mOpen := systray.AddMenuItem("Open Dashboard", "Open PxeLab web dashboard")
		mData := systray.AddMenuItem("Open Data Directory", "Open data folder")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Stop PxeLab")

		dashboardURL := resolveDashboardURL(httpAddr)

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					openBrowser(dashboardURL)
				case <-mData.ClickedCh:
					openLocation(dataDir)
				case <-mQuit.ClickedCh:
					cancel()
					systray.Quit()
					return
				case <-ctx.Done():
					systray.Quit()
					return
				}
			}
		}()
	}, func() {
		cancel()
	})
}

func resolveDashboardURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = "8080"
	}
	if host == "" || host == "0.0.0.0" {
		host = "localhost"
	}
	if port == "" {
		port = "8080"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func openLocation(path string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "explorer"
		args = []string{path}
	case "darwin":
		cmd = "open"
		args = []string{path, "--reveal"}
	default:
		cmd = "xdg-open"
		args = []string{path}
	}
	if err := exec.Command(cmd, args...).Start(); err != nil {
		slog.Debug("打开目录失败", "path", path, "error", err)
	}
}
