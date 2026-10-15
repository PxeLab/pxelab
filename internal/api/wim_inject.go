package api

// WIM 离线驱动注入（借鉴项 1/4/5，Phase 1 → docs/design/windows-install-roadmap.md §2.2）：
//
//	POST /api/v1/os-images/{id}/inject-drivers  离线注入驱动包（dism Add-WindowsDriver）
//	GET  /api/v1/os-images/{id}/wim-info       列出 WIM 内可选镜像索引（Get-WindowsImage）
//
// 流程：原 WIM 先备份为 <name>.orig（已存在则保留，绝不覆盖）→ 挂载 →
// Add-WindowsDriver → 卸载前用 Get-WindowsDriver 验证注入结果（借鉴项 5）→
// 提交卸载 → dism /Cleanup-Wim 清理。仅 Windows 主机
// 支持（非 Windows 返回 501，提示改用 PE 阶段运行时驱动加载）。

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/osimage"
)

// wimInjectLocks 保证同一镜像同一时刻只有一次离线注入/还原在运行：并发注入会
// 同时 mount/commit 同一 WIM，可能损坏镜像或留下幽灵挂载点（dism /Cleanup-Wim
// 未必能救）。按镜像 ID 分锁，不同镜像互不阻塞。
var wimInjectLocks sync.Map // uint -> *sync.Mutex

// lockWIMInject 尝试获取镜像的注入锁；已被占用时返回 nil（调用方应回 409）。
func lockWIMInject(id uint) *sync.Mutex {
	mu, _ := wimInjectLocks.LoadOrStore(id, &sync.Mutex{})
	m := mu.(*sync.Mutex)
	if !m.TryLock() {
		return nil
	}
	return m
}

type wimInjectRequest struct {
	DriverPacks []string `json:"driver_packs"` // 驱动包名列表（boot 根目录 drivers/<名称>/）
	Target      string   `json:"target"`       // "boot"（默认，注入 PE 引导镜像）| "install"（注入安装镜像）
	Index       int      `json:"index"`        // WIM 镜像索引；默认 1（boot.wim 必须 index 1）
	Recurse     *bool    `json:"recurse"`      // 递归包含子目录；默认 true
}

// stagedWIMPack 一个已校验待注入的驱动包（name + 按架构过滤后的文件清单）。
type stagedWIMPack struct {
	name  string
	files []string
}

// Capabilities 报告本平台是否支持离线 WIM 驱动注入（前端据此置灰入口，
// 避免非 Windows 上点击后才拿到 501）。无平台限制，任何镜像状态均可调用。
func (h *OSImageHandler) Capabilities(w http.ResponseWriter, r *http.Request) {
	OK(w, osimage.WIMCapabilities())
}

// WIMInfo 列出镜像 WIM 内全部可选镜像索引（注入 index 参数的选项来源，借鉴项 4）。
// target 查询参数：boot（默认）| install。
func (h *OSImageHandler) WIMInfo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	img, err := h.store.GetOSImage(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "image not found")
		return
	}
	if img.ExtractedTo == "" {
		Error(w, http.StatusBadRequest, "image is not extracted; extract it first")
		return
	}
	target := normalizeWimTarget(r.URL.Query().Get("target"))
	wimPath := h.wimPathOf(img, target)
	if _, err := os.Stat(wimPath); err != nil {
		Error(w, http.StatusBadRequest, fmt.Sprintf("wim not found: %s", wimRelPath(target)))
		return
	}
	indexes, err := osimage.GetWIMInfo(wimPath)
	if err != nil {
		respondWIMError(w, err)
		return
	}
	OK(w, map[string]any{
		"target":  target,
		"wim":     wimPath,
		"indexes": indexes,
	})
}

