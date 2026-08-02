package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
	"runtime/debug"

	"github.com/pxelab/pxelab/internal/api"
	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/dhcp"
	"github.com/pxelab/pxelab/internal/dns"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/httpd"
	"github.com/pxelab/pxelab/internal/models"
	pxelabnfs "github.com/pxelab/pxelab/internal/nfs"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/netboot/menus"
	"github.com/pxelab/pxelab/internal/logbus"
	"github.com/pxelab/pxelab/internal/servicemanager"
	"github.com/pxelab/pxelab/internal/session"
	"github.com/pxelab/pxelab/internal/store"
	"github.com/pxelab/pxelab/internal/tftp"
	"github.com/pxelab/pxelab/internal/updatecheck"
	"github.com/pxelab/pxelab/internal/wol"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Version 在编译时通过 -ldflags -X main.Version=xxx 注入
var Version = "dev"

// vcsRevision 从 Go 构建信息中读取 git commit
func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			rev := s.Value
			if len(rev) > 8 {
				rev = rev[:8]
			}
			return rev
		}
	}
	return ""
}

var rootCmd = &cobra.Command{
	Use:     "PxeLab",
	Short:   "PxeLab - 一体化 PXE 服务器",
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if handled, err := runAsService(); handled {
			return err
		}
		cfgFile, _ := cmd.Flags().GetString("config")
		cfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}
		mode, _ := cmd.Flags().GetString("mode")
		return run(cfg, mode == "app" || cfg.Global.AppMode, context.Background())
	},
}

func init() {
	config.BindFlags(rootCmd)
}

