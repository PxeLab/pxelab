package boot

import (
	"fmt"
	"os"
	"path/filepath"
)

type BootFileServer struct {
	rootDir string
}

func NewBootFileServer(rootDir string) *BootFileServer {
	return &BootFileServer{rootDir: rootDir}
}

func (b *BootFileServer) Root() string {
	return b.rootDir
}

func (b *BootFileServer) Read(path string) ([]byte, error) {
	fullPath := filepath.Join(b.rootDir, path)
	// 安全检查：防止目录遍历
	absRoot, _ := filepath.Abs(b.rootDir)
	absFile, _ := filepath.Abs(fullPath)
	if len(absFile) < len(absRoot) || absFile[:len(absRoot)] != absRoot {
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
		return nil, err
	}
	infos := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		infos = append(infos, info)
	}
	return infos, nil
}
