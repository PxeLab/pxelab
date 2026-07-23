package network

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type PingResult struct {
	Host      string       `json:"host"`
	IP        string       `json:"ip"`
	Sent      int          `json:"sent"`
	Received  int          `json:"received"`
	Lost      int          `json:"lost"`
	MinRTT    float64      `json:"min_rtt"`
	MaxRTT    float64      `json:"max_rtt"`
	AvgRTT    float64      `json:"avg_rtt"`
	StdDev    float64      `json:"stddev_rtt"`
	Packets   []PingPacket `json:"packets"`
	Reachable bool         `json:"reachable"`
	Error     string       `json:"error,omitempty"`
}

type PingPacket struct {
	Seq   int     `json:"seq"`
	RTT   float64 `json:"rtt"`
	TTL   int     `json:"ttl"`
	Bytes int     `json:"bytes"`
	Error string  `json:"error,omitempty"`
}

type PingOptions struct {
	Count     int
	Timeout   time.Duration
	Interval  time.Duration
	Size      int
	TTL       int
	LocalAddr string
}

func (o *PingOptions) applyDefaults() {
	// Count <= 0 表示持续 ping（由调用方中止），不再强制默认 4
	if o.Timeout <= 0 {
		o.Timeout = time.Second * 2
	}
	if o.Interval <= 0 {
		o.Interval = time.Second
	}
}

var (
	reWinReply = regexp.MustCompile(`=(\d+)\s+\S+[=<](\d+)\s*ms\s+TTL=(\d+)`)
	reLinReply = regexp.MustCompile(`bytes from [^(]+\((\d+\.\d+\.\d+\.\d+)\): icmp_seq=(\d+) ttl=(\d+) time=([0-9.]+)\s*ms`)
	reLinTimeout = regexp.MustCompile(`no answer|timeout|100% packet loss`)
)

// gbkRequestTimeout 是"请求超时"的 GBK 字节序列。
// Go regexp 按 UTF-8 解析输入，GBK 字节会被替换成 U+FFFD，因此不能用正则匹配 GBK 词，
// 但 strings.Contains 按原始字节工作，可以安全使用。
const gbkRequestTimeout = "\xc7\xeb\xc7\xf3\xb3\xac\xca\xb1"

func isWinTimeout(line string) bool {
	return strings.Contains(strings.ToLower(line), "timed out") || strings.Contains(line, gbkRequestTimeout)
}

func resolveHost(host string) (string, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("无法解析主机: %w", err)
	}
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			return ip4.String(), nil
		}
	}
	return "", fmt.Errorf("无法解析 IPv4 地址")
}

func Ping(ctx context.Context, host string, opts PingOptions, onPacket func(PingPacket)) (*PingResult, error) {
	opts.applyDefaults()

	ip, err := resolveHost(host)
	if err != nil {
		return nil, err
	}

	result := &PingResult{
		Host:    host,
		IP:      ip,
		Packets: make([]PingPacket, 0, opts.Count),
	}

	if runtime.GOOS == "windows" {
		return pingWindows(ctx, host, ip, opts, onPacket, result)
	}
	return pingLinux(ctx, host, ip, opts, onPacket, result)
}

func pingWindows(ctx context.Context, host, ip string, opts PingOptions, onPacket func(PingPacket), result *PingResult) (*PingResult, error) {
	var args []string
	if opts.Count > 0 {
		args = append(args, "-n", strconv.Itoa(opts.Count))
	} else {
		args = append(args, "-t") // 持续 ping，直到 ctx 取消被 kill
	}
	if opts.Size > 0 {
		args = append(args, "-l", strconv.Itoa(opts.Size))
	}
	args = append(args, "-w", strconv.Itoa(int(opts.Timeout.Milliseconds())), ip)

	cmd := exec.CommandContext(ctx, "ping", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("启动 ping 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 ping 失败: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		if m := reWinReply.FindStringSubmatch(line); m != nil {
			bytes, _ := strconv.Atoi(m[1])
			timeMs, _ := strconv.ParseFloat(m[2], 64)
			ttl, _ := strconv.Atoi(m[3])
			seq := len(result.Packets)
			result.Received++
			p := PingPacket{Seq: seq, RTT: timeMs, TTL: ttl, Bytes: bytes}
			result.Packets = append(result.Packets, p)
			if onPacket != nil {
				onPacket(p)
			}
			if result.MinRTT == 0 || timeMs < result.MinRTT {
				result.MinRTT = timeMs
			}
			if timeMs > result.MaxRTT {
				result.MaxRTT = timeMs
			}
		} else if isWinTimeout(line) {
			seq := len(result.Packets)
			p := PingPacket{Seq: seq, Error: "超时"}
			result.Packets = append(result.Packets, p)
			if onPacket != nil {
				onPacket(p)
			}
		}
	}

	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		if result.Error == "" {
			result.Error = "ping 进程异常退出"
		}
	}

	if opts.Count > 0 {
		result.Sent = opts.Count
	} else {
		result.Sent = len(result.Packets)
	}
	result.Lost = result.Sent - result.Received
	result.Reachable = result.Received > 0
	computeStats(result)
	return result, nil
}

