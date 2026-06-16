package boot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type BootFileServer struct {
	rootDir string
}

func NewBootFileServer(rootDir string) *BootFileServer {
	// 自动创建根目录
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		// 仅记录，不阻塞启动
	}
	return &BootFileServer{rootDir: rootDir}
}

func (b *BootFileServer) Root() string {
	return b.rootDir
}

func (b *BootFileServer) Read(path string) ([]byte, error) {
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("路径越权: %s", path)
	}
	fullPath := filepath.Join(b.rootDir, cleanPath)

	absRoot, err := filepath.Abs(b.rootDir)
	if err != nil {
		return nil, fmt.Errorf("获取根路径失败: %w", err)
	}
	absFile, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("获取文件路径失败: %w", err)
	}

	// 安全检查：确认文件路径在 rootDir 之下
	rootPrefix := absRoot + string(filepath.Separator)
	if !strings.HasPrefix(absFile, rootPrefix) && absFile != absRoot {
		return nil, fmt.Errorf("路径越权: %s", path)
	}

	return os.ReadFile(fullPath)
}

func (b *BootFileServer) Exists(path string) bool {
	fullPath := filepath.Join(b.rootDir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (b *BootFileServer) List(dir string) ([]os.FileInfo, error) {
	fullPath := filepath.Join(b.rootDir, dir)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []os.FileInfo{}, nil
		}
		return nil, err
	}
	infos := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		infos = append(infos, info)
	}
	return infos, nil
}