// InjectDrivers 把驱动包离线注入镜像的 WIM（借鉴项 1；备份/验证/清理为借鉴项 5）。
// 驱动文件按镜像架构过滤（借鉴项 3）后暂存到临时目录再交给 dism。
func (h *OSImageHandler) InjectDrivers(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	img, err := h.store.GetOSImage(ctx, uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "image not found")
		return
	}
	var req wimInjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if len(req.DriverPacks) == 0 {
		Error(w, http.StatusBadRequest, "driver_packs required")
		return
	}
	// target 严格校验：未知值直接 400（静默回落 boot 会把驱动误注入 boot.wim）
	switch strings.ToLower(strings.TrimSpace(req.Target)) {
	case "", "boot", "install":
	default:
		Error(w, http.StatusBadRequest, `target 必须是 "boot" 或 "install"`)
		return
	}
	target := normalizeWimTarget(req.Target)
	if img.Status != "ready" {
		Error(w, http.StatusBadRequest, "image is not ready")
		return
	}
	if img.ExtractedTo == "" {
		Error(w, http.StatusBadRequest, "image is not extracted; extract it first")
		return
	}
	wimPath := h.wimPathOf(img, target)
	if _, err := os.Stat(wimPath); err != nil {
		Error(w, http.StatusBadRequest, fmt.Sprintf("wim not found: %s", wimRelPath(target)))
		return
	}
	index := req.Index
	if index <= 0 {
		index = 1
	}
	recurse := req.Recurse == nil || *req.Recurse

	// 校验 + 收集驱动包文件（先全部校验，避免半途失败留下脏临时目录）
	staged := make([]stagedWIMPack, 0, len(req.DriverPacks))
	for _, name := range req.DriverPacks {
		files, err := h.injectablePackFiles(name, img.Arch)
		if err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		staged = append(staged, stagedWIMPack{name: name, files: files})
	}

	// 并发互斥：同一镜像的注入/还原串行化，避免同时 mount/commit 同一 WIM。
	mu := lockWIMInject(img.ID)
	if mu == nil {
		Error(w, http.StatusConflict, "该镜像正有离线注入/还原在进行，请稍后重试")
		return
	}
	defer mu.Unlock()

	// 备份原 WIM（已存在则保留 —— 原始介质只备份一次）
	backupPath := wimPath + ".orig"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := copyFileTo(wimPath, backupPath); err != nil {
			Error(w, http.StatusInternalServerError, fmt.Sprintf("backup wim failed: %v", err))
			return
		}
	} else if err != nil {
		Error(w, http.StatusInternalServerError, fmt.Sprintf("check backup: %v", err))
		return
	}

	// 暂存驱动文件到本地目录（dism 需要真实目录，不能从 boot 树直接读）
	stageDir, err := os.MkdirTemp("", "pxelab-wimdrivers-*")
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(stageDir)
	for _, sp := range staged {
		for _, f := range sp.files {
			src := filepath.Join(h.bootFS.Root(), "drivers", sp.name, filepath.FromSlash(f))
			dst := filepath.Join(stageDir, sp.name, filepath.FromSlash(f))
			if err := copyFileTo(src, dst); err != nil {
				Error(w, http.StatusInternalServerError, fmt.Sprintf("stage driver %s failed: %v", sp.name, err))
				return
			}
		}
	}

	mountDir, err := os.MkdirTemp("", "pxelab-wimmount-*")
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer os.RemoveAll(mountDir)

	if err := osimage.MountWIM(wimPath, mountDir, index); err != nil {
		h.wimCleanup(mountDir)
		respondWIMError(w, err)
		return
	}
	if err := osimage.AddDriverToWIM(mountDir, stageDir, recurse); err != nil {
		// 注入失败：丢弃改动卸载，保证无残留挂载
		h.wimDiscard(mountDir)
		respondWIMError(w, err)
		return
	}
	// 借鉴项 5：卸载前验证注入结果（Get-WindowsDriver）；失败不阻断提交
	var drivers []osimage.WIMDriver
	if list, lerr := osimage.ListWIMDrivers(mountDir); lerr != nil {
		slog.Warn("WIM 注入验证失败（Get-WindowsDriver）", "error", lerr)
	} else {
		drivers = list
	}
	if err := osimage.UnmountWIM(mountDir, true); err != nil {
		// 提交失败：尝试丢弃卸载避免幽灵挂载点，再报错
		h.wimDiscard(mountDir)
		Error(w, http.StatusInternalServerError, fmt.Sprintf("commit wim failed: %v", err))
		return
	}
	os.RemoveAll(mountDir)
	if err := osimage.CleanupWIM(); err != nil {
		slog.Warn("WIM 挂载点数据库清理失败（dism /Cleanup-Wim）", "error", err)
	}

	now := time.Now().UTC()
	img.InjectedDrivers = mustJSONStringArray(namesOf(staged))
	img.DriverInjectedAt = &now
	img.DriverInjectCount++
	if err := h.store.UpdateOSImage(ctx, img); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(ctx, h.store, models.AuditUpdate, "os_image", strconv.FormatUint(uint64(img.ID), 10), remoteIP(r),
		fmt.Sprintf("WIM 离线注入驱动: %s → %s (index %d, 第 %d 次)", strings.Join(namesOf(staged), ","), wimRelPath(target), index, img.DriverInjectCount))
	OK(w, map[string]any{
		"injected":     namesOf(staged),
		"target":       target,
		"index":        index,
		"wim":          wimPath,
		"backup":       backupPath,
		"drivers":      drivers,
		"inject_count": img.DriverInjectCount,
	})
}

// restoreRequest 还原请求体（target 缺省 boot）。
type restoreRequest struct {
	Target string `json:"target"`
}

