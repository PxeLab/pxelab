//go:build !windows

package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
)

// systemdServicePath is where user-level systemd units go.
const systemdServicePath = "/etc/systemd/system/pxelab.service"

// launchdPlistPath is where launchd plists go.
const launchdPlistPath = "/Library/LaunchDaemons/com.pxelab.plist"

func isWindowsService() bool { return false }

func runAsService() (bool, error) { return false, nil }

func serviceInstall(cfgFile string) error {
	if hasSystemctl() {
		return installSystemd(cfgFile)
	}
	if hasLaunchctl() {
		return installLaunchd(cfgFile)
	}
	return fmt.Errorf("未检测到 systemd 或 launchd")
}

func serviceRemove() error {
	if hasSystemctl() {
		return removeSystemd()
	}
	if hasLaunchctl() {
		return removeLaunchd()
	}
	return fmt.Errorf("未检测到 systemd 或 launchd")
}

func serviceStart() error {
	if hasSystemctl() {
		return runSystemctl("start")
	}
	if hasLaunchctl() {
		return runLaunchctl("load")
	}
	return fmt.Errorf("未检测到 systemd 或 launchd")
}

func serviceStop() error {
	if hasSystemctl() {
		return runSystemctl("stop")
	}
	if hasLaunchctl() {
		return runLaunchctl("unload")
	}
	return fmt.Errorf("未检测到 systemd 或 launchd")
}

// --- systemd ---

func hasSystemctl() bool {
	return exec.Command("systemctl", "--version").Run() == nil
}

func installSystemd(cfgFile string) error {
	exe, err := os.Executable()
	if err != nil {
		exe = "PxeLab"
	}

	unit := fmt.Sprintf(`[Unit]
Description=PxeLab - all-in-one PXE server
Documentation=https://github.com/pxelab/pxelab
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
`, exe)

	// Write unit file
	cmd := exec.Command("sh", "-c", fmt.Sprintf("cat > %s << 'SERVICEEOF'\n%s\nSERVICEEOF", systemdServicePath, unit))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("写入 systemd unit 失败: %w\n%s", err, out)
	}

	// Reload and enable
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	if err := exec.Command("systemctl", "enable", "PxeLab").Run(); err != nil {
		return fmt.Errorf("启用服务失败: %w", err)
	}

	slog.Info("systemd 服务已安装")
	return nil
}

func removeSystemd() error {
	exec.Command("systemctl", "disable", "PxeLab").Run()
	exec.Command("rm", "-f", systemdServicePath).Run()
	runSystemctl("daemon-reload")
	slog.Info("systemd 服务已移除")
	return nil
}

func runSystemctl(action string) error {
	cmd := exec.Command("systemctl", action, "PxeLab")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl %s 失败: %w\n%s", action, err, out)
	}
	return nil
}

// --- launchd ---

func hasLaunchctl() bool {
	return exec.Command("launchctl", "version").Run() == nil
}

func installLaunchd(cfgFile string) error {
	exe, err := os.Executable()
	if err != nil {
		exe = "PxeLab"
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.pxelab</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>KeepAlive</key>
	<true/>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>/var/log/pxelab.log</string>
	<key>StandardErrorPath</key>
	<string>/var/log/pxelab.log</string>
</dict>
</plist>
`, exe)

	cmd := exec.Command("sh", "-c", fmt.Sprintf("cat > %s << 'SERVICEEOF'\n%s\nSERVICEEOF", launchdPlistPath, plist))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("写入 launchd plist 失败: %w\n%s", err, out)
	}

	slog.Info("launchd 服务已安装")
	return nil
}

func removeLaunchd() error {
	runLaunchctl("unload")
	exec.Command("rm", "-f", launchdPlistPath).Run()
	slog.Info("launchd 服务已移除")
	return nil
}

func runLaunchctl(action string) error {
	cmd := exec.Command("launchctl", action, launchdPlistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl %s 失败: %w\n%s", action, err, out)
	}
	return nil
}
