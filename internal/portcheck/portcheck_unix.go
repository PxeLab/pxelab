//go:build !windows

package portcheck

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// whoOccupies 优先用 ss（iproute2），失败时回退 lsof，再回退 fuser。
func whoOccupies(port int, protocol string) ([]Process, error) {
	proto := strings.ToLower(protocol)
	if proto != "tcp" && proto != "udp" {
		proto = "udp"
	}

	if procs, err := viaSS(port, proto); err == nil && len(procs) > 0 {
		return procs, nil
	}
	if procs, err := viaLsof(port); err == nil && len(procs) > 0 {
		return procs, nil
	}
	return nil, fmt.Errorf("无法检测端口 %d 的占用进程（ss/lsof 均不可用或无结果）", port)
}

// viaSS 解析 `ss -lunp` / `ss -ltnp` 输出，形如：
// users:(("verge-mihomo",pid=18408,fd=13))
func viaSS(port int, proto string) ([]Process, error) {
	flags := "-" + proto[0:1] + "np" // -unp / -tnp
	out, err := exec.Command("ss", flags, fmt.Sprintf("sport = :%d", port)).Output()
	if err != nil {
		return nil, err
	}
	var procs []Process
	for _, line := range strings.Split(string(out), "\n") {
		idx := strings.Index(line, "users:((")
		if idx < 0 {
			continue
		}
		rest := line[idx+len("users:(("):]
		// 解析 name,pid=N
		nameEnd := strings.Index(rest, `"`)
		name := ""
		if nameEnd > 0 {
			name = rest[:nameEnd]
		}
		pidIdx := strings.Index(rest, "pid=")
		if pidIdx < 0 {
			continue
		}
		pidStr := ""
		for _, c := range rest[pidIdx+4:] {
			if c >= '0' && c <= '9' {
				pidStr += string(c)
			} else {
				break
			}
		}
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid == 0 {
			continue
		}
		procs = append(procs, Process{PID: pid, Name: name})
	}
	return procs, nil
}

// viaLsof 解析 `lsof -i :port -P -n` 输出。
func viaLsof(port int) ([]Process, error) {
	out, err := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-P", "-n").Output()
	if err != nil {
		return nil, err
	}
	var procs []Process
	lines := strings.Split(string(out), "\n")
	for i, line := range lines {
		if i == 0 {
			continue // 表头
		}
		fields := strings.Fields(line)
		// 形如: verge-mih 18408 root 13u IPv4 12345 0t0 UDP *:53
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid == 0 {
			continue
		}
		procs = append(procs, Process{PID: pid, Name: fields[0]})
	}
	return procs, nil
}
