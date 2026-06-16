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
			DataDir: dataDir,
		},
		Boot: BootConfig{
			RootDir: "./boot",
		},
		Store: StoreConfig{
			DSN: filepath.Join(dataDir, "pxego.db"),
		},
		Log: LogConfig{
			Level: DefaultLogLevel,
		},
	}
}
