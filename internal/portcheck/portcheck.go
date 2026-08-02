// Package portcheck 提供跨平台的端口占用检测能力：
// 给定端口号与协议，返回占用该端口的进程列表，用于服务启动失败时的排错提示。
package portcheck

import (
	"fmt"
	"strings"
)

// Process 描述一个占用端口的进程。
type Process struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
	Path string `json:"path,omitempty"`
}

// WhoOccupies 返回占用指定端口的进程列表。
// protocol 为 "tcp" 或 "udp"（不区分大小写）。
func WhoOccupies(port int, protocol string) ([]Process, error) {
	return whoOccupies(port, protocol)
}

// Describe 返回人类可读的占用描述，如 "verge-mihomo.exe (PID 18408)"。
// 多个进程用逗号分隔；无结果时返回空字符串。
func Describe(procs []Process) string {
	if len(procs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(procs))
	for _, p := range procs {
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("PID %d", p.PID)
		}
		parts = append(parts, fmt.Sprintf("%s (PID %d)", name, p.PID))
	}
	return strings.Join(parts, ", ")
}