// RestoreWIM 用 `<name>.orig` 覆盖回原始 WIM，回滚离线注入并清空注入记录
// （借用项 5 的“可回滚”）。需先前有过注入（即存在 .orig 备份）。
func (h *OSImageHandler) RestoreWIM(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	ctx := r.Context()
	img, err := h.store.GetOSImage(ctx, uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "image not found")
		return
	}
	var req restoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	switch strings.ToLower(strings.TrimSpace(req.Target)) {
	case "", "boot", "install":
	default:
		Error(w, http.StatusBadRequest, `target 必须是 "boot" 或 "install"`)
		return
	}
	target := normalizeWimTarget(req.Target)
	if img.ExtractedTo == "" {
		Error(w, http.StatusBadRequest, "image is not extracted; extract it first")
		return
	}
	wimPath := h.wimPathOf(img, target)
	backupPath := wimPath + ".orig"
	if _, err := os.Stat(backupPath); err != nil {
		Error(w, http.StatusBadRequest, fmt.Sprintf("未找到备份 %s，无法还原（仅注入过才可还原）", wimRelPath(target)+".orig"))
		return
	}

	mu := lockWIMInject(img.ID)
	if mu == nil {
		Error(w, http.StatusConflict, "该镜像正有离线注入/还原在进行，请稍后重试")
		return
	}
	defer mu.Unlock()

	if err := copyFileTo(backupPath, wimPath); err != nil {
		Error(w, http.StatusInternalServerError, fmt.Sprintf("还原失败: %v", err))
		return
	}
	img.InjectedDrivers = ""
	img.DriverInjectedAt = nil
	img.DriverInjectCount = 0
	if err := h.store.UpdateOSImage(ctx, img); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(ctx, h.store, models.AuditUpdate, "os_image", strconv.FormatUint(uint64(img.ID), 10), remoteIP(r),
		fmt.Sprintf("WIM 还原为原始文件: %s ← %s", wimRelPath(target), wimRelPath(target)+".orig"))
	OK(w, map[string]any{
		"restored": wimPath,
		"from":     backupPath,
	})
}

// injectablePackFiles 返回驱动包按镜像架构过滤后的文件清单（借鉴项 3 的
// 架构过滤用于离线注入侧）。bootFS 不可用、包名非法/不存在/为空、或无匹配
// 架构文件时返回错误。
func (h *OSImageHandler) injectablePackFiles(name, arch string) ([]string, error) {
	if h.bootFS == nil {
		return nil, fmt.Errorf("boot 文件服务不可用")
	}
	files, err := listDriverPackFiles(h.bootFS.Root(), name)
	if err != nil {
		return nil, err
	}
	if arch != "" {
		files = filterDriverFilesByArch(files, arch)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("驱动包无匹配架构 %q 的文件: %s", arch, name)
	}
	return files, nil
}

func (h *OSImageHandler) wimPathOf(img *models.OSImage, target string) string {
	return filepath.Join(img.ExtractedTo, "sources", target+".wim")
}

// wimDiscard 丢弃式卸载已挂载的 WIM（出错路径的兜底），失败仅告警。
func (h *OSImageHandler) wimDiscard(mountDir string) {
	if err := osimage.UnmountWIM(mountDir, false); err != nil {
		slog.Warn("WIM 丢弃卸载失败", "mount_dir", mountDir, "error", err)
	}
	os.RemoveAll(mountDir)
}

// wimCleanup 挂载失败后的清理（未挂载成功，直接清目录 + 清挂载点数据库）。
func (h *OSImageHandler) wimCleanup(mountDir string) {
	os.RemoveAll(mountDir)
	if err := osimage.CleanupWIM(); err != nil {
		slog.Warn("WIM 挂载点数据库清理失败（dism /Cleanup-Wim）", "error", err)
	}
}

// normalizeWimTarget 归一化 target 参数：空/"boot" → boot；"install" → install。
func normalizeWimTarget(target string) string {
	t := strings.ToLower(strings.TrimSpace(target))
	if t == "install" {
		return "install"
	}
	return "boot"
}

func wimRelPath(target string) string {
	return filepath.ToSlash(filepath.Join("sources", target+".wim"))
}

// respondWIMError 按 WIM 错误类型分流：平台不支持 → 501 并提示替代方案；
// 其余 → 500。
func respondWIMError(w http.ResponseWriter, err error) {
	if errors.Is(err, osimage.ErrWIMUnsupported) {
		Error(w, http.StatusNotImplemented, "离线 WIM 注入需要 Windows 主机（dism）；非 Windows 平台请改用 PE 阶段运行时驱动加载（winpeshl install.bat 注入）")
		return
	}
	Error(w, http.StatusInternalServerError, err.Error())
}

func namesOf(staged []stagedWIMPack) []string {
	out := make([]string, 0, len(staged))
	for _, sp := range staged {
		out = append(out, sp.name)
	}
	return out
}

// copyFileTo 流式复制文件（保留目标父目录自动创建）。
func copyFileTo(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func mustJSONStringArray(items []string) string {
	b, err := json.Marshal(items)
	if err != nil {
		// []string 永不失败；防御性兜底为空数组
		return "[]"
	}
	return string(b)
}
