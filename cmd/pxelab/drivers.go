package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/drivers"
)

// driversCmd 是驱动包相关的运维工具集合。
var driversCmd = &cobra.Command{
	Use:   "drivers",
	Short: "驱动包工具（规模估算等）",
}

// driversStatsCmd 估算驱动包规模：文件数 / 总大小 / 指定架构下的
// initrd 直投行数与传输量。切换 netboot.driver_delivery=initrd 前用它评估
// 预启动开销（initrd 行数 ≈ 目标架构下的文件数）。
var driversStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "估算驱动包规模（文件数 / 大小 / initrd 直投行数）",
	Long: `估算 boot 根目录下 drivers/<名称>/ 各驱动包的规模。

输出每个包的文件数、总大小、包内架构，以及在目标架构下（按 x64/x86/arm64
子目录过滤后）的文件数与大小 —— 后者即 netboot.driver_delivery=initrd 时
将生成的 iPXE initrd 行数与预启动传输量。

boot 根目录解析顺序：--root 显式指定 > 配置文件 boot.root_dir > <数据目录>/boot
（数据目录默认 ~/.pxelab，可用 --data-dir 覆盖）。`,
	RunE: runDriversStats,
}

func init() {
	driversStatsCmd.Flags().String("root", "", "boot 根目录（默认取配置/data-dir）")
	driversStatsCmd.Flags().String("arch", "amd64", "目标架构（amd64/x64、x86、arm64；空 = 不过滤）")
	driversStatsCmd.Flags().Bool("json", false, "以 JSON 输出")
	driversCmd.AddCommand(driversStatsCmd)
	rootCmd.AddCommand(driversCmd)
}

// driversStatsTotals 汇总。
type driversStatsTotals struct {
	Packs       int   `json:"packs"`
	TotalFiles  int   `json:"total_files"`
	TotalBytes  int64 `json:"total_bytes"`
	InitrdRows  int   `json:"initrd_rows"` // = 目标架构过滤后文件数合计
	InitrdBytes int64 `json:"initrd_bytes"`
}

type driversStatsReport struct {
	Root    string               `json:"root"`
	Arch    string               `json:"arch"`
	Packs   []drivers.Projection `json:"packs"`
	Totals  driversStatsTotals   `json:"totals"`
	Skipped []string             `json:"skipped,omitempty"`
}

func runDriversStats(cmd *cobra.Command, _ []string) error {
	rootOpt, _ := cmd.Flags().GetString("root")
	arch, _ := cmd.Flags().GetString("arch")
	asJSON, _ := cmd.Flags().GetBool("json")

	root, err := resolveBootRoot(rootOpt)
	if err != nil {
		return err
	}
	driversDir := filepath.Join(root, "drivers")
	if fi, statErr := os.Stat(driversDir); statErr != nil || !fi.IsDir() {
		return fmt.Errorf("未找到驱动包目录: %s（用 --root 指定 boot 根目录）", driversDir)
	}

	entries, err := os.ReadDir(driversDir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	report := driversStatsReport{Root: root, Arch: arch}
	for _, name := range names {
		proj, err := drivers.Project(root, name, arch)
		if err != nil {
			report.Skipped = append(report.Skipped, fmt.Sprintf("%s (%v)", name, err))
			continue
		}
		report.Packs = append(report.Packs, *proj)
		report.Totals.Packs++
		report.Totals.TotalFiles += proj.TotalFiles
		report.Totals.TotalBytes += proj.TotalBytes
		report.Totals.InitrdRows += proj.Files
		report.Totals.InitrdBytes += proj.Bytes
	}

	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printDriversStats(report, arch)
	return nil
}

// resolveBootRoot 解析 boot 根目录：显式 --root > 配置文件 boot.root_dir > <数据目录>/boot。
func resolveBootRoot(root string) (string, error) {
	if root != "" {
		return filepath.Abs(root)
	}
	if cfg, err := config.LoadConfig(""); err == nil && cfg.Boot.RootDir != "" {
		return cfg.Boot.RootDir, nil
	}
	// LoadConfig 校验失败时仍尽量尊重 --data-dir（持久标志已绑定到 viper）
	if dd := viper.GetString("global.data_dir"); dd != "" {
		return filepath.Join(dd, "boot"), nil
	}
	return filepath.Join(config.DefaultDataDir(), "boot"), nil
}

func printDriversStats(r driversStatsReport, arch string) {
	archLabel := arch
	if archLabel == "" {
		archLabel = "（不过滤）"
	}
	fmt.Printf("PxeLab 驱动包规模估算\n")
	fmt.Printf("boot 根目录: %s\n", r.Root)
	fmt.Printf("目标架构:   %s\n\n", archLabel)

	if len(r.Packs) == 0 {
		fmt.Printf("（drivers/ 下没有可用驱动包）\n")
	} else {
		fmt.Printf("%-24s %6s %10s %12s %10s %12s  %s\n",
			"包名", "文件", "总大小", "过滤后文件", "initrd行", "过滤后大小", "架构")
		for _, p := range r.Packs {
			archs := "-"
			if len(p.Archs) > 0 {
				archs = joinComma(p.Archs)
			}
			fmt.Printf("%-24s %6d %10s %12d %10d %12s  %s\n",
				truncate(p.Pack, 24), p.TotalFiles, humanBytes(p.TotalBytes),
				p.Files, p.Files, humanBytes(p.Bytes), archs)
		}
		fmt.Printf("\n合计: %d 个包 · %d 个文件 · %s\n", r.Totals.Packs, r.Totals.TotalFiles, humanBytes(r.Totals.TotalBytes))
		fmt.Printf("initrd 直投: 将生成 %d 行 initrd，预启动传输 %s\n", r.Totals.InitrdRows, humanBytes(r.Totals.InitrdBytes))
	}

	if len(r.Skipped) > 0 {
		fmt.Printf("\n跳过（非法名/空包）: %s\n", joinComma(r.Skipped))
	}

	fmt.Printf("\n提示:\n")
	fmt.Printf("  · initrd 直投在 PE 启动前把每个驱动文件注入内存盘：行数≈文件数，\n")
	fmt.Printf("    PE 内存占用≈过滤后大小 + PE 本身；文件越多，引导越慢。\n")
	fmt.Printf("  · 若行数/体积过大，保持 netboot.driver_delivery=http（PE 内下载）。\n")
}

func humanBytes(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", f, units[i])
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func joinComma(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
