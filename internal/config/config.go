package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ConfigPath string            `yaml:"-" json:"-" mapstructure:"-"` // 配置文件的完整路径，运行时追踪用
	Global     GlobalConfig      `mapstructure:"global"`
	Interfaces []InterfaceConfig `mapstructure:"interfaces"`
	Auth       AuthConfig        `mapstructure:"auth"`
	Boot       BootConfig        `mapstructure:"boot"`
	Netboot    NetbootConfig     `mapstructure:"netboot"`
	Store      StoreConfig       `mapstructure:"store"`
	Log        LogConfig         `mapstructure:"log"`
}

type GlobalConfig struct {
	DataDir    string `mapstructure:"data_dir"`
	AppMode    bool   `mapstructure:"app_mode"`
	ServerName string `mapstructure:"server_name"`
}

type InterfaceConfig struct {
	Name        string         `mapstructure:"name"`
	IP          string         `mapstructure:"ip"`
	Subnets     []SubnetConfig `mapstructure:"subnets"`
	DHCP        string         `mapstructure:"dhcp"`         // full | proxy | hybrid | off
	Bootloader  string         `mapstructure:"bootloader"`  // ipxe | pxelinux | grub2
	ChainToIPXE bool           `mapstructure:"chain_to_ipxe"` // pxelinux/grub2 → chain-load to iPXE
	TFTP        bool           `mapstructure:"tftp"`
	HTTP        bool           `mapstructure:"http"`
	DNS         bool           `mapstructure:"dns"`
}

type SubnetConfig struct {
	CIDR       string   `mapstructure:"cidr"`
	DHCP       string   `mapstructure:"dhcp"`
	Pool       string   `mapstructure:"pool"`       // 兼容旧格式单地址池
	Pools      []string `mapstructure:"pools"`      // 多地址池 ["start1-end1", "start2-end2"]
	Gateway    string   `mapstructure:"gateway"`
	DNSServers string   `mapstructure:"dns_servers"`
	NextServer string   `mapstructure:"next_server"`
	LeaseTime  int      `mapstructure:"lease_time"`
}

type AuthConfig struct {
	Token string `mapstructure:"token"`
}

type NetbootConfig struct {
	Enabled        bool              `mapstructure:"enabled"`
	DefaultBoot    string            `mapstructure:"default_boot"`
	FallbackOnline bool              `mapstructure:"fallback_online"`
	MenuTitle      string            `mapstructure:"menu_title"`
	ScriptTemplate string            `mapstructure:"script_template"`
	Boot           BootConfig        `mapstructure:"boot"`
	Sync           NetbootSyncConfig `mapstructure:"sync"`
	Paths          NetbootPathConfig `mapstructure:"paths"`
}

type NetbootSyncConfig struct {
	Auto bool   `mapstructure:"auto"`
	Repo string `mapstructure:"repo"`
	URL  string `mapstructure:"url"`
}

type NetbootPathConfig struct {
	Catalog   string `mapstructure:"catalog"`
	Scripts   string `mapstructure:"scripts"`
	BootFiles string `mapstructure:"boot_files"`
}

type BootConfig struct {
	RootDir         string                `mapstructure:"root_dir"`
	DefaultMenu     DefaultMenuConfig     `mapstructure:"default_menu"`
	ProfileBehavior ProfileBehaviorConfig `mapstructure:"profile_behavior"`
	CatalogRedirect CatalogRedirectConfig `mapstructure:"catalog_redirect"`
	CatalogDisplay  CatalogDisplayConfig  `mapstructure:"catalog_display"`
}

type DefaultMenuConfig struct {
	Title   string      `mapstructure:"title"`
	Timeout int         `mapstructure:"timeout"`
	Default int         `mapstructure:"default"`
	Entries []MenuEntry `mapstructure:"entries"`
}

type MenuEntry struct {
	Label   string  `mapstructure:"label"`
	Type    string  `mapstructure:"type"`
	Kernel  *string `mapstructure:"kernel,omitempty"`
	Initrd  *string `mapstructure:"initrd,omitempty"`
	Cmdline *string `mapstructure:"cmdline,omitempty"`
	URL     *string `mapstructure:"url,omitempty"`
	WIM     *string `mapstructure:"wim,omitempty"`
}

type ProfileBehaviorConfig struct {
	AppendLocal    bool   `mapstructure:"append_local"`
	AppendNetboot  bool   `mapstructure:"append_netboot"`
	AppendPosition string `mapstructure:"append_position"`
}

type CatalogRedirectConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	TargetURL  string `mapstructure:"target_url"`
	DetectArch bool   `mapstructure:"detect_arch"`
	Preamble   string `mapstructure:"preamble"`
}

type CatalogDisplayConfig struct {
	Title  string         `mapstructure:"title"`
	Groups []CatalogGroup `mapstructure:"groups"`
}

type CatalogGroup struct {
	Name    string `mapstructure:"name"`
	Title   string `mapstructure:"title"`
	Enabled bool   `mapstructure:"enabled"`
	Order   int    `mapstructure:"order"`
}

type StoreConfig struct {
	DSN string `mapstructure:"dsn"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	File   string `mapstructure:"file"` // 日志文件路径，为空则不写文件
}

func (c *Config) Validate() error {
	for _, iface := range c.Interfaces {
		if iface.DHCP != "" && iface.DHCP != "full" && iface.DHCP != "proxy" && iface.DHCP != "hybrid" && iface.DHCP != "off" {
			return fmt.Errorf("interface %s: 无效的 DHCP 模式: %s", iface.Name, iface.DHCP)
		}
		if iface.Bootloader != "" && iface.Bootloader != "ipxe" && iface.Bootloader != "pxelinux" && iface.Bootloader != "grub2" {
			return fmt.Errorf("interface %s: 无效的引导加载器: %s", iface.Name, iface.Bootloader)
		}
		if iface.ChainToIPXE && iface.Bootloader != "pxelinux" && iface.Bootloader != "grub2" {
			return fmt.Errorf("interface %s: chain_to_ipxe 仅支持 pxelinux 或 grub2", iface.Name)
		}
	}
	return nil
}

func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".pxego"
	}
	return filepath.Join(home, ".pxego")
}
