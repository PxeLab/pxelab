package network

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/ipv4"
)

type TracerouteResult struct {
	Host  string          `json:"host"`
	IP    string          `json:"ip"`
	Hops  []TracerouteHop `json:"hops"`
	Error string          `json:"error,omitempty"`
}

type TracerouteHop struct {
	TTL     int       `json:"ttl"`
	IP      string    `json:"ip"`
	Host    string    `json:"host"`
	RTT     float64   `json:"rtt"`
	RTTs    []float64 `json:"rtts"`
	Timeout bool      `json:"timeout"`
}

type TracerouteOptions struct {
	MaxHops   int
	Timeout   time.Duration
	Probes    int
	LocalAddr string
}

func (o *TracerouteOptions) applyDefaults() {
	if o.MaxHops <= 0 {
		o.MaxHops = 30
	}
	if o.Timeout <= 0 {
		o.Timeout = time.Second * 2
	}
	if o.Probes <= 0 {
		o.Probes = 3
	}
}

func Traceroute(ctx context.Context, host string, opts TracerouteOptions, onHop func(TracerouteHop)) (*TracerouteResult, error) {
	opts.applyDefaults()

	ip4Str, err := resolveHost(host)
	if err != nil {
		return nil, err
	}
	ip4 := net.ParseIP(ip4Str)

	result := &TracerouteResult{
		Host: host,
		IP:   ip4Str,
		Hops: make([]TracerouteHop, 0, opts.MaxHops),
	}

	if runtime.GOOS == "windows" {
		return tracerouteWindows(ctx, ip4Str, opts, result, onHop)
	}

	id := uint16(rand.Intn(0xffff))

	for ttl := 1; ttl <= opts.MaxHops; ttl++ {
		select {
		case <-ctx.Done():
			result.Error = "取消"
			return result, nil
		default:
		}

		hop := TracerouteHop{TTL: ttl, RTTs: make([]float64, 0, opts.Probes)}
		destReached := false

		for p := 0; p < opts.Probes; p++ {
			seq := ttl*100 + p

			rttMs, hopIP, ok := sendProbe(ctx, ip4, id, seq, ttl, opts)
			if ok {
				hop.RTTs = append(hop.RTTs, rttMs)
				hop.IP = hopIP
				hop.RTT = rttMs
				hop.Timeout = false

				if p == 0 {
					names, _ := net.LookupAddr(hopIP)
					if len(names) > 0 {
						hop.Host = names[0]
					}
				}

				if hopIP == ip4Str {
					destReached = true
				}
			}
		}

		if !destReached && hop.IP == "" {
			hop.Timeout = true
		}

		result.Hops = append(result.Hops, hop)
		if onHop != nil {
			onHop(hop)
		}

		if destReached {
			break
		}
	}

	return result, nil
}

