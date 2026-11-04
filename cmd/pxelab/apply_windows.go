//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func applyServiceMode() bool {
	return isWindowsService()
}

func configureDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008 | 0x00000200, // DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP
	}
}

func applyWaitForProcess(pid int, timeout time.Duration) error {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// 进程已经不存在，或无权打开；两种情况都可以继续尝试安装
		return nil
	}
	defer windows.CloseHandle(h)

	event, err := windows.WaitForSingleObject(h, uint32(timeout/time.Millisecond))
	if err != nil {
		return err
	}
	if event == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("等待旧进程退出超时")
	}
	return nil
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
