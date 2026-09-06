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
			// 带内容哈希的构建产物可长缓存；其余静态文件每次协商，避免更新后浏览器沿用旧副本
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback：所有无扩展名的路径返回 index.html（不缓存，保证更新后立即可见）
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(indexStr))
	})
}

//go:generate powershell -Command "Remove-Item -Recurse -Force './bootdist/*' -Exclude 'README.md' -ErrorAction SilentlyContinue; Copy-Item -Recurse -Force -Path '../../boot/*' -Destination './bootdist/' -Exclude 'README.md'"

//go:embed all:bootdist
var embeddedBootFS embed.FS

// bootdistVersion 内嵌启动文件集版本号。
// 每次 bootdist/ 内容（NBP、脚本、配置模板）发生变更时递增该值，
// 使已部署机器下次启动时自动覆盖释放更新后的文件。
// 注：bootdistVersion 从 "2" 起步，因为老版本从未写入版本标记，
// 缺失标记即视为需要首次升级释放。
const bootdistVersion = "2"

// bootdistMarker 存放在 rootDir 下、记录已释放的 bootdist 版本。
const bootdistMarker = ".bootdist-version"

// extractBootFiles 将嵌入的启动文件释放到 rootDir。
//
// autoUpdate=true（默认）：
// rootDir 下的 .bootdist-version 标记缺失或与当前 bootdistVersion
// 不一致时，覆盖释放内嵌清单中的所有文件（用户额外上传的自定义文件不受影响），
// 并写入新标记；标记一致时直接跳过，避免重复 IO。
//
// autoUpdate=false：
// 仅补发缺失的内嵌文件（首次运行 / 用户删除了默认文件），
// 绝不覆盖已存在的文件——用户自定义或修改过的文件保持原样，
// 也不更新版本标记。
func extractBootFiles(rootDir string, autoUpdate bool) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		slog.Warn("创建启动目录失败", "error", err)
		return
	}

	srcFS, err := fs.Sub(embeddedBootFS, "bootdist")
	if err != nil {
		slog.Warn("读取嵌入启动文件失败", "error", err)
		return
	}

	markerPath := filepath.Join(rootDir, bootdistMarker)
	if autoUpdate {
		if data, err := os.ReadFile(markerPath); err == nil && strings.TrimSpace(string(data)) == bootdistVersion {
			return // 版本一致，跳过
		}
	}

	// 覆盖释放内嵌文件（autoUpdate）或仅补发缺失文件（autoUpdate=false），
	// 保证升级后与当前二进制一致
	if err := fs.WalkDir(srcFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// 在 rootDir 下创建对应子目录
			subDir := filepath.Join(rootDir, path)
			if err := os.MkdirAll(subDir, 0755); err != nil {
				slog.Warn("创建子目录失败", "dir", subDir, "error", err)
			}
			return nil
		}
		data, err := fs.ReadFile(srcFS, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(rootDir, path)
		if !autoUpdate {
			// 手动模式：已存在文件一律不覆盖，保留用户自定义内容
			if _, err := os.Stat(dst); err == nil {
				return nil
			}
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			slog.Warn("释放启动文件失败", "path", path, "error", err)
		}
		return nil
	}); err != nil {
		slog.Warn("释放启动文件失败", "error", err)
		return
	}

	if !autoUpdate {
		// 手动模式不写入版本标记，下次启动仍只补缺失文件
		slog.Info("已补发缺失的内嵌启动文件", "dir", rootDir)
		return
	}

	// 全部释放成功后才写入版本标记
	if err := os.WriteFile(markerPath, []byte(bootdistVersion), 0644); err != nil {
		slog.Warn("写入 bootdist 版本标记失败", "path", markerPath, "error", err)
		return
	}
	slog.Info("已释放内嵌启动文件", "version", bootdistVersion, "dir", rootDir)
}
