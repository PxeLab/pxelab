package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pxego/pxego/internal/boot"
)

type FileHandler struct {
	bootFS *boot.BootFileServer
}

func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "."
	}
	// 安全检查：防止目录遍历
	clean := filepath.Clean(dir)
	if strings.Contains(clean, "..") {
		Error(w, http.StatusBadRequest, "无效的目录路径")
		return
	}
	infos, err := h.bootFS.List(clean)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	files := make([]map[string]any, 0, len(infos))
	for _, info := range infos {
		files = append(files, map[string]any{
			"name":    info.Name(),
			"size":    info.Size(),
			"is_dir":  info.IsDir(),
			"modtime": info.ModTime(),
		})
	}
	OK(w, files)
}

func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		Error(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") {
		Error(w, http.StatusBadRequest, "无效的文件路径")
		return
	}
	fullPath := filepath.Join(h.bootFS.Root(), clean)
	absRoot, _ := filepath.Abs(h.bootFS.Root())
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absRoot) {
		Error(w, http.StatusForbidden, "路径越权")
		return
	}
	if err := os.Remove(fullPath); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
