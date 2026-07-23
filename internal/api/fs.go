package api

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

// FSHandler 提供服务器本地文件系统的只读浏览（供前端目录选择器使用）
type FSHandler struct{}

type fsBrowseResult struct {
	Path   string   `json:"path"`
	Parent string   `json:"parent"`
	Dirs   []string `json:"dirs"`
}

// Browse GET /fs/browse?path=
// path 为空：Windows 返回盘符列表，Linux 返回根目录
func (h *FSHandler) Browse(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")

	if p == "" {
		if runtime.GOOS == "windows" {
			var drives []string
			for c := 'C'; c <= 'Z'; c++ {
				d := string(c) + `:\`
				if _, err := os.Stat(d); err == nil {
					drives = append(drives, d)
				}
			}
			OK(w, fsBrowseResult{Path: "", Parent: "", Dirs: drives})
			return
		}
		p = "/"
	}

	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		Error(w, http.StatusBadRequest, "dir not found")
		return
	}

	var dirs []string
	entries, _ := os.ReadDir(p)
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)

	parent := filepath.Dir(p)
	if parent == p {
		parent = ""
	}
	OK(w, fsBrowseResult{Path: p, Parent: parent, Dirs: dirs})
}
