//go:build windows

package portcheck

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// whoOccupies 通过 netstat -ano 解析监听端口对应的 PID，再用 tasklist 解析进程名。
func whoOccupies(port int, protocol string) ([]Process, error) {
	proto := strings.ToLower(protocol)
	if proto != "tcp" && proto != "udp" {
		proto = "udp"
	}

	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return nil, fmt.Errorf("netstat: %w", err)
	}

	portStr := ":" + strconv.Itoa(port)
	pids := make(map[int]struct{})
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// 典型行: UDP    0.0.0.0:53           *:*     18408
		//         TCP    0.0.0.0:53           0.0.0.0:0  LISTENING  18408
		// UDP 行只有 4 个字段，不能按 TCP 的字段数过滤
		if len(fields) < 3 {
			continue
		}
		if !strings.EqualFold(fields[0], proto) {
			continue
		}
		// 本地地址是第二个字段，须以 ":port" 结尾
		local := fields[1]
		if !strings.HasSuffix(local, portStr) {
			continue
		}
		// 状态列可能是 *:* 或 LISTENING，PID 是最后一个字段
		pidStr := fields[len(fields)-1]
		pid, err := strconv.Atoi(pidStr)
		if err != nil || pid == 0 {
			continue
		}
		pids[pid] = struct{}{}
	}

	var procs []Process
	for pid := range pids {
		name := processName(pid)
		procs = append(procs, Process{PID: pid, Name: name})
	}
	return procs, nil
}

// processName 用 tasklist 查询 PID 对应的进程名。
func processName(pid int) string {
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return ""
	}
	// CSV 首字段是进程名，如 "verge-mihomo.exe"
	line := strings.TrimSpace(string(out))
	if line == "" {
		return ""
	}
	fields := strings.SplitN(line, ",", 2)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], `"`)
}
