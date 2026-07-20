package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ConfigPath       string                  `yaml:"-" json:"-" mapstructure:"-"` // 配置文件的完整路径，运行时追踪用
	Global           GlobalConfig            `yaml:"global" mapstructure:"global"`
	Interfaces       []InterfaceConfig       `yaml:"interfaces" mapstructure:"interfaces"`
	Auth             AuthConfig              `yaml:"auth" mapstructure:"auth"`
	DNS              DNSConfig               `yaml:"dns" mapstructure:"dns"`
	NFS              NFSConfig               `yaml:"nfs" mapstructure:"nfs"`
	Boot             BootConfig              `yaml:"boot" mapstructure:"boot"`
	TFTP             TFTPConfig              `yaml:"tftp" mapstructure:"tftp"`
	Netboot          NetbootConfig           `yaml:"netboot" mapstructure:"netboot"`
	Store            StoreConfig             `yaml:"store" mapstructure:"store"`
	Log              LogConfig               `yaml:"log" mapstructure:"log"`
	ServiceAutoStart ServiceAutoStartConfig  `yaml:"service_auto_start" mapstructure:"service_auto_start"`
	BlacklistSeeds   []MACEntry              `yaml:"blacklist,omitempty" mapstructure:"blacklist,omitempty"`
	WhitelistSeeds   []WhitelistSeedEntry    `yaml:"whitelist,omitempty" mapstructure:"whitelist,omitempty"`
}

type ServiceAutoStartConfig struct {
	TFTP bool `yaml:"tftp" mapstructure:"tftp"`
	HTTP bool `yaml:"http" mapstructure:"http"` // default true
	DNS  bool `yaml:"dns" mapstructure:"dns"`
	NFS  bool `yaml:"nfs" mapstructure:"nfs"`
}

type GlobalConfig struct {
	DataDir          string `yaml:"data_dir" mapstructure:"data_dir"`
	AppMode          bool   `yaml:"app_mode" mapstructure:"app_mode"`
	ServerName       string `yaml:"server_name" mapstructure:"server_name"`
	ListenAddr       string `yaml:"listen_addr" mapstructure:"listen_addr"`
	WhitelistEnabled bool   `yaml:"whitelist_enabled" mapstructure:"whitelist_enabled"`
	PageSize         int    `yaml:"page_size" mapstructure:"page_size"`
}

type InterfaceConfig struct {
	Name        string         `yaml:"name" mapstructure:"name"`
	IP          string         `yaml:"ip" mapstructure:"ip"`
	Subnets     []SubnetConfig `yaml:"subnets" mapstructure:"subnets"`
	Bootloader  string         `yaml:"bootloader" mapstructure:"bootloader"`       // ipxe | pxelinux | grub2
	TFTP        bool           `yaml:"tftp" mapstructure:"tftp"`
	HTTP        bool           `yaml:"http" mapstructure:"http"`
	AutoStart   bool           `yaml:"auto_start" mapstructure:"auto_start"`      // auto-start DHCP/ProxyDHCP for this interface
}

type SubnetConfig struct {
	CIDR             string   `yaml:"cidr" mapstructure:"cidr"`
	DHCP             string   `yaml:"dhcp" mapstructure:"dhcp"`
	Pool             string   `yaml:"pool" mapstructure:"pool"`                // 兼容旧格式单地址池
	Pools            []string `yaml:"pools" mapstructure:"pools"`              // 多地址池 ["start1-end1", "start2-end2"]
	Gateway          string   `yaml:"gateway" mapstructure:"gateway"`
	DNSServers       string   `yaml:"dns_servers" mapstructure:"dns_servers"`
	NextServer       string   `yaml:"next_server" mapstructure:"next_server"`
	LeaseTime        int      `yaml:"lease_time" mapstructure:"lease_time"`
	WhitelistEnabled bool     `yaml:"whitelist_enabled" mapstructure:"whitelist_enabled"`
	ChainToIPXE      bool     `yaml:"chain_to_ipxe" mapstructure:"chain_to_ipxe"` // 子网级别：pxelinux/grub2 → chain-load to iPXE
}

type TFTPConfig struct {
	Port    int `yaml:"port" mapstructure:"port"`
	Timeout int `yaml:"timeout" mapstructure:"timeout"` // seconds, 0 = library default
}

type AuthConfig struct {
	Token     string `yaml:"token" mapstructure:"token"`
	TokenHash string `yaml:"token_hash" mapstructure:"token_hash"`
}

type DNSConfig struct {
	Enabled       bool   `yaml:"enabled" mapstructure:"enabled"`
	Port          int    `yaml:"port" mapstructure:"port"`
	Upstream      string `yaml:"upstream" mapstructure:"upstream"`
	LocalDomain   string `yaml:"local_domain" mapstructure:"local_domain"`
	DefaultRecord bool   `yaml:"default_record" mapstructure:"default_record"`
	// DefaultRecordIP 由保存设置时自动填充第一个接口 IP，前端不直接编辑
	DefaultRecordIP string `yaml:"default_record_ip" mapstructure:"default_record_ip"`
}

type NFSMountPoint struct {
	Label      string   `yaml:"label"       mapstructure:"label"`
	ExportPath string   `yaml:"export_path" mapstructure:"export_path"`
	LocalDir   string   `yaml:"local_dir"   mapstructure:"local_dir"`
	ReadOnly   bool     `yaml:"read_only"   mapstructure:"read_only"`
	AllowIPs   []string `yaml:"allow_ips"   mapstructure:"allow_ips"`
}

