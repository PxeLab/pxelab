package api

import (
	"io"
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

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		Error(w, http.StatusBadRequest, "无效的 multipart 表单")
		return
	}
	defer r.MultipartForm.RemoveAll()

	rootDir := h.bootFS.Root()
	var uploaded []map[string]any

	for _, headers := range r.MultipartForm.File {
		for _, header := range headers {
			file, err := header.Open()
			if err != nil {
				Error(w, http.StatusInternalServerError, "文件打开失败")
				return
			}

			clean := filepath.Clean(header.Filename)
			if strings.Contains(clean, "..") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "\\") {
				file.Close()
				Error(w, http.StatusBadRequest, "无效的文件名")
				return
			}

			dst := filepath.Join(rootDir, clean)
			dstFile, err := os.Create(dst)
			if err != nil {
				file.Close()
				Error(w, http.StatusInternalServerError, "文件创建失败")
				return
			}

			written, err := io.Copy(dstFile, file)
			file.Close()
			dstFile.Close()
			if err != nil {
				Error(w, http.StatusInternalServerError, "文件写入失败")
				return
			}

			uploaded = append(uploaded, map[string]any{
				"name": header.Filename,
				"size": written,
			})
		}
	}

	Created(w, uploaded)
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
