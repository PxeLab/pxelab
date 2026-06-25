package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ConfigPath      string                `yaml:"-" json:"-" mapstructure:"-"` // 配置文件的完整路径，运行时追踪用
	Global          GlobalConfig          `yaml:"global" mapstructure:"global"`
	Interfaces      []InterfaceConfig     `yaml:"interfaces" mapstructure:"interfaces"`
	Auth            AuthConfig            `yaml:"auth" mapstructure:"auth"`
	Boot            BootConfig            `yaml:"boot" mapstructure:"boot"`
	Netboot         NetbootConfig         `yaml:"netboot" mapstructure:"netboot"`
	Store           StoreConfig           `yaml:"store" mapstructure:"store"`
	Log             LogConfig             `yaml:"log" mapstructure:"log"`
	ServiceAutoStart ServiceAutoStartConfig `yaml:"service_auto_start" mapstructure:"service_auto_start"`
}

type ServiceAutoStartConfig struct {
	TFTP bool `yaml:"tftp" mapstructure:"tftp"`
	HTTP bool `yaml:"http" mapstructure:"http"` // default true
	DNS  bool `yaml:"dns" mapstructure:"dns"`
}

type GlobalConfig struct {
	DataDir    string `yaml:"data_dir" mapstructure:"data_dir"`
	AppMode    bool   `yaml:"app_mode" mapstructure:"app_mode"`
	ServerName string `yaml:"server_name" mapstructure:"server_name"`
}

type InterfaceConfig struct {
	Name        string         `yaml:"name" mapstructure:"name"`
	IP          string         `yaml:"ip" mapstructure:"ip"`
	Subnets     []SubnetConfig `yaml:"subnets" mapstructure:"subnets"`
	DHCP        string         `yaml:"dhcp" mapstructure:"dhcp"`                  // full | proxy | hybrid | off
	Bootloader  string         `yaml:"bootloader" mapstructure:"bootloader"`       // ipxe | pxelinux | grub2
	ChainToIPXE bool           `yaml:"chain_to_ipxe" mapstructure:"chain_to_ipxe"` // pxelinux/grub2 → chain-load to iPXE
	TFTP        bool           `yaml:"tftp" mapstructure:"tftp"`
	HTTP        bool           `yaml:"http" mapstructure:"http"`
	DNS         bool           `yaml:"dns" mapstructure:"dns"`
	AutoStart   bool           `yaml:"auto_start" mapstructure:"auto_start"`      // auto-start DHCP/ProxyDHCP for this interface
}

type SubnetConfig struct {
	CIDR       string   `yaml:"cidr" mapstructure:"cidr"`
	DHCP       string   `yaml:"dhcp" mapstructure:"dhcp"`
	Pool       string   `yaml:"pool" mapstructure:"pool"`                // 兼容旧格式单地址池
	Pools      []string `yaml:"pools" mapstructure:"pools"`              // 多地址池 ["start1-end1", "start2-end2"]
	Gateway    string   `yaml:"gateway" mapstructure:"gateway"`
	DNSServers string   `yaml:"dns_servers" mapstructure:"dns_servers"`
	NextServer string   `yaml:"next_server" mapstructure:"next_server"`
	LeaseTime  int      `yaml:"lease_time" mapstructure:"lease_time"`
}

type AuthConfig struct {
	Token string `yaml:"token" mapstructure:"token"`
}

type NetbootConfig struct {
	Enabled        bool              `yaml:"enabled" mapstructure:"enabled"`
	DefaultBoot    string            `yaml:"default_boot" mapstructure:"default_boot"`
	FallbackOnline bool              `yaml:"fallback_online" mapstructure:"fallback_online"`
	MenuTitle      string            `yaml:"menu_title" mapstructure:"menu_title"`
	ScriptTemplate string            `yaml:"script_template" mapstructure:"script_template"`
	Boot           BootConfig        `yaml:"boot" mapstructure:"boot"`
	Sync           NetbootSyncConfig `yaml:"sync" mapstructure:"sync"`
	Paths          NetbootPathConfig `yaml:"paths" mapstructure:"paths"`
}