type NFSConfig struct {
	Enabled     bool            `yaml:"enabled"      mapstructure:"enabled"`
	Port        int             `yaml:"port"         mapstructure:"port"`
	MountPoints []NFSMountPoint `yaml:"mount_points" mapstructure:"mount_points"`
	// Deprecated: 旧格式字段，用于自动迁移
	RootDir  string   `yaml:"root_dir,omitempty"  mapstructure:"root_dir"`
	ReadOnly bool     `yaml:"read_only,omitempty" mapstructure:"read_only"`
	AllowIPs []string `yaml:"allow_ips,omitempty" mapstructure:"allow_ips"`
}

type NetbootConfig struct {
	Enabled        bool              `yaml:"enabled" mapstructure:"enabled"`
	DefaultBoot    string            `yaml:"default_boot" mapstructure:"default_boot"`
	FallbackOnline bool              `yaml:"fallback_online" mapstructure:"fallback_online"`
	MenuTitle      string            `yaml:"menu_title" mapstructure:"menu_title"`
	ScriptTemplate string            `yaml:"script_template" mapstructure:"script_template"`
	FailsafePrompt bool              `yaml:"failsafe_prompt" mapstructure:"failsafe_prompt"`
	ProxyHTTPS     bool              `yaml:"proxy_https" mapstructure:"proxy_https"`
	CacheEnabled   bool              `yaml:"cache_enabled" mapstructure:"cache_enabled"`
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

// ArchEntry 定义单个 PXE 架构的引导文件映射
type ArchEntry struct {
	IPXE       string `yaml:"ipxe" mapstructure:"ipxe"`
	PXELinux   string `yaml:"pxelinux" mapstructure:"pxelinux"`
	GRUB       string `yaml:"grub" mapstructure:"grub"`
	GRUBConfig string `yaml:"grub_config" mapstructure:"grub_config"`
}

type BootConfig struct {
	RootDir         string                `yaml:"root_dir" mapstructure:"root_dir"`
	DefaultMenu     DefaultMenuConfig     `yaml:"default_menu" mapstructure:"default_menu"`
	CatalogRedirect CatalogRedirectConfig `yaml:"catalog_redirect" mapstructure:"catalog_redirect"`
	CatalogDisplay  CatalogDisplayConfig  `yaml:"catalog_display" mapstructure:"catalog_display"`
	PXEConfigFile   string                `yaml:"pxe_config_file" mapstructure:"pxe_config_file"`
	GRUBConfigFile  string                `yaml:"grub_config_file" mapstructure:"grub_config_file"`
	ArchMap         map[int]ArchEntry     `yaml:"arch_map,omitempty" mapstructure:"arch_map,omitempty"`
}

type DefaultMenuConfig struct {
	Title           string      `yaml:"title" mapstructure:"title"`
	Timeout         int         `yaml:"timeout" mapstructure:"timeout"`
	Default         int         `yaml:"default" mapstructure:"default"`
	ListAllProfiles bool        `yaml:"list_all_profiles" mapstructure:"list_all_profiles"`
	Entries         []MenuEntry `yaml:"entries" mapstructure:"entries"`
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
	Level           string `yaml:"level" mapstructure:"level"`
	Format          string `yaml:"format" mapstructure:"format"`
	File            string `yaml:"file" mapstructure:"file"`                               // 日志文件路径，为空则不写文件
	MaxSizeMB       int    `yaml:"max_size_mb" mapstructure:"max_size_mb"`                 // 单文件最大体积 (MB)，0=不限制
	MaxBackups      int    `yaml:"max_backups" mapstructure:"max_backups"`                 // 保留轮转文件数，0=不限制
	MaxAgeDays      int    `yaml:"max_age_days" mapstructure:"max_age_days"`               // 保留天数，0=不限制
	Compress        bool   `yaml:"compress" mapstructure:"compress"`                       // 是否 gzip 压缩旧日志
	CleanupInterval int    `yaml:"cleanup_interval" mapstructure:"cleanup_interval"`       // 清理检查间隔（小时），0=不自动清理
}

func (c *Config) Validate() error {
	for _, iface := range c.Interfaces {
		for _, sn := range iface.Subnets {
			if sn.DHCP != "" && sn.DHCP != "server" && sn.DHCP != "proxy" && sn.DHCP != "off" {
				return fmt.Errorf("interface %s subnet %s: 无效的 DHCP 模式: %s", iface.Name, sn.CIDR, sn.DHCP)
			}
				if sn.ChainToIPXE && iface.Bootloader != "pxelinux" && iface.Bootloader != "grub2" {
					return fmt.Errorf("interface %s subnet %s: chain_to_ipxe 仅支持 pxelinux 或 grub2", iface.Name, sn.CIDR)
				}
		}
		if iface.Bootloader != "" && iface.Bootloader != "ipxe" && iface.Bootloader != "pxelinux" && iface.Bootloader != "grub2" && iface.Bootloader != "undionly" {
			return fmt.Errorf("interface %s: 无效的引导加载器: %s", iface.Name, iface.Bootloader)
		}
	}
	return nil
}

func DefaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".pxelab"
	}
	return filepath.Join(home, ".pxelab")
}

// MACEntry represents a MAC address with optional reason, used for seed entries in config.
type MACEntry struct {
	MAC    string `yaml:"mac" mapstructure:"mac"`
	Reason string `yaml:"reason,omitempty" mapstructure:"reason,omitempty"`
}

// WhitelistSeedEntry is a seed entry for whitelists (includes subnet).
type WhitelistSeedEntry struct {
	MAC    string `yaml:"mac" mapstructure:"mac"`
	Reason string `yaml:"reason,omitempty" mapstructure:"reason,omitempty"`
	Subnet string `yaml:"subnet" mapstructure:"subnet"`
}
