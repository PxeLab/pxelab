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
	Name       string         `mapstructure:"name"`
	IP         string         `mapstructure:"ip"`
	Subnets    []SubnetConfig `mapstructure:"subnets"`
	DHCP       string         `mapstructure:"dhcp"` // full | proxy | hybrid | off
	Bootloader string         `mapstructure:"bootloader"` // ipxe | pxelinux | grub2
	TFTP       bool           `mapstructure:"tftp"`
	HTTP       bool           `mapstructure:"http"`
	DNS        bool           `mapstructure:"dns"`
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

type BootConfig struct {
	RootDir string `mapstructure:"root_dir"`
}

type NetbootConfig struct {
	Enabled        bool              `mapstructure:"enabled"`
	DefaultBoot    string            `mapstructure:"default_boot"`
	FallbackOnline bool              `mapstructure:"fallback_online"`
	MenuTitle      string            `mapstructure:"menu_title"`
	Sync           NetbootSyncConfig `mapstructure:"sync"`
	Paths          NetbootPathConfig `mapstructure:"paths"`
}

type NetbootSyncConfig struct {
	Auto bool   `mapstructure:"auto"`
	Repo string `mapstructure:"repo"`
}

type NetbootPathConfig struct {
	Catalog   string `mapstructure:"catalog"`
	Scripts   string `mapstructure:"scripts"`
	BootFiles string `mapstructure:"boot_files"`
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
