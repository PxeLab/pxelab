//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func applyServiceMode() bool {
	// systemd 会给服务进程设置 INVOCATION_ID；launchd 会设置 XPC_SERVICE_NAME
	if os.Getenv("INVOCATION_ID") != "" && hasSystemctl() {
		return true
	}
	if os.Getenv("XPC_SERVICE_NAME") != "" && hasLaunchctl() {
		return true
	}
	return false
}

func configureDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func applyWaitForProcess(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			return nil
		}
		if err != nil && err != syscall.EPERM {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待旧进程退出超时")
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func applyStopService() error {
	return serviceStop()
}

func applyStartService() error {
	return serviceStart()
}

func applyStartDetached(target string, args []string, dir string) error {
	cmd := exec.Command(target, args...)
	cmd.Dir = dir
	configureDetached(cmd)
	return cmd.Start()
}
