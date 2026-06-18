package main

import (
	"fmt"
	"path/filepath"

	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/netboot"
	"github.com/spf13/cobra"
)

var netbootCmd = &cobra.Command{
	Use:   "netboot",
	Short: "管理 netboot.xyz 操作系统目录",
}

var netbootListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有可用发行版",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig("")
		if err != nil {
			// Use defaults if config not found
			cfg = &config.Config{Global: config.GlobalConfig{DataDir: config.DefaultDataDir()}}
		}
		catalogDir := resolveNetbootPath(cfg, "catalog")
		cat, err := netboot.LoadCatalog(catalogDir)
		if err != nil {
			cat = netboot.DefaultCatalog()
		}
		for _, d := range cat.Distros {
			status := "✓"
			if !d.Enabled {
				status = "✗"
			}
			fmt.Printf(" %s %s (%s, %d versions)\n", status, d.Name, d.MenuGroup, len(d.Versions))
		}
		return nil
	},
}

var netbootInfoCmd = &cobra.Command{
	Use:   "info [distro]",
	Short: "查看发行版详细信息",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		cfg, err := config.LoadConfig("")
		if err != nil {
			cfg = &config.Config{Global: config.GlobalConfig{DataDir: config.DefaultDataDir()}}
		}
		catalogDir := resolveNetbootPath(cfg, "catalog")
		cat, err := netboot.LoadCatalog(catalogDir)
		if err != nil {
			return fmt.Errorf("无法加载目录: %w", err)
		}
		for _, d := range cat.Distros {
			if d.Name == name {
				fmt.Printf("名称: %s\n", d.Name)
				fmt.Printf("启用: %v\n", d.Enabled)
				fmt.Printf("分类: %s\n", d.MenuGroup)
				fmt.Printf("镜像: %s\n", d.Mirror)
				fmt.Printf("版本:\n")
				for _, v := range d.Versions {
					hasLocal := "远程"
					if v.Local != nil {
						hasLocal = "本地"
					}
					fmt.Printf("  - %s (%s) [%s] [%s]\n", v.Codename, v.Name, v.Arch, hasLocal)
				}
				return nil
			}
		}
		return fmt.Errorf("未找到发行版: %s", name)
	},
}

var netbootSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "从上游 netboot.xyz 同步发行版定义",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig("")
		if err != nil {
			return fmt.Errorf("加载配置失败: %w", err)
		}
		dataDir := cfg.Global.DataDir
		if dataDir == "" {
			dataDir = config.DefaultDataDir()
		}
		catalogDir := filepath.Join(dataDir, cfg.Netboot.Paths.Catalog)
		repoDir := cfg.Netboot.Sync.Repo
		if !filepath.IsAbs(repoDir) {
			repoDir = filepath.Join(dataDir, "..", repoDir)
		}

		fmt.Println("正在从 netboot.xyz 同步发行版定义...")
		if err := netboot.SyncFromUpstream(repoDir, catalogDir, cfg.Netboot.Sync.URL); err != nil {
			return fmt.Errorf("同步失败: %w", err)
		}
		fmt.Println("同步完成")
		return nil
	},
}

func resolveNetbootPath(cfg *config.Config, key string) string {
	dataDir := cfg.Global.DataDir
	if dataDir == "" {
		dataDir = config.DefaultDataDir()
	}
	switch key {
	case "catalog":
		return filepath.Join(dataDir, cfg.Netboot.Paths.Catalog)
	case "scripts":
		return filepath.Join(dataDir, cfg.Netboot.Paths.Scripts)
	}
	return filepath.Join(dataDir, "netboot")
}

func init() {
	netbootCmd.AddCommand(netbootListCmd)
	netbootCmd.AddCommand(netbootInfoCmd)
	netbootCmd.AddCommand(netbootSyncCmd)
	rootCmd.AddCommand(netbootCmd)
}
