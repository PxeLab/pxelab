package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/pxelab/pxelab/internal/updatecheck"
)

// applyPlan 是由当前进程写给更新 helper 的安装计划。
// helper 以同一二进制启动，等旧进程退出后替换文件并重启。
type applyPlan struct {
	ParentPID int      `json:"parent_pid"`
	Target    string   `json:"target"`
	NewBinary string   `json:"new_binary"`
	Backup    string   `json:"backup"`
	Service   bool     `json:"service"`
	Args      []string `json:"args"`
	WorkDir   string   `json:"work_dir"`
	LogFile   string   `json:"log_file"`
}

// startApply 启动后台 helper；helper 会等待本进程退出后替换并重启。
func startApply(staged *updatecheck.StagedUpdate) (*applyPlan, error) {
	target, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("获取当前程序路径失败: %w", err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("解析当前程序路径失败: %w", err)
	}

	updatesDir := filepath.Dir(staged.BinaryPath)
	helperName := "pxelab-update-helper"
	if runtime.GOOS == "windows" {
		helperName += ".exe"
	}
	helperPath := filepath.Join(updatesDir, helperName)
	if err := copyExecutable(target, helperPath); err != nil {
		return nil, fmt.Errorf("准备更新助手失败: %w", err)
	}

	wd, _ := os.Getwd()
	plan := &applyPlan{
		ParentPID: os.Getpid(),
		Target:    target,
		NewBinary: staged.BinaryPath,
		Backup:    target + ".bak",
		Service:   applyServiceMode(),
		Args:      append([]string(nil), os.Args[1:]...),
		WorkDir:   wd,
		LogFile:   filepath.Join(updatesDir, "apply-"+staged.Version+".log"),
	}

	planPath := filepath.Join(updatesDir, "apply-"+staged.Version+".json")
	if err := writePlan(planPath, plan); err != nil {
		return nil, fmt.Errorf("写入安装计划失败: %w", err)
	}

	logFile, err := os.OpenFile(plan.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("创建安装日志失败: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(helperPath, "__apply-update", "--plan", planPath)
	cmd.Dir = updatesDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	configureDetached(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动更新助手失败: %w", err)
	}
	return plan, nil
}

func writePlan(path string, plan *applyPlan) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// runApplyHelper 是隐藏子命令的入口，返回进程退出码。
func runApplyHelper(args []string) int {
	fs := flag.NewFlagSet("__apply-update", flag.ContinueOnError)
	planPath := fs.String("plan", "", "安装计划路径")
	if err := fs.Parse(args); err != nil || *planPath == "" {
		fmt.Fprintln(os.Stderr, "invalid apply plan")
		return 2
	}

	data, err := os.ReadFile(*planPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read plan:", err)
		return 1
	}
	var plan applyPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		fmt.Fprintln(os.Stderr, "parse plan:", err)
		return 1
	}

	logFile, err := os.OpenFile(plan.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open log:", err)
		return 1
	}
	defer logFile.Close()
	logger := log.New(logFile, "[update] ", log.LstdFlags)
	logger.Printf("helper started: parent=%d target=%s service=%v", plan.ParentPID, plan.Target, plan.Service)

	if plan.Service {
		// 服务模式先停止服务，避免 systemd/sc 拉起旧二进制
		if err := applyStopService(); err != nil {
			logger.Printf("stop service: %v", err)
		}
	}

	if err := applyWaitForProcess(plan.ParentPID, 90*time.Second); err != nil {
		logger.Printf("wait for parent: %v", err)
	}
	// 给服务管理器/文件句柄一点释放时间
	time.Sleep(500 * time.Millisecond)

	if err := installUpdate(&plan, logger); err != nil {
		logger.Printf("install failed: %v", err)
		rollbackUpdate(&plan, logger)
		return 1
	}
	logger.Printf("update installed, restart requested")
	return 0
}

func installUpdate(plan *applyPlan, logger *log.Logger) error {
	_ = os.Remove(plan.Backup)
	if err := os.Rename(plan.Target, plan.Backup); err != nil {
		return fmt.Errorf("备份旧程序失败: %w", err)
	}

	if err := copyExecutable(plan.NewBinary, plan.Target); err != nil {
		_ = os.Rename(plan.Backup, plan.Target)
		return fmt.Errorf("写入新程序失败: %w", err)
	}

	if plan.Service {
		if err := applyStartService(); err != nil {
			return fmt.Errorf("启动服务失败: %w", err)
		}
	} else {
		if err := applyStartDetached(plan.Target, plan.Args, plan.WorkDir); err != nil {
			return fmt.Errorf("启动新程序失败: %w", err)
		}
	}
	logger.Printf("new binary installed at %s", plan.Target)
	return nil
}

func rollbackUpdate(plan *applyPlan, logger *log.Logger) {
	logger.Printf("rolling back to %s", plan.Backup)
	_ = os.Remove(plan.Target)
	if err := os.Rename(plan.Backup, plan.Target); err != nil {
		logger.Printf("rollback rename failed: %v", err)
		return
	}
	if plan.Service {
		if err := applyStartService(); err != nil {
			logger.Printf("rollback start service failed: %v", err)
		}
	} else {
		if err := applyStartDetached(plan.Target, plan.Args, plan.WorkDir); err != nil {
			logger.Printf("rollback start process failed: %v", err)
		}
	}
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	_ = os.Chmod(tmp, 0755)
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
