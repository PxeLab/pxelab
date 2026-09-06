//go:build !windows

package main

import "context"

// startTray 在非 Windows 平台是 no-op（系统托盘仅 Windows 桌面使用）。
func startTray(ctx context.Context, cancel context.CancelFunc, httpAddr, dataDir string) {}