func pingLinux(ctx context.Context, host, ip string, opts PingOptions, onPacket func(PingPacket), result *PingResult) (*PingResult, error) {
	args := []string{"-W", strconv.Itoa(int(opts.Timeout.Seconds())), "-i", fmt.Sprintf("%.1f", opts.Interval.Seconds())}
	if opts.Count > 0 {
		args = append([]string{"-c", strconv.Itoa(opts.Count)}, args...)
	}
	args = append(args, ip)
	if opts.Size > 0 {
		args = append(args, "-s", strconv.Itoa(opts.Size))
	}
	if opts.TTL > 0 {
		args = append(args, "-m", strconv.Itoa(opts.TTL))
	}

	cmd := exec.CommandContext(ctx, "ping", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("启动 ping 失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 ping 失败: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		if m := reLinReply.FindStringSubmatch(line); m != nil {
			ttl, _ := strconv.Atoi(m[3])
			timeMs, _ := strconv.ParseFloat(m[4], 64)
			seq := len(result.Packets)
			result.Received++
			p := PingPacket{Seq: seq, RTT: timeMs, TTL: ttl, Bytes: 64}
			result.Packets = append(result.Packets, p)
			if onPacket != nil {
				onPacket(p)
			}
			if result.MinRTT == 0 || timeMs < result.MinRTT {
				result.MinRTT = timeMs
			}
			if timeMs > result.MaxRTT {
				result.MaxRTT = timeMs
			}
		} else if reLinTimeout.MatchString(line) {
			seq := len(result.Packets)
			p := PingPacket{Seq: seq, Error: "超时"}
			result.Packets = append(result.Packets, p)
			if onPacket != nil {
				onPacket(p)
			}
		}
	}

	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		if result.Error == "" {
			result.Error = "ping 进程异常退出"
		}
	}

	if opts.Count > 0 {
		result.Sent = opts.Count
	} else {
		result.Sent = len(result.Packets)
	}
	result.Lost = result.Sent - result.Received
	result.Reachable = result.Received > 0
	computeStats(result)
	return result, nil
}

func computeStats(result *PingResult) {
	if result.Received == 0 {
		return
	}
	var total float64
	for _, p := range result.Packets {
		if p.Error == "" {
			total += p.RTT
		}
	}
	result.AvgRTT = total / float64(result.Received)

	var sumSq float64
	for _, p := range result.Packets {
		if p.Error == "" {
			diff := p.RTT - result.AvgRTT
			sumSq += diff * diff
		}
	}
	result.StdDev = math.Sqrt(sumSq / float64(result.Received))
}

type icmpEcho struct {
	Type     uint8
	Code     uint8
	Checksum uint16
	ID       uint16
	Seq      uint16
}

func (p *icmpEcho) Marshal() []byte {
	buf := make([]byte, 8)
	buf[0] = p.Type
	buf[1] = p.Code
	binary.BigEndian.PutUint16(buf[4:], p.ID)
	binary.BigEndian.PutUint16(buf[6:], p.Seq)
	p.Checksum = icmpChecksum(buf)
	binary.BigEndian.PutUint16(buf[2:], p.Checksum)
	return buf
}

func icmpChecksum(data []byte) uint16 {
	var sum uint32
	length := len(data)
	index := 0
	for length > 1 {
		sum += uint32(binary.BigEndian.Uint16(data[index : index+2]))
		index += 2
		length -= 2
	}
	if length > 0 {
		sum += uint32(data[index]) << 8
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += sum >> 16
	return ^uint16(sum)
}

func icmpOffset(buf []byte, n int) int {
	if n < 8 {
		return 0
	}
	t := buf[0]
	if t == 0 || t == 3 || t == 4 || t == 5 || t == 8 || t == 11 || t == 12 || t == 13 || t == 14 {
		return 0
	}
	if n < 20 {
		return 0
	}
	ihl := int(buf[0]&0x0f) * 4
	if ihl < 20 || ihl > n {
		return 20
	}
	return ihl
}
