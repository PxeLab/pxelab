//go:build windows

package main

import "golang.org/x/sys/windows"

func hideConsoleWindow() {
	kernel32 := windows.NewLazyDLL("kernel32.dll")
	freeConsole := kernel32.NewProc("FreeConsole")
	freeConsole.Call()
}
