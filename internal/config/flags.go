package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func BindFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("config", "", "配置文件路径")
	cmd.PersistentFlags().String("data-dir", DefaultDataDir(), "数据目录")
	cmd.PersistentFlags().String("log-level", "info", "日志级别")
	cmd.PersistentFlags().Bool("app-mode", false, "应用模式（自动打开浏览器）")

	viper.BindPFlag("global.data_dir", cmd.PersistentFlags().Lookup("data-dir"))
	viper.BindPFlag("log.level", cmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("global.app_mode", cmd.PersistentFlags().Lookup("app-mode"))
}

func LoadConfig(cfgPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.pxego")
	v.AddConfigPath("/etc/pxego")

	if cfgPath != "" {
		v.SetConfigFile(cfgPath)
	}

	v.SetDefault("store.dsn", "pxego.db")
	v.SetDefault("boot.root_dir", "/etc/pxego/boot")
	v.SetDefault("log.level", "info")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
