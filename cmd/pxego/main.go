package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"

	"github.com/pxego/pxego/internal/app"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/dhcp"
	"github.com/pxego/pxego/internal/dns"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/httpd"
	"github.com/pxego/pxego/internal/store"
	"github.com/pxego/pxego/internal/tftp"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pxego",
	Short: "PxeGo - 一体化 PXE 服务器",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 检查是否应该以 Windows 服务模式运行
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

	// 处理 --mode 简写标记
	if mode, _ := cmd.Flags().GetString("mode"); mode == "app" {
		cmd.Flags().Set("app-mode", "true")
	}

	appMode, _ := cmd.Flags().GetBool("app-mode")

	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化日志
	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetLogLoggerLevel(level)

	slog.Info("PxeGo 启动", "data_dir", cfg.Global.DataDir)

	// 初始化存储
	st, err := store.NewSQLite(cfg.Store.DSN)
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	if err := st.Migrate(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	defer st.Close()

	// 创建全局组件
	bus := eventbus.New()
	bootFS := boot.NewBootFileServer(cfg.Boot.RootDir)

	// 创建 DHCP LeaseManager（共享实例）
	leaseMgr := dhcp.NewLeaseManager(st)

	// 创建共享 DHCP Handler
	dhcpHandler := dhcp.NewHandler(cfg, st, bus, leaseMgr)
	dhcpHandler.InitSubnets()

	// 创建 App 编排器
	pxeApp := app.New()

	// 创建并注册各服务
	if len(cfg.Interfaces) > 0 {
		dhcpServer := dhcp.NewServer("0.0.0.0:67", dhcpHandler)
		pxeApp.Register(dhcpServer)

		proxyDHCP := dhcp.NewProxyServer4011("0.0.0.0:4011", dhcpHandler)
		pxeApp.Register(proxyDHCP)
	}

	tftpServer := tftp.NewServer(config.DefaultPortTFTP, bootFS, bus)
	pxeApp.Register(tftpServer)

	httpServer := httpd.NewServer(cfg, st, bus, bootFS, spaHandler())
	pxeApp.Register(httpServer)

	// 填充服务状态
	svc := httpServer.API().Services
	if len(cfg.Interfaces) > 0 {
		svc["DHCP"] = "running"
	}
	svc["TFTP"] = "running"
	svc["HTTP"] = "running"
	if len(cfg.Interfaces) > 0 && cfg.Interfaces[0].DNS {
		svc["DNS"] = "running"
	}

	// 自动打开浏览器
	if appMode {
		slog.Info("自动打开浏览器")
		openBrowser("http://localhost:8080")
	}

	if len(cfg.Interfaces) > 0 && cfg.Interfaces[0].DNS {
		dnsServer := dns.NewServer(config.DefaultPortDNS, "8.8.8.8:53", bus)
		pxeApp.Register(dnsServer)
	}

	return pxeApp.Run(context.Background())
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