func sendProbe(ctx context.Context, dst net.IP, id uint16, seq int, ttl int, opts TracerouteOptions) (rttMs float64, hopIP string, ok bool) {
	listenAddr := "0.0.0.0"
	if opts.LocalAddr != "" {
		listenAddr = opts.LocalAddr
	}

	conn, err := net.ListenPacket("ip4:icmp", listenAddr)
	if err != nil {
		return 0, "", false
	}
	defer conn.Close()

	// 关键：设置 IP TTL，中间路由器才会回 ICMP Time Exceeded，traceroute 才能逐跳推进
	if err := ipv4.NewPacketConn(conn).SetTTL(ttl); err != nil {
		return 0, "", false
	}

	pkt := icmpEcho{
		Type: 8,
		Code: 0,
		ID:   id,
		Seq:  uint16(seq),
	}

	dstAddr := &net.IPAddr{IP: dst}
	start := time.Now()
	deadline := start.Add(opts.Timeout)
	conn.SetDeadline(deadline)

	_, err = conn.WriteTo(pkt.Marshal(), dstAddr)
	if err != nil {
		return 0, "", false
	}

	buf := make([]byte, 1500)
	for {
		n, raddr, readErr := conn.ReadFrom(buf)
		if readErr != nil {
			return 0, "", false
		}

		off := icmpOffset(buf, n)
		icmpData := buf[off:]
		icmpLen := n - off
		if icmpLen < 8 {
			continue
		}

		replyType := icmpData[0]

		if replyType == 11 && icmpLen >= 28 {
			origIP := icmpData[8:]
			if len(origIP) >= 28 {
				origIHL := int(origIP[0]&0x0f) * 4
				if origIHL < 20 {
					origIHL = 20
				}
				origICMP := origIP[origIHL:]
				if len(origICMP) >= 8 {
					replyID := binary.BigEndian.Uint16(origICMP[4:6])
					replySeq := binary.BigEndian.Uint16(origICMP[6:8])
					if replyID == id && replySeq == uint16(seq) {
						rtt := float64(time.Since(start)) / float64(time.Millisecond)
						return rtt, raddr.String(), true
					}
				}
			}
		}

		if replyType == 0 {
			replyID := binary.BigEndian.Uint16(icmpData[4:6])
			replySeq := binary.BigEndian.Uint16(icmpData[6:8])
			if replyID == id && replySeq == uint16(seq) {
				rtt := float64(time.Since(start)) / float64(time.Millisecond)
				return rtt, raddr.String(), true
			}
		}
	}
}

// ── Windows：调用 tracert.exe 并解析输出（无需管理员权限） ──
// 中文系统输出为 GBK 编码，Go regexp 会把非法 UTF-8 字节替换成 U+FFFD，
// 所以解析不依赖任何本地化词（毫秒/请求超时），只按 token 结构提取。
var (
	reTracertHop = regexp.MustCompile(`^\s*(\d+)\s+(.*)$`)
	reTracertIP  = regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)`)
)

// gbkMillisecond 是"毫秒"的 GBK 字节序列
const gbkMillisecond = "\xba\xc1\xc3\xeb"

// parseTracertRTTs 从左到右解析 RTT token（数字或 <数字），遇到主机名/IP 停止。
// 单位 token（ms / GBK毫秒）跳过；超时的 * 跳过。
func parseTracertRTTs(rest string) []float64 {
	rtts := make([]float64, 0, 3)
	for _, tok := range strings.Fields(rest) {
		if tok == "*" || tok == "ms" || tok == gbkMillisecond {
			continue
		}
		if strings.HasPrefix(tok, "<") {
			if _, err := strconv.ParseFloat(tok[1:], 64); err == nil {
				rtts = append(rtts, 0.5) // 亚毫秒记为 0.5
				continue
			}
		}
		if v, err := strconv.ParseFloat(tok, 64); err == nil {
			rtts = append(rtts, v)
			continue
		}
		break // 主机名或 IP，RTT 区结束
	}
	return rtts
}

func tracerouteWindows(ctx context.Context, ip4Str string, opts TracerouteOptions, result *TracerouteResult, onHop func(TracerouteHop)) (*TracerouteResult, error) {
	args := []string{"-h", strconv.Itoa(opts.MaxHops), "-w", strconv.Itoa(int(opts.Timeout.Milliseconds())), ip4Str}
	cmd := exec.CommandContext(ctx, "tracert", args...)
	hideWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("启动 tracert 失败: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 tracert 失败: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		m := reTracertHop.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		hopNum, _ := strconv.Atoi(m[1])
		rest := m[2]

		hop := TracerouteHop{TTL: hopNum, RTTs: parseTracertRTTs(rest)}
		if ipm := reTracertIP.FindStringSubmatch(rest); ipm != nil {
			hop.IP = ipm[1]
		}
		if len(hop.RTTs) > 0 {
			hop.RTT = hop.RTTs[0]
			hop.Timeout = false
		} else {
			hop.Timeout = true
		}
		result.Hops = append(result.Hops, hop)
		if onHop != nil {
			onHop(hop)
		}

		if hop.IP == ip4Str {
			break
		}
	}

	cmd.Wait()
	return result, nil
}
