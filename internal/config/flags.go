package config

import (
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func BindFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("config", "", "配置文件路径")
	cmd.PersistentFlags().String("data-dir", DefaultDataDir(), "数据目录")
	cmd.PersistentFlags().String("log-level", "info", "日志级别")
	cmd.PersistentFlags().String("mode", "", `运行模式 ("app" 将自动打开浏览器)`)

	viper.BindPFlag("global.data_dir", cmd.PersistentFlags().Lookup("data-dir"))
	viper.BindPFlag("log.level", cmd.PersistentFlags().Lookup("log-level"))
}

func LoadConfig(cfgPath string) (*Config, error) {
	v := viper.GetViper()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath(DefaultDataDir())
	v.AddConfigPath("/etc/pxelab")

	if cfgPath != "" {
		v.SetConfigFile(cfgPath)
	}

	v.SetDefault("global.listen_addr", ":8080")
	v.SetDefault("store.dsn", "")
	v.SetDefault("boot.root_dir", "")
	v.SetDefault("log.level", "info")

	// service auto-start defaults
	v.SetDefault("service_auto_start.http", true)
	v.SetDefault("service_auto_start.tftp", false)
	v.SetDefault("service_auto_start.dns", false)
	v.SetDefault("service_auto_start.nfs", false)

	// nfs defaults
	v.SetDefault("nfs.port", DefaultPortNFS)
	v.SetDefault("nfs.mount_points", []NFSMountPoint{})
	v.SetDefault("nfs.root_dir", "")
	v.SetDefault("nfs.read_only", true)

	// dns defaults
	v.SetDefault("dns.port", DefaultPortDNS)
	v.SetDefault("dns.upstream", "")
	v.SetDefault("dns.local_domain", "pxelab.local")

	// netboot defaults
	v.SetDefault("netboot.enabled", true)
	v.SetDefault("netboot.failsafe_prompt", true)
	v.SetDefault("netboot.sync.url", "https://github.com/netbootxyz/netboot.xyz.git")
	v.SetDefault("netboot.sync.repo", "contrib/netboot.xyz")
	v.SetDefault("netboot.paths.catalog", "netboot/catalog")
	v.SetDefault("netboot.paths.scripts", "netboot/scripts")
	v.SetDefault("netboot.paths.boot_files", "boot/netboot")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// 记录配置文件路径，保存时写回同一位置
	if used := v.ConfigFileUsed(); used != "" {
		cfg.ConfigPath = used
	} else if cfgPath != "" {
		cfg.ConfigPath = cfgPath
	}

	// 确保 DataDir 有默认值
	if cfg.Global.DataDir == "" {
		cfg.Global.DataDir = DefaultDataDir()
	}

	// 根据 data_dir 派生默认值
	dd := cfg.Global.DataDir
	if cfg.Store.DSN == "" && dd != "" {
		cfg.Store.DSN = filepath.Join(dd, "pxelab.db")
	}
	if cfg.Boot.RootDir == "" && dd != "" {
		cfg.Boot.RootDir = filepath.Join(dd, "boot")
	}
	if cfg.NFS.RootDir == "" && dd != "" {
		cfg.NFS.RootDir = filepath.Join(dd, "boot", "isos")
	}

	// 旧格式自动迁移：如果 MountPoints 为空但 RootDir 有值，转为单个挂载点
	if len(cfg.NFS.MountPoints) == 0 && cfg.NFS.RootDir != "" {
		cfg.NFS.MountPoints = []NFSMountPoint{{
			Label:      "Default",
			ExportPath: "/",
			LocalDir:   cfg.NFS.RootDir,
			ReadOnly:   cfg.NFS.ReadOnly,
			AllowIPs:   cfg.NFS.AllowIPs,
		}}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
