package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

//go:embed webdist/*
var spaFS embed.FS

func spaHandler() http.Handler {
	subFS, _ := fs.Sub(spaFS, "webdist")
	return http.FileServer(http.FS(subFS))
}

//go:embed bootdist/*
var embeddedBootFS embed.FS

// extractBootFiles 将嵌入的 iPXE 启动文件释放到 rootDir（仅首次）
func extractBootFiles(rootDir string) {
	srcFS, err := fs.Sub(embeddedBootFS, "bootdist")
	if err != nil {
		slog.Warn("读取嵌入启动文件失败", "error", err)
		return
	}
	entries, err := fs.ReadDir(srcFS, ".")
	if err != nil {
		return
	}
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		slog.Warn("创建启动目录失败", "error", err)
		return
	}
	for _, e := range entries {
		dst := filepath.Join(rootDir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		data, err := fs.ReadFile(srcFS, e.Name())
		if err != nil {
			continue
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			slog.Warn("释放启动文件失败", "name", e.Name(), "error", err)
		}
	}
}
