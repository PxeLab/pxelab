package api

import (
	"net"
	"strings"

	"github.com/pxelab/pxelab/internal/config"
)

// httpHostIsLoopback 判断请求 Host（可能带端口）是否指回本机。
func httpHostIsLoopback(host string) bool {
	h := host
	if x, _, err := net.SplitHostPort(host); err == nil {
		h = x
	}
	h = strings.ToLower(strings.Trim(h, "[]"))
	if h == "localhost" || h == "::1" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// chooseServerBase 选用于注入 URL 的服务器地址：
//   - 客户端用局域网 IP/域名访问时，直接用请求 Host（随客户端所见一致）
//   - 回环/localhost（例如管理员本机预览）时，回落成平台可被局域网访问的地址
func chooseServerBase(serverBase, reqHost string) string {
	if serverBase != "" && httpHostIsLoopback(reqHost) {
		return serverBase
	}
	return reqHost
}

// buildHTTPBase 从配置推导平台可被访问的 HTTP 基址（host:port）。
// 优先级：global.http_base > 第一个非回环网卡 IP + listen 端口。
func buildHTTPBase(cfgBase string, ifaceIPs []string, listenAddr string) string {
	if strings.TrimSpace(cfgBase) != "" {
		return strings.TrimSpace(cfgBase)
	}
	ip := ""
	for _, s := range ifaceIPs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		p := net.ParseIP(s)
		if p != nil && p.IsLoopback() {
			continue
		}
		ip = s
		break
	}
	port := "8080"
	if _, p, err := net.SplitHostPort(listenAddr); err == nil && p != "" {
		port = p
	}
	if ip == "" {
		return ""
	}
	return net.JoinHostPort(ip, port)
}

// ifaceIPs 返回配置中各接口的 IP（用于推导可被局域网访问的基址）。
func ifaceIPs(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	out := make([]string, 0, len(cfg.Interfaces))
	for _, it := range cfg.Interfaces {
		out = append(out, it.IP)
	}
	return out
}