func run(cfg *config.Config, appMode bool, ctx context.Context) error {

	// 创建事件总线
	bus := eventbus.New()

	// ── 日志设置 ──
	var logLevel slog.Level
	switch cfg.Log.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	// 日志输出：stderr + 按服务分离的文件日志
	var logWriters io.Writer = os.Stderr

	// 默认日志目录: ~/.pxelab/logs/
	logDir := filepath.Join(cfg.Global.DataDir, "logs")
	if cfg.Log.File != "" {
		logDir = filepath.Dir(cfg.Log.File)
	}
	os.MkdirAll(logDir, 0755)

	// 可选的组合日志文件
	if cfg.Log.File != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Log.File), 0755); err == nil {
			f, err := os.OpenFile(cfg.Log.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				logWriters = io.MultiWriter(os.Stderr, f)
			}
		}
	} else {
		// 默认写组合日志文件 ~/.pxelab/logs/pxelab.log
		combinedPath := filepath.Join(logDir, "pxelab.log")
		f, err := os.OpenFile(combinedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			logWriters = io.MultiWriter(os.Stderr, f)
		}
	}

	slog.SetDefault(slog.New(logbus.NewBusHandler(
		slog.NewTextHandler(logWriters, &slog.HandlerOptions{Level: logLevel}),
		bus,
		logDir,
		logbus.RotatingConfig{
			MaxSizeMB:    cfg.Log.MaxSizeMB,
			MaxBackups:   cfg.Log.MaxBackups,
			MaxAgeDays:   cfg.Log.MaxAgeDays,
			Compress:     cfg.Log.Compress,
			CleanupHours: cfg.Log.CleanupInterval,
		},
	)))

	buildVer := Version
	if rev := vcsRevision(); rev != "" {
		buildVer = Version + " (" + rev + ")"
	}
	slog.Info("PxeLab 启动", "version", buildVer, "data_dir", cfg.Global.DataDir, "log_dir", logDir)

	// 初始化存储
	st, err := store.NewSQLite(cfg.Store.DSN)
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	if err := st.Migrate(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	if err := st.Seed(); err != nil {
		return fmt.Errorf("初始化默认数据失败: %w", err)
	}
	// 创建默认 DNS @ A 记录（指向第一个非 loopback 的网卡 IP）
	if cfg.DNS.LocalDomain != "" {
		serverIP := "127.0.0.1"
		for _, iface := range cfg.Interfaces {
			ip := net.ParseIP(iface.IP)
			if ip != nil && !ip.IsLoopback() {
				serverIP = iface.IP
				break
			}
		}
		existing, _ := st.FindDNSRecords(context.Background(), "@", "A", "")
		if len(existing) == 0 {
			_ = st.CreateDNSRecord(context.Background(), &models.DNSRecord{
				Name:    "@",
				Type:    "A",
				Value:   serverIP,
				TTL:     300,
				Enabled: true,
			})
			slog.Info("已创建默认 DNS 记录", "domain", cfg.DNS.LocalDomain, "value", serverIP)
		}

			// 创建服务器名称 A 记录（如 pxe-server.pxelab.local）
			srvName := cfg.Global.ServerName
			if srvName == "" {
				srvName = "PxeLab"
			}
			if srvName != "@" {
				existing, _ := st.FindDNSRecords(context.Background(), srvName, "A", "")
				if len(existing) == 0 {
					_ = st.CreateDNSRecord(context.Background(), &models.DNSRecord{
						Name:    srvName,
						Type:    "A",
						Value:   serverIP,
						TTL:     300,
						Enabled: true,
					})
					slog.Info("已创建服务器名称 DNS 记录", "name", srvName, "domain", cfg.DNS.LocalDomain, "value", serverIP)
				}
			}
	}

	// 导入黑白名单种子（幂等——MAC 已存在则跳过）
	for _, entry := range cfg.BlacklistSeeds {
		exists, err := st.IsBlacklisted(context.Background(), entry.MAC)
		if err != nil {
			slog.Warn("查询黑名单种子失败", "mac", entry.MAC, "error", err)
			continue
		}
		if !exists {
			if err := st.CreateBlacklist(context.Background(), &models.BlacklistEntry{
				MAC:    entry.MAC,
				Reason: entry.Reason,
				Source: "seed",
			}); err != nil {
				slog.Warn("导入黑名单种子失败", "mac", entry.MAC, "error", err)
			} else {
				slog.Info("已导入黑名单种子", "mac", entry.MAC)
			}
		}
	}
	for _, entry := range cfg.WhitelistSeeds {
		exists, err := st.IsWhitelisted(context.Background(), entry.MAC, entry.Subnet)
		if err != nil {
			slog.Warn("查询白名单种子失败", "mac", entry.MAC, "error", err)
			continue
		}
		if !exists {
			if err := st.CreateWhitelist(context.Background(), &models.WhitelistEntry{
				MAC:        entry.MAC,
				SubnetCIDR: entry.Subnet,
				Reason:     entry.Reason,
				Source:     "seed",
			}); err != nil {
				slog.Warn("导入白名单种子失败", "mac", entry.MAC, "error", err)
			} else {
				slog.Info("已导入白名单种子", "mac", entry.MAC, "subnet", entry.Subnet)
			}
		}
	}

	defer st.Close()

	// 配置文件中没有 arch_map 时自动写入默认值
	if cfg.Boot.ArchMap == nil {
		cfg.Boot.ArchMap = boot.GetArchMap()
		cfgPath := cfg.ConfigPath
		if cfgPath == "" {
			cfgPath = filepath.Join(cfg.Global.DataDir, "config.yaml")
		}
		if data, err := yaml.Marshal(cfg); err == nil {
			if err := os.WriteFile(cfgPath, data, 0644); err == nil {
				slog.Info("已写入默认架构映射到配置文件", "path", cfgPath)
			}
		}
	}
	boot.InitArchMap(cfg.Boot.ArchMap)

	bootFS := boot.NewBootFileServer(cfg.Boot.RootDir)

	extractBootFiles(cfg.Boot.RootDir, cfg.Boot.AutoUpdateBootFiles)

	leaseMgr := dhcp.NewLeaseManager(st)

	dhcpHandler := dhcp.NewHandler(cfg, st, bus, leaseMgr)
	dhcpHandler.InitSubnets()
	svcMgr := servicemanager.New()

	// DHCP/ProxyDHCP — 按子网 DHCP 模式注册对应服务
	hasDHCP := false
	registered67 := false
	for i, iface := range cfg.Interfaces {
		if iface.IP == "" {
			continue
		}

		// 遍历子网，确定需要启动的 DHCP 服务
		needsDHCP := false
		needsProxy := false
		allOff := len(iface.Subnets) > 0
		for _, sn := range iface.Subnets {
			mode := sn.DHCP
			if mode == "" {
				mode = "server"
			}
			if mode == "off" {
				continue
			}
			allOff = false
			if mode == "server" {
				needsDHCP = true
			} else if mode == "proxy" {
				needsProxy = true
			}
		}
		if len(iface.Subnets) == 0 || allOff {
			continue
		}

		hasDHCP = true
		ifaceHandler := dhcpHandler.WithInterfaceFilter(i)

		// 67 端口最多注册一次：无论 server 还是 proxy 模式都必须应答，
		// PXE 客户端先发 DISCOVER 到 67（代理模式返回 yiaddr=0 的 offer），
		// 收到响应后才会进入 4011 阶段请求 bootfile。
		if (needsDHCP || needsProxy) && !registered67 {
			// 绑定 0.0.0.0 以接收发往受限广播地址(255.255.255.255)的 DHCP 请求；
			// 绑定具体接口 IP 时 Windows 收不到此类广播。接口过滤由 handler 完成。
			dhcpAddr := "0.0.0.0:67"
			dhcpServer := dhcp.NewServer(dhcpAddr, ifaceHandler)
			svcMgr.Register("dhcp/"+iface.Name, "DHCP ("+iface.Name+")", dhcpServer, iface.AutoStart, false, 67, "UDP")
			slog.Info("DHCP 服务", "addr", dhcpAddr, "interface", iface.Name)
			registered67 = true
		}

		if needsProxy {
			// 同上：ProxyDHCP 必须绑定 0.0.0.0 才能收到 PXE 广播
			proxyAddr := "0.0.0.0:4011"
			proxyDHCP := dhcp.NewProxyServer4011(proxyAddr, ifaceHandler)
			svcMgr.Register("proxy/"+iface.Name, "ProxyDHCP ("+iface.Name+")", proxyDHCP, iface.AutoStart, false, 4011, "UDP")
			slog.Info("ProxyDHCP 服务", "addr", proxyAddr, "interface", iface.Name)
		}
	}

	// 无接口配置时回退到 :67 + :4011 监听所有地址（默认不自动启动，需用户手动开启）
	if !hasDHCP {
		dhcpServer := dhcp.NewServer("0.0.0.0:67", dhcpHandler)
		svcMgr.Register("dhcp/any", "DHCP", dhcpServer, false, false, 67, "UDP")
		proxyDHCP := dhcp.NewProxyServer4011("0.0.0.0:4011", dhcpHandler)
		svcMgr.Register("proxy/any", "ProxyDHCP", proxyDHCP, false, false, 4011, "UDP")
	}

	tftpCfg := cfg.TFTP
	if tftpCfg.Port <= 0 {
		tftpCfg.Port = config.DefaultPortTFTP
	}
	if tftpCfg.Timeout <= 0 {
		tftpCfg.Timeout = config.DefaultTFTPTimeout
	}
	tftpServer := tftp.NewServer(tftpCfg, bootFS, bus)
	svcMgr.Register("tftp", "TFTP", tftpServer, cfg.ServiceAutoStart.TFTP, false, tftpCfg.Port, "UDP")

	// Create netboot manager — extract embedded seed, then load from disk
	catalogDir := filepath.Join(cfg.Global.DataDir, "netboot", "catalog")
	os.MkdirAll(catalogDir, 0755)
	netboot.ExtractSeed(catalogDir)
	// Extract netboot.xyz menu files for static serving
	menuDir := filepath.Join(cfg.Global.DataDir, "netboot", "menu")
	if err := menus.ExtractMenus(menuDir); err != nil {
		slog.Warn("extract netboot menus failed", "error", err)
	}

	cat, err := netboot.LoadCatalog(catalogDir)
	if err != nil || len(cat.Distros) == 0 {
		cat = netboot.DefaultCatalog() // fallback to embedded-only
		slog.Warn("netboot catalog from disk empty, using embedded", "path", catalogDir)
	}
	netbootMgr := netboot.NewManager(cat)

	sessions := session.NewStore(0)

	nfsServer := pxelabnfs.NewServer(cfg.NFS.Port, cfg.NFS.MountPoints)
	svcMgr.Register("nfs", "NFS", nfsServer, cfg.ServiceAutoStart.NFS, false, cfg.NFS.Port, "TCP")

	nfsConnInfo := func() map[string]api.NFSConnectionInfo {
		stats := nfsServer.GetConnectionStats()
		result := make(map[string]api.NFSConnectionInfo, len(stats))
		for path, s := range stats {
			clients := make([]api.NFSClientInfo, len(s.Clients))
			for i, c := range s.Clients {
				clients[i] = api.NFSClientInfo{
					IP:           c.IP,
					ConnectedAt: c.ConnectedAt.Format(time.RFC3339),
					LastActivity: c.LastActivity.Format(time.RFC3339),
				}
			}
			result[path] = api.NFSConnectionInfo{
				Connections: s.Connections,
				Clients:     clients,
			}
		}
		return result
	}

	// 版本检查器（启动时异步检查一次）
	updater := updatecheck.NewChecker(Version, cfg.Global.DataDir)
	go func() {
		if _, err := updater.Check(); err != nil {
			slog.Debug("启动版本检查失败（不影响运行）", "error", err)
		}
	}()

	httpServer := httpd.NewServer(cfg, st, bus, bootFS, spaHandler(), dhcpHandler, netbootMgr, dhcpHandler.GetClientByIP, svcMgr, sessions, nfsServer.SetMountPoints, nfsConnInfo, func(name string) bool {
		info, ok := svcMgr.Get(name)
		return ok && info.Status == servicemanager.StatusRunning
	}, Version, updater)
	svcMgr.Register("http", "HTTP", httpServer, cfg.ServiceAutoStart.HTTP, true, 8080, "TCP")


	if appMode {
		slog.Info("自动打开浏览器")
		openBrowser("http://localhost:8080")
	}

	dnsServer := dns.NewServer(cfg, st, bus)
	svcMgr.Register("dns", "DNS", dnsServer, cfg.ServiceAutoStart.DNS, false, cfg.DNS.Port, "UDP")

	wolScheduler := wol.NewScheduler(st, cfg, bus)
	go wolScheduler.Start()

	return svcMgr.Run(ctx)
}

