// Package smb 实现内置的 SMB2 服务。它主要为 WinPE 安装 Windows 时读取
// install.wim 而存在：WinPE 是客户端，PxeLab 是服务端。
//
// 基于纯 Go 的 go-filesystems/smb，支持 SMB 2.1/3.0/3.0.2、NTLMv2 认证与
// SMB 签名（HMAC-SHA256 / AES-CMAC）。Windows 11 24H2 起客户端默认要求签名，
// 且“要求签名时 guest 访问被禁用”，因此这里只提供 NTLMv2 账号认证，不提供 guest。
package smb

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/go-filesystems/osfs"
	gofsmb "github.com/go-filesystems/smb"
	"github.com/pxelab/pxelab/internal/config"
)

// Server 实现 app.Server 接口，导出配置中的共享目录。
type Server struct {
	name       string
	port       int
	serverName string
	shares     []config.SMBShareConfig
	users      []config.SMBUserConfig
	allowNets  []*net.IPNet

	mu       sync.RWMutex
	srv      *gofsmb.Server
	listener net.Listener
	fsystems []*osfs.FS
}

// NewServer 根据配置构造一个 SMB 服务（尚未启动）。
func NewServer(cfg config.SMBConfig) *Server {
	port := cfg.Port
	if port <= 0 {
		port = config.DefaultPortSMB
	}
	return &Server{
		name:       "SMB",
		port:       port,
		serverName: cfg.ServerName,
		shares:     cfg.Shares,
		users:      cfg.Users,
		allowNets:  parseAllowIPs(cfg.AllowIPs),
	}
}

// Name 返回服务名。
func (s *Server) Name() string { return s.name }

// Start 构建 SMB 服务端、注册共享并开始监听。
func (s *Server) Start(ctx context.Context) error {
	s.mu.RLock()
	port := s.port
	serverName := s.serverName
	shares := s.shares
	users := s.users
	allowNets := s.allowNets
	s.mu.RUnlock()

	srv := gofsmb.New()
	if serverName != "" {
		srv.SetName(serverName)
	}
	for _, u := range users {
		if strings.TrimSpace(u.Name) == "" {
			continue
		}
		srv.AddUser(u.Name, u.Password)
	}
	if len(users) == 0 {
		// go-filesystems/smb 没有匿名模式：没有账号即拒绝所有连接。
		slog.Warn("SMB 未配置任何账号，将拒绝所有连接", "service", "SMB")
	}

	var fsysList []*osfs.FS
	closeAll := func() {
		for _, f := range fsysList {
			_ = f.Close()
		}
	}

	for _, sh := range shares {
		if strings.TrimSpace(sh.Name) == "" || strings.TrimSpace(sh.Path) == "" {
			slog.Warn("SMB 共享缺少 name 或 path，已跳过", "service", "SMB", "name", sh.Name, "path", sh.Path)
			continue
		}
		if err := os.MkdirAll(sh.Path, 0o755); err != nil {
			closeAll()
			return fmt.Errorf("smb: 创建共享目录失败 %s: %w", sh.Path, err)
		}

		var osfsOpts []osfs.Option
		if sh.ReadOnly {
			osfsOpts = append(osfsOpts, osfs.ReadOnly())
		}
		fsys, err := osfs.Open(sh.Path, osfsOpts...)
		if err != nil {
			closeAll()
			return fmt.Errorf("smb: 打开共享目录失败 %s: %w", sh.Path, err)
		}

		var shareOpts []gofsmb.ShareOption
		if sh.ReadOnly {
			shareOpts = append(shareOpts, gofsmb.ReadOnly())
		}
		if len(sh.AllowUsers) > 0 {
			shareOpts = append(shareOpts, gofsmb.AllowUsers(sh.AllowUsers...))
		}
		if len(sh.WriteUsers) > 0 {
			shareOpts = append(shareOpts, gofsmb.WriteUsers(sh.WriteUsers...))
		}
		if err := srv.Share(sh.Name, fsys, shareOpts...); err != nil {
			_ = fsys.Close()
			closeAll()
			return fmt.Errorf("smb: 注册共享 %s 失败: %w", sh.Name, err)
		}
		fsysList = append(fsysList, fsys)
		slog.Info("SMB 共享已注册", "service", "SMB", "share", sh.Name, "path", sh.Path, "read_only", sh.ReadOnly)
	}

	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		closeAll()
		// 445 冲突在 Windows 上极常见（系统自带 SMB 服务 LanmanServer 占用），
		// 底层 bind 错误对用户不可操作，这里追加可操作提示。
		if port == config.DefaultPortSMB {
			return fmt.Errorf("smb: 监听端口 %d 失败: %w；该端口通常被系统自带的 SMB 服务占用（Windows 上为 LanmanServer /「Server」服务），"+
				"可执行 `Stop-Service LanmanServer`（管理员）后再试，或改用其它端口（注意 WinPE 的 net use 只能走 445）", port, err)
		}
		return fmt.Errorf("smb: 监听端口 %d 失败: %w", port, err)
	}
	if len(allowNets) > 0 {
		listener = &filteredListener{Listener: listener, allowNets: allowNets}
	}

	s.mu.Lock()
	s.srv = srv
	s.listener = listener
	s.fsystems = fsysList
	s.mu.Unlock()

	go func() {
		slog.Info("SMB 服务已启动", "service", "SMB", "port", port, "shares", len(fsysList))
		if err := srv.Serve(listener); err != nil {
			select {
			case <-ctx.Done():
			default:
				slog.Error("SMB Serve 异常退出", "service", "SMB", "error", err)
			}
		}
	}()

	return nil
}

// Stop 关闭监听、断开连接并释放共享目录句柄。
func (s *Server) Stop(ctx context.Context) error {
	slog.Info("SMB 服务关闭", "service", "SMB")

	s.mu.Lock()
	srv := s.srv
	listener := s.listener
	fsystems := s.fsystems
	s.srv = nil
	s.listener = nil
	s.fsystems = nil
	s.mu.Unlock()

	var firstErr error
	if srv != nil {
		if err := srv.Close(); err != nil {
			firstErr = err
		}
	}
	if listener != nil {
		_ = listener.Close()
	}
	for _, f := range fsystems {
		if err := f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// parseAllowIPs 把 "IP" / "CIDR" 字符串解析为网段列表，跳过无法解析的项。
func parseAllowIPs(ips []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, raw := range ips {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, cidr, err := net.ParseCIDR(raw)
		if err != nil {
			ip := net.ParseIP(raw)
			if ip == nil {
				slog.Warn("SMB 允许列表解析失败，跳过", "service", "SMB", "value", raw)
				continue
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			cidr = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
		}
		nets = append(nets, cidr)
	}
	return nets
}

// filteredListener 只接受来源在允许列表内的连接，其余立即关闭。
type filteredListener struct {
	net.Listener
	allowNets []*net.IPNet
}

func (l *filteredListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		if isAllowedAddr(conn.RemoteAddr(), l.allowNets) {
			return conn, nil
		}
		slog.Warn("SMB 连接被拒绝（不在允许列表）", "service", "SMB", "client", conn.RemoteAddr().String())
		_ = conn.Close()
	}
}

// isAllowedAddr 判断地址是否落在允许列表内；空列表表示允许全部。
func isAllowedAddr(addr net.Addr, allowNets []*net.IPNet) bool {
	if len(allowNets) == 0 {
		return true
	}
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return false
	}
	for _, n := range allowNets {
		if n.Contains(tcpAddr.IP) {
			return true
		}
	}
	return false
}