type NetbootSyncConfig struct {
	Auto bool   `yaml:"auto" mapstructure:"auto"`
	Repo string `yaml:"repo" mapstructure:"repo"`
	URL  string `yaml:"url" mapstructure:"url"`
}

type NetbootPathConfig struct {
	Catalog   string `yaml:"catalog" mapstructure:"catalog"`
	Scripts   string `yaml:"scripts" mapstructure:"scripts"`
	BootFiles string `yaml:"boot_files" mapstructure:"boot_files"`
}

type BootConfig struct {
	RootDir         string                `yaml:"root_dir" mapstructure:"root_dir"`
	DefaultMenu     DefaultMenuConfig     `yaml:"default_menu" mapstructure:"default_menu"`
	ProfileBehavior ProfileBehaviorConfig `yaml:"profile_behavior" mapstructure:"profile_behavior"`
	CatalogRedirect CatalogRedirectConfig `yaml:"catalog_redirect" mapstructure:"catalog_redirect"`
	CatalogDisplay  CatalogDisplayConfig  `yaml:"catalog_display" mapstructure:"catalog_display"`
}

type DefaultMenuConfig struct {
	Title   string      `yaml:"title" mapstructure:"title"`
	Timeout int         `yaml:"timeout" mapstructure:"timeout"`
	Default int         `yaml:"default" mapstructure:"default"`
	Entries []MenuEntry `yaml:"entries" mapstructure:"entries"`
}

type MenuEntry struct {
	Label   string  `yaml:"label" mapstructure:"label"`
	Type    string  `yaml:"type" mapstructure:"type"`
	Kernel  *string `yaml:"kernel,omitempty" mapstructure:"kernel,omitempty"`
	Initrd  *string `yaml:"initrd,omitempty" mapstructure:"initrd,omitempty"`
	Cmdline *string `yaml:"cmdline,omitempty" mapstructure:"cmdline,omitempty"`
	URL     *string `yaml:"url,omitempty" mapstructure:"url,omitempty"`
	WIM     *string `yaml:"wim,omitempty" mapstructure:"wim,omitempty"`
}

type ProfileBehaviorConfig struct {
	AppendLocal    bool   `yaml:"append_local" mapstructure:"append_local"`
	AppendNetboot  bool   `yaml:"append_netboot" mapstructure:"append_netboot"`
	AppendPosition string `yaml:"append_position" mapstructure:"append_position"`
}

type CatalogRedirectConfig struct {
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	TargetURL  string `yaml:"target_url" mapstructure:"target_url"`
	DetectArch bool   `yaml:"detect_arch" mapstructure:"detect_arch"`
	Preamble   string `yaml:"preamble" mapstructure:"preamble"`
}

type CatalogDisplayConfig struct {
	Title  string         `yaml:"title" mapstructure:"title"`
	Groups []CatalogGroup `yaml:"groups" mapstructure:"groups"`
}

type CatalogGroup struct {
	Name    string `yaml:"name" mapstructure:"name"`
	Title   string `yaml:"title" mapstructure:"title"`
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	Order   int    `yaml:"order" mapstructure:"order"`
}

type StoreConfig struct {
	DSN string `yaml:"dsn" mapstructure:"dsn"`
}

type LogConfig struct {
	Level  string `yaml:"level" mapstructure:"level"`
	Format string `yaml:"format" mapstructure:"format"`
	File   string `yaml:"file" mapstructure:"file"` // 日志文件路径，为空则不写文件
}

func (c *Config) Validate() error {
	for _, iface := range c.Interfaces {
		if iface.DHCP != "" && iface.DHCP != "full" && iface.DHCP != "proxy" && iface.DHCP != "hybrid" && iface.DHCP != "off" {
			return fmt.Errorf("interface %s: 无效的 DHCP 模式: %s", iface.Name, iface.DHCP)
		}
		if iface.Bootloader != "" && iface.Bootloader != "ipxe" && iface.Bootloader != "pxelinux" && iface.Bootloader != "grub2" && iface.Bootloader != "undionly" {
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