func main() {
	if useTray() {
		hideConsoleWindow()
		cfg, err := config.LoadConfig("")
		if err != nil {
			slog.Error("加载配置失败", "error", err)
			return
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			if err := run(cfg, cfg.Global.AppMode, ctx); err != nil {
				slog.Error("Server error", "error", err)
			}
		}()

		// 自动打开浏览器（等 HTTP 就绪）
		go func() {
			host, port, _ := net.SplitHostPort(cfg.Global.ListenAddr)
			if host == "" || host == "0.0.0.0" {
				host = "localhost"
			}
			if port == "" {
				port = "8080"
			}
			for i := 0; i < 30; i++ {
				if conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second); err == nil {
					conn.Close()
					break
				}
				time.Sleep(500 * time.Millisecond)
			}
			openBrowser("http://" + net.JoinHostPort(host, port))
		}()

		startTray(ctx, cancel, cfg.Global.ListenAddr, cfg.Global.DataDir)
		return
	}
	if err := rootCmd.Execute(); err != nil {
		slog.Error("程序退出", "error", err)
		os.Exit(1)
	}
}

func useTray() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	if len(os.Args) > 1 {
		return false
	}
	return !isWindowsService()
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}
	go func() {
		if err := exec.Command(cmd, args...).Start(); err != nil {
			slog.Debug("打开浏览器失败", "error", err)
		}
	}()
}
