package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Global     GlobalConfig      `mapstructure:"global"`
	Interfaces []InterfaceConfig `mapstructure:"interfaces"`
	Auth       AuthConfig        `mapstructure:"auth"`
	Boot       BootConfig        `mapstructure:"boot"`
	Store      StoreConfig       `mapstructure:"store"`
	Log        LogConfig         `mapstructure:"log"`
}

type GlobalConfig struct {
	DataDir string `mapstructure:"data_dir"`
	AppMode bool   `mapstructure:"app_mode"`
}

type InterfaceConfig struct {
	Name    string         `mapstructure:"name"`
	IP      string         `mapstructure:"ip"`
	Subnets []SubnetConfig `mapstructure:"subnets"`
	DHCP    string         `mapstructure:"dhcp"` // full | proxy | hybrid | off
	TFTP    bool           `mapstructure:"tftp"`
	HTTP    bool           `mapstructure:"http"`
	DNS     bool           `mapstructure:"dns"`
}

type SubnetConfig struct {
	CIDR       string `mapstructure:"cidr"`
	DHCP       string `mapstructure:"dhcp"`
	Pool       string `mapstructure:"pool"`
	Gateway    string `mapstructure:"gateway"`
	DNSServers string `mapstructure:"dns_servers"`
	NextServer string `mapstructure:"next_server"`
	LeaseTime  int    `mapstructure:"lease_time"`
}

type AuthConfig struct {
	Token string `mapstructure:"token"`
}

type BootConfig struct {
	RootDir string `mapstructure:"root_dir"`
}

type StoreConfig struct {
	DSN string `mapstructure:"dsn"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func (c *Config) Validate() error {
	for _, iface := range c.Interfaces {
		if iface.DHCP != "" && iface.DHCP != "full" && iface.DHCP != "proxy" && iface.DHCP != "hybrid" && iface.DHCP != "off" {
			return fmt.Errorf("interface %s: 无效的 DHCP 模式: %s", iface.Name, iface.DHCP)
		}
	}
	return nil
}

func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pxego")
}
