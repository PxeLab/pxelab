package config

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
	return &Config{
		Global: GlobalConfig{
			DataDir: DefaultDataDir(),
		},
		Boot: BootConfig{
			RootDir: "/etc/pxego/boot",
		},
		Store: StoreConfig{
			DSN: "pxego.db",
		},
		Log: LogConfig{
			Level: DefaultLogLevel,
		},
	}
}
