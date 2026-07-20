package logbus

import (
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RotatingFile 是一个支持大小轮转、备份数限制、过期清理和 gzip 压缩的日志文件写入器。
type RotatingFile struct {
	dir        string
	baseName   string
	maxSize    int64 // 字节，0=不限制
	maxBackups int   // 0=不限制
	maxAge     time.Duration
	compress   bool

	mu            sync.Mutex
	f             *os.File
	currentSize   int64
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

type RotatingConfig struct {
	Dir            string
	BaseName       string
	MaxSizeMB      int
	MaxBackups     int
	MaxAgeDays     int
	Compress       bool
	CleanupHours   int // 0=不自动清理
}

func NewRotatingFile(cfg RotatingConfig) (*RotatingFile, error) {
	if err := os.MkdirAll(cfg.Dir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	rf := &RotatingFile{
		dir:      cfg.Dir,
		baseName: cfg.BaseName,
		maxSize:  int64(cfg.MaxSizeMB) * 1024 * 1024,
		maxBackups: cfg.MaxBackups,
		compress: cfg.Compress,
	}
	if cfg.MaxAgeDays > 0 {
		rf.maxAge = time.Duration(cfg.MaxAgeDays) * 24 * time.Hour
	}

	// 获取当前文件大小
	fpath := filepath.Join(rf.dir, rf.baseName)
	if info, err := os.Stat(fpath); err == nil {
		rf.currentSize = info.Size()
	}

	// 打开当前日志文件
	f, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("打开日志文件失败: %w", err)
	}
	rf.f = f

	// 启动后台清理协程
	if cfg.CleanupHours > 0 {
		rf.stopCleanup = make(chan struct{})
		rf.cleanupTicker = time.NewTicker(time.Duration(cfg.CleanupHours) * time.Hour)
		go rf.cleanupLoop()
	}

	return rf, nil
}

func (rf *RotatingFile) Write(p []byte) (n int, err error) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.f == nil {
		return 0, fmt.Errorf("文件未打开")
	}

	// 检查是否需要轮转
	if rf.maxSize > 0 && rf.currentSize+int64(len(p)) > rf.maxSize {
		if err := rf.rotate(); err != nil {
			slog.Error("日志轮转失败", "file", rf.baseName, "error", err)
			// 轮转失败仍然继续写入，避免丢失日志
		}
	}

	n, err = rf.f.Write(p)
	rf.currentSize += int64(n)
	return
}

// rotate 执行日志轮转：当前文件 → .1，旧备份依次递增，超过 maxBackups 的删除
func (rf *RotatingFile) rotate() error {
	if rf.f != nil {
		rf.f.Close()
		rf.f = nil
	}

	basePath := filepath.Join(rf.dir, rf.baseName)

	// 删除最老的备份（如果超过限制）
	if rf.maxBackups > 0 {
		rf.pruneBackups()
	}

	// 依次重命名: .N-1 → .N (从最老到最新)
	for i := rf.maxBackups; i >= 1; i-- {
		src := rf.backupPath(i - 1)
		dst := rf.backupPath(i)
		if i == 1 {
			src = basePath
		}
		// 移除已存在的目标（压缩文件也考虑在内）
		os.Remove(dst)
		os.Remove(dst + ".gz")
		os.Rename(src, dst)
	}

	// 如果启用了压缩，异步压缩旧备份文件（不阻塞写入）
	if rf.compress {
		go rf.compressOldBackups()
	}

	// 打开新的当前文件
	f, err := os.OpenFile(basePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	rf.f = f
	rf.currentSize = 0
	return nil
}

// backupPath 返回第 N 级备份文件路径（不含 .gz 后缀）
func (rf *RotatingFile) backupPath(n int) string {
	return filepath.Join(rf.dir, fmt.Sprintf("%s.%d", rf.baseName, n))
}

// pruneBackups 删除超出 maxBackups 限制的最老备份
func (rf *RotatingFile) pruneBackups() {
	for i := rf.maxBackups + 1; ; i++ {
		p1 := rf.backupPath(i)
		p2 := p1 + ".gz"
		if _, err := os.Stat(p1); os.IsNotExist(err) {
			if _, err := os.Stat(p2); os.IsNotExist(err) {
				break
			}
			os.Remove(p2)
		} else {
			os.Remove(p1)
		}
	}
}

// compressOldBackups 对未压缩的备份文件进行 gzip 压缩
func (rf *RotatingFile) compressOldBackups() {
	for i := 2; i <= rf.maxBackups; i++ {
		srcPath := rf.backupPath(i)
		gzPath := srcPath + ".gz"
		// 已压缩则跳过
		if _, err := os.Stat(gzPath); err == nil {
			continue
		}
		// 源文件不存在则跳过
		srcFile, err := os.Open(srcPath)
		if err != nil {
			continue
		}
		gzFile, err := os.Create(gzPath)
		if err != nil {
			srcFile.Close()
			continue
		}
		gz := gzip.NewWriter(gzFile)
		if _, err := io.Copy(gz, srcFile); err != nil {
			gz.Close()
			gzFile.Close()
			srcFile.Close()
			os.Remove(gzPath)
			continue
		}
		gz.Close()
		gzFile.Close()
		srcFile.Close()
		os.Remove(srcPath)
	}
}

// cleanupLoop 定期清理过期日志文件
func (rf *RotatingFile) cleanupLoop() {
	for {
		select {
		case <-rf.stopCleanup:
			return
		case <-rf.cleanupTicker.C:
			rf.cleanup()
		}
	}
}

// cleanup 删除超过 maxAge 的日志文件
func (rf *RotatingFile) cleanup() {
	if rf.maxAge <= 0 {
		return
	}
	cutoff := time.Now().Add(-rf.maxAge)
	pattern := filepath.Join(rf.dir, rf.baseName+"*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}
	for _, fpath := range matches {
		// 跳过当前活跃日志文件
		if filepath.Base(fpath) == rf.baseName {
			continue
		}
		info, err := os.Stat(fpath)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(fpath)
		}
	}
}

// Close 关闭文件和后台协程
func (rf *RotatingFile) Close() {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.cleanupTicker != nil {
		rf.cleanupTicker.Stop()
		close(rf.stopCleanup)
	}
	if rf.f != nil {
		rf.f.Close()
		rf.f = nil
	}
}

// ── 日志文件信息（供 API 使用） ──

type LogFileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
}

// ListLogFiles 列出日志目录中所有日志文件（含轮转备份）
func ListLogFiles(logDir string) ([]LogFileInfo, error) {
	matches, err := filepath.Glob(filepath.Join(logDir, "*.log*"))
	if err != nil {
		return nil, err
	}
	// 也匹配 .gz
	gzMatches, err := filepath.Glob(filepath.Join(logDir, "*.log.gz"))
	if err == nil {
		seen := make(map[string]bool)
		for _, m := range matches {
			seen[m] = true
		}
		for _, m := range gzMatches {
			if !seen[m] {
				matches = append(matches, m)
			}
		}
	}

	var files []LogFileInfo
	for _, fpath := range matches {
		info, err := os.Stat(fpath)
		if err != nil {
			continue
		}
		files = append(files, LogFileInfo{
			Name:    filepath.Base(fpath),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	return files, nil
}

// LogDirSize 计算日志目录总大小
func LogDirSize(logDir string) (int64, error) {
	var total int64
	matches, err := filepath.Glob(filepath.Join(logDir, "*"))
	if err != nil {
		return 0, err
	}
	for _, fpath := range matches {
		info, err := os.Stat(fpath)
		if err != nil {
			continue
		}
		total += info.Size()
	}
	return total, nil
}

// CleanupLogs 手动清理过期日志文件，返回删除的文件数
func CleanupLogs(logDir string, maxAgeDays int, maxBackups int) (int, error) {
	if maxAgeDays <= 0 && maxBackups <= 0 {
		return 0, nil
	}

	var removed int

	// 按天数清理
	if maxAgeDays > 0 {
		cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
		patterns := []string{
			filepath.Join(logDir, "*.log.*"),
			filepath.Join(logDir, "*.log.*.gz"),
		}
		for _, pattern := range patterns {
			matches, _ := filepath.Glob(pattern)
			for _, fpath := range matches {
				info, err := os.Stat(fpath)
				if err != nil {
					continue
				}
				if info.ModTime().Before(cutoff) {
					os.Remove(fpath)
					removed++
				}
			}
		}
	}

	// 按备份数清理：提取每种基础日志的备份编号，删除超出的
	if maxBackups > 0 {
		matches, _ := filepath.Glob(filepath.Join(logDir, "*.log.*"))
		gzMatches, _ := filepath.Glob(filepath.Join(logDir, "*.log.*.gz"))
		all := append(matches, gzMatches...)

		// 按基础名分组
		groups := make(map[string][]string)
		for _, fpath := range all {
			base := filepath.Base(fpath)
			// 提取基础日志名: dhcp.log.1.gz → dhcp.log
			name := base
			name = strings.TrimSuffix(name, ".gz")
			name = strings.TrimSuffix(name, ".log.")
			name += ".log"
			groups[name] = append(groups[name], fpath)
		}

		for _, files := range groups {
			if len(files) <= maxBackups {
				continue
			}
			// 按文件名排序（编号越大越新）
			sort.Strings(files)
			toDelete := files[:len(files)-maxBackups]
			for _, fpath := range toDelete {
				os.Remove(fpath)
				removed++
			}
		}
	}

	return removed, nil
}
