package boot

import (
	"fmt"
	"net/http"
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

// resolvePath 校验并拼接完整路径，返回绝对路径
func (b *BootFileServer) resolvePath(path string) (string, error) {
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("路径越权: %s", path)
	}
	fullPath := filepath.Join(b.rootDir, cleanPath)

	absRoot, err := filepath.Abs(b.rootDir)
	if err != nil {
		return "", fmt.Errorf("获取根路径失败: %w", err)
	}
	absFile, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("获取文件路径失败: %w", err)
	}

	rootPrefix := absRoot + string(filepath.Separator)
	if !strings.HasPrefix(absFile, rootPrefix) && absFile != absRoot {
		return "", fmt.Errorf("路径越权: %s", path)
	}
	return absFile, nil
}

// ResolvePath exposes resolvePath for use from other packages.
func (b *BootFileServer) ResolvePath(path string) (string, error) {
	return b.resolvePath(path)
}

func (b *BootFileServer) Read(path string) ([]byte, error) {
	absFile, err := b.resolvePath(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(absFile)
}

// Open 返回 *os.File 用于 http.ServeContent 的 Range/HEAD/Seek 支持
func (b *BootFileServer) Open(path string) (*os.File, error) {
	absFile, err := b.resolvePath(path)
	if err != nil {
		return nil, err
	}
	return os.Open(absFile)
}

// Serve 将文件写入 http.ResponseWriter，支持 Range/HEAD
func (b *BootFileServer) Serve(w http.ResponseWriter, r *http.Request, path string) {
	f, err := b.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, stat.Name(), stat.ModTime(), f)
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
