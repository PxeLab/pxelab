package config

import "path/filepath"

const (
	DefaultPortDHCP     = 67
	DefaultPortDHCP4011 = 4011
	DefaultPortTFTP     = 69
	DefaultPortDNS      = 53
	DefaultPortNFS      = 2049
	DefaultPortHTTP     = 8080
	DefaultLeaseTime    = 3600
	DefaultLogLevel     = "info"
)

func DefaultConfig() *Config {
	dataDir := DefaultDataDir()
	return &Config{
		Global: GlobalConfig{
			DataDir:    dataDir,
			ServerName: "PxeLab",
			ListenAddr: ":8080",
			PageSize:   50,
		},
		Boot: BootConfig{
			RootDir:        "./boot",
			PXEConfigFile:  "pxelinux.cfg/default",
			GRUBConfigFile: "grub2/grub.cfg",
		},
		Netboot: NetbootConfig{
			Enabled:        true,
			DefaultBoot:    "menu",
			FallbackOnline: true,
			FailsafePrompt: true,
			ProxyHTTPS:     true,
			CacheEnabled:   true,
			MenuTitle:      "[OS] Netboot OS Install Catalog",
			Sync: NetbootSyncConfig{
				Auto: false,
				URL:  "https://github.com/netbootxyz/netboot.xyz.git",
				Repo: "contrib/netboot.xyz",
			},
			Paths: NetbootPathConfig{
				Catalog:   "netboot/catalog",
				Scripts:   "netboot/scripts",
				BootFiles: "boot/netboot",
			},
		},
		Store: StoreConfig{
			DSN: filepath.Join(dataDir, "pxelab.db"),
		},
		NFS: NFSConfig{
			Port: DefaultPortNFS,
			MountPoints: []NFSMountPoint{{
				Label:      "ISOs",
				ExportPath: "/",
				LocalDir:   filepath.Join(dataDir, "boot", "isos"),
				ReadOnly:   true,
			}},
		},
		DNS: DNSConfig{
			Port:        DefaultPortDNS,
			Upstream:    "",
			LocalDomain: "pxelab.local",
		},
		Log: LogConfig{
			Level: DefaultLogLevel,
		},
		ServiceAutoStart: ServiceAutoStartConfig{
			HTTP: true,
		},
	}
}
