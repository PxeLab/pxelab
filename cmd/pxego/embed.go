package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed webdist/*
var spaFS embed.FS

func spaHandler() http.Handler {
	subFS, _ := fs.Sub(spaFS, "webdist")
	fileServer := http.FileServer(http.FS(subFS))

	// 读取 index.html 供 SPA fallback 使用
	indexBytes, err := fs.ReadFile(subFS, "index.html")
	if err != nil {
		panic("嵌入的 SPA 缺少 index.html: " + err.Error())
	}
	indexStr := string(indexBytes)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 如果是 API 路径则不处理（已由上层路由处理）
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/boot/") || r.URL.Path == "/health" {
			http.NotFound(w, r)
			return
		}

		// 检查请求的是否是真实文件（有扩展名且文件存在）
		ext := filepath.Ext(r.URL.Path)
		if ext != "" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback：所有无扩展名的路径返回 index.html
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(indexStr))
	})
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
