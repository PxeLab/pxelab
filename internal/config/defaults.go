package config

import "path/filepath"

const (
	DefaultPortDHCP     = 67
	DefaultPortDHCP4011 = 4011
	DefaultPortTFTP     = 69
	DefaultPortDNS      = 53
	DefaultPortHTTP     = 8080
	DefaultLeaseTime    = 3600
	DefaultLogLevel     = "info"
)

func DefaultConfig() *Config {
	dataDir := DefaultDataDir()
	return &Config{
		Global: GlobalConfig{
			DataDir:    dataDir,
			ServerName: "pxego",
		},
		Boot: BootConfig{
			RootDir: "./boot",
		},
		Netboot: NetbootConfig{
			Enabled:        true,
			DefaultBoot:    "menu",
			FallbackOnline: true,
			MenuTitle:      "[OS] Netboot OS Install Catalog",
			Sync: NetbootSyncConfig{
				Auto: false,
				Repo: "contrib/netboot.xyz",
			},
			Paths: NetbootPathConfig{
				Catalog:   "netboot/catalog",
				Scripts:   "netboot/scripts",
				BootFiles: "boot/netboot",
			},
		},
		Store: StoreConfig{
			DSN: filepath.Join(dataDir, "pxego.db"),
		},
		Log: LogConfig{
			Level: DefaultLogLevel,
		},
	}
}
