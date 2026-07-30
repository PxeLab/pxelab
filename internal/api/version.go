package api

import (
	"log/slog"
	"net/http"

	"github.com/pxelab/pxelab/internal/updatecheck"
)

// VersionHandler 版本检查与更新处理
type VersionHandler struct {
	version       string
	updateChecker *updatecheck.Checker
}

// NewVersionHandler 创建版本处理器
func NewVersionHandler(version string, checker *updatecheck.Checker) *VersionHandler {
	return &VersionHandler{
		version:       version,
		updateChecker: checker,
	}
}

// GetVersion 返回当前版本信息和最近一次检查结果
// GET /api/v1/version
func (h *VersionHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	result := map[string]any{
		"current_version": h.version,
	}

	if h.updateChecker != nil {
		if cached := h.updateChecker.Result(); cached != nil {
			result["check"] = cached
		}
	}

	OK(w, result)
}

// CheckUpdate 手动触发版本检查
// POST /api/v1/version/check
func (h *VersionHandler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	if h.updateChecker == nil {
		Error(w, http.StatusServiceUnavailable, "版本检查器未初始化")
		return
	}

	result, err := h.updateChecker.Check()
	if err != nil {
		slog.Warn("手动版本检查失败", "error", err)
		// 即使网络失败也返回结果（内含错误信息）
		OK(w, map[string]any{
			"current_version": h.version,
			"check":           result,
		})
		return
	}

	OK(w, map[string]any{
		"current_version": h.version,
		"check":           result,
	})
}

// DownloadUpdate 下载最新版本更新包
// POST /api/v1/version/download
func (h *VersionHandler) DownloadUpdate(w http.ResponseWriter, r *http.Request) {
	if h.updateChecker == nil {
		Error(w, http.StatusServiceUnavailable, "版本检查器未初始化")
		return
	}

	result, err := h.updateChecker.Download()
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	OK(w, result)
}
