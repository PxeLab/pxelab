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
	"runtime/debug"

	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/dhcp"
	"github.com/pxego/pxego/internal/dns"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/httpd"
	"github.com/pxego/pxego/internal/models"
	pxegonfs "github.com/pxego/pxego/internal/nfs"
	"github.com/pxego/pxego/internal/netboot"
	"github.com/pxego/pxego/internal/netboot/menus"
	"github.com/pxego/pxego/internal/logbus"
	"github.com/pxego/pxego/internal/servicemanager"
	"github.com/pxego/pxego/internal/session"
	"github.com/pxego/pxego/internal/store"
	"github.com/pxego/pxego/internal/tftp"
	"github.com/spf13/cobra"
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
	Use:     "pxego",
	Short:   "PxeGo - 一体化 PXE 服务器",
	Version: Version,
	RunE: func(cmd *cobra.Command, args []string) error {
		if handled, err := runAsService(); handled {
			return err
		}
		return run(cmd)
	},
}

func init() {
	config.BindFlags(rootCmd)
}

func run(cmd *cobra.Command) error {
	cfgFile, _ := cmd.Flags().GetString("config")

	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	mode, _ := cmd.Flags().GetString("mode")
	appMode := mode == "app" || cfg.Global.AppMode

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

	// 默认日志目录: ~/.pxego/logs/
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
		// 默认写组合日志文件 ~/.pxego/logs/pxego.log
		combinedPath := filepath.Join(logDir, "pxego.log")
		f, err := os.OpenFile(combinedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			logWriters = io.MultiWriter(os.Stderr, f)
		}
	}

	slog.SetDefault(slog.New(logbus.NewBusHandler(
		slog.NewTextHandler(logWriters, &slog.HandlerOptions{Level: logLevel}),
		bus,
		logDir,
	)))

	buildVer := Version
	if rev := vcsRevision(); rev != "" {
		buildVer = Version + " (" + rev + ")"
	}
	slog.Info("PxeGo 启动", "version", buildVer, "data_dir", cfg.Global.DataDir, "log_dir", logDir)

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

			// 创建服务器名称 A 记录（如 pxe-server.pxego.local）
			srvName := cfg.Global.ServerName
			if srvName == "" {
				srvName = "pxego"
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

	bootFS := boot.NewBootFileServer(cfg.Boot.RootDir)

	extractBootFiles(cfg.Boot.RootDir)

	leaseMgr := dhcp.NewLeaseManager(st)

	dhcpHandler := dhcp.NewHandler(cfg, st, bus, leaseMgr)
	dhcpHandler.InitSubnets()
	svcMgr := servicemanager.New()

	// DHCP/ProxyDHCP — 按子网 DHCP 模式注册对应服务
	hasDHCP := false
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
				mode = "full"
			}
			if mode == "off" {
				continue
			}
			allOff = false
			if mode == "full" {
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

		if needsDHCP {
			dhcpAddr := iface.IP + ":67"
			dhcpServer := dhcp.NewServer(dhcpAddr, ifaceHandler)
			svcMgr.Register("dhcp/"+iface.Name, "DHCP ("+iface.Name+")", dhcpServer, iface.AutoStart, false, 67, "UDP")
			slog.Info("DHCP 服务", "addr", dhcpAddr, "interface", iface.Name)
		}

		if needsProxy {
			proxyAddr := iface.IP + ":4011"
			proxyDHCP := dhcp.NewProxyServer4011(proxyAddr, ifaceHandler)
			svcMgr.Register("proxy/"+iface.Name, "ProxyDHCP ("+iface.Name+")", proxyDHCP, iface.AutoStart, false, 4011, "UDP")
			slog.Info("ProxyDHCP 服务", "addr", proxyAddr, "interface", iface.Name)
		}
	}

	// 无接口配置时回退到 :67 + :4011 监听所有地址
	if !hasDHCP {
		dhcpServer := dhcp.NewServer("0.0.0.0:67", dhcpHandler)
		svcMgr.Register("dhcp/any", "DHCP", dhcpServer, true, false, 67, "UDP")
		proxyDHCP := dhcp.NewProxyServer4011("0.0.0.0:4011", dhcpHandler)
		svcMgr.Register("proxy/any", "ProxyDHCP", proxyDHCP, true, false, 4011, "UDP")
	}

	tftpServer := tftp.NewServer(config.DefaultPortTFTP, bootFS, bus)
	svcMgr.Register("tftp", "TFTP", tftpServer, cfg.ServiceAutoStart.TFTP, false, 69, "UDP")

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
	httpServer := httpd.NewServer(cfg, st, bus, bootFS, spaHandler(), dhcpHandler, netbootMgr, dhcpHandler.GetClientByIP, svcMgr, sessions)
	svcMgr.Register("http", "HTTP", httpServer, cfg.ServiceAutoStart.HTTP, true, 8080, "TCP")


	if appMode {
		slog.Info("自动打开浏览器")
		openBrowser("http://localhost:8080")
	}

	dnsServer := dns.NewServer(cfg, st, bus)
	svcMgr.Register("dns", "DNS", dnsServer, cfg.ServiceAutoStart.DNS, false, cfg.DNS.Port, "UDP")

	nfsServer := pxegonfs.NewServer(cfg.NFS.Port, cfg.NFS.RootDir, cfg.NFS.ReadOnly, cfg.NFS.AllowIPs)
	svcMgr.Register("nfs", "NFS", nfsServer, cfg.ServiceAutoStart.NFS, false, cfg.NFS.Port, "TCP")

	return svcMgr.Run(context.Background())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("程序退出", "error", err)
		os.Exit(1)
	}
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
