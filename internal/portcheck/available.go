package portcheck

import (
	"fmt"
	"net"
	"strings"
)

// Available 检查端口当前是否可绑定（尝试绑定后立即释放）。
//
// protocol 为 "tcp"/"udp"（不区分大小写）；无法识别的协议视为“无信息”，返回可用。
// 端口不可用时返回占用它的进程（尽力而为：Windows 经 netstat/tasklist 解析，
// 可能拿不到名字）。
//
// 这是启动前预检：与真正的 bind 之间仍有极小竞态，调用方在 bind 失败时
// 仍应使用真实错误兜底。
func Available(port int, protocol string) (bool, []Process) {
	if port <= 0 || port > 65535 {
		return true, nil
	}
	proto := strings.ToLower(strings.TrimSpace(protocol))
	var err error
	switch proto {
	case "tcp":
		var l net.Listener
		l, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = l.Close()
		}
	case "udp":
		var c net.PacketConn
		c, err = net.ListenPacket("udp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = c.Close()
		}
	default:
		return true, nil
	}
	if err == nil {
		return true, nil
	}
	procs, _ := WhoOccupies(port, proto)
	return false, procs
}

// DescribeList 把占用进程转成 UI 友好的字符串切片，如 ["svchost.exe (PID 4)"]。
func DescribeList(procs []Process) []string {
	if len(procs) == 0 {
		return nil
	}
	out := make([]string, 0, len(procs))
	for _, p := range procs {
		name := p.Name
		if name == "" {
			name = fmt.Sprintf("PID %d", p.PID)
		}
		out = append(out, fmt.Sprintf("%s (PID %d)", name, p.PID))
	}
	return out
}
