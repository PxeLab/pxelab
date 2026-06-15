package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/pxego/pxego/internal/app"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/store"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "pxego",
	Short: "PxeGo - 一体化 PXE 服务器",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func init() {
	config.BindFlags(rootCmd)
}

func run(cmd *cobra.Command) error {
	cfgFile, _ := cmd.Flags().GetString("config")
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化日志
	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetLogLoggerLevel(level)

	slog.Info("PxeGo 启动", "data_dir", cfg.Global.DataDir)

	// 初始化数据库
	st, err := store.NewSQLite(cfg.Store.DSN)
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	if err := st.Migrate(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	defer st.Close()

	// 创建 App
	pxeApp := app.New()
	// TODO: 后续注册各服务
	_ = pxeApp

	return pxeApp.Run(context.Background())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("程序退出", "error", err)
		os.Exit(1)
	}
}
