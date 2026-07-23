package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/osimage"
	"github.com/pxelab/pxelab/internal/store"
)

var rCtx = context.Background

type OSImageHandler struct {
	store    store.Interface
	config   *config.Config
	eventBus *eventbus.Bus
}

func NewOSImageHandler(st store.Interface, cfg *config.Config, bus *eventbus.Bus) *OSImageHandler {
	return &OSImageHandler{store: st, config: cfg, eventBus: bus}
}

func (h *OSImageHandler) List(w http.ResponseWriter, r *http.Request) {
	imgs, err := h.store.ListOSImages(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	for i := range imgs {
		img := &imgs[i]
		img.FilePath = h.isoPathOf(img)
		// 自愈：用户在系统外删除/移动文件后，DB 记录与磁盘状态对齐
		changed := false
		if _, err := os.Stat(h.isoPathOf(img)); err != nil {
			if img.Status != "missing" {
				img.Status = "missing"
				changed = true
			}
		} else if img.Status == "missing" {
			img.Status = "ready"
			changed = true
		}
		// 自愈：识别中途服务重启会残留 validating 状态且再无推进，超时后标记为错误以便重新识别
		if img.Status == "validating" && time.Since(img.UpdatedAt) > 10*time.Minute {
			img.Status = "error"
			img.ErrorMessage = "识别中断，请重新识别"
			changed = true
		}
		if img.ExtractedTo != "" {
			if _, err := os.Stat(img.ExtractedTo); err != nil {
				img.ExtractedTo = ""
				changed = true
			}
		}
		if img.MountPoint != "" {
			if _, err := os.Stat(img.MountPoint); err != nil {
				img.MountPoint = ""
				changed = true
			}
		}
		if changed {
			h.store.UpdateOSImage(r.Context(), img)
		}
	}
	OK(w, imgs)
}

func (h *OSImageHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid id")
		return
	}
	img, err := h.store.GetOSImage(r.Context(), uint(id))
	if err != nil {
		if err == store.ErrNotFound {
			Error(w, http.StatusNotFound, "image not found")
			return
		}
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	img.FilePath = h.isoPathOf(img)
	OK(w, img)
}

func (h *OSImageHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
	if img.MountPoint != "" {
		if err := osimage.UnmountISO(img.MountPoint, h.isoPathOf(img)); err != nil {
			Error(w, http.StatusInternalServerError, fmt.Sprintf("unmount failed: %v", err))
			return
		}
		// 只删除我们创建的空挂载目录（Windows 下 MountPoint 是盘符，绝不能 RemoveAll）
		os.RemoveAll(h.mountDir(img.ID))
	}
	if img.ExtractedTo != "" {
		os.RemoveAll(img.ExtractedTo)
	}
	// 导入的外部文件默认只删记录；显式传 ?delete_file=1 才删除源文件；托管文件随记录删除
	if img.SourcePath == "" || r.URL.Query().Get("delete_file") == "1" {
		os.Remove(h.isoPathOf(img))
	}

	if err := h.store.DeleteOSImage(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "os_image", strconv.FormatUint(id, 10), remoteIP(r), "删除系统镜像: "+img.Filename)
	w.WriteHeader(http.StatusNoContent)
}

func (h *OSImageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(4 << 30); err != nil {
		Error(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	defer r.MultipartForm.RemoveAll()

	file, header, err := r.FormFile("file")
	if err != nil {
		Error(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	filename := header.Filename
	if !strings.HasSuffix(strings.ToLower(filename), ".iso") {
		Error(w, http.StatusBadRequest, "only ISO files are supported")
		return
	}

	imgDir := h.imageDir()
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		Error(w, http.StatusInternalServerError, "cannot create images directory")
		return
	}

	dst := filepath.Join(imgDir, filename)
	dstFile, err := os.Create(dst)
	if err != nil {
		Error(w, http.StatusInternalServerError, "cannot create file")
		return
	}

	written, err := io.Copy(dstFile, file)
	dstFile.Close()
	if err != nil {
		os.Remove(dst)
		Error(w, http.StatusInternalServerError, "write failed")
		return
	}

	if err := osimage.ValidateISO(dst); err != nil {
		os.Remove(dst)
		Error(w, http.StatusBadRequest, fmt.Sprintf("invalid ISO: %v", err))
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = strings.TrimSuffix(filename, ".iso")
	}

	img := &models.OSImage{
		Name:     name,
		Filename: filename,
		Size:     written,
		Status:   "validating",
	}
	if err := h.store.CreateOSImage(r.Context(), img); err != nil {
		os.Remove(dst)
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	go h.processImage(img.ID, dst)
	RecordAudit(r.Context(), h.store, models.AuditCreate, "os_image", strconv.FormatUint(uint64(img.ID), 10), remoteIP(r), "上传系统镜像: "+filename)
	Created(w, img)
}

func (h *OSImageHandler) processImage(id uint, isoPath string) {
	ctx := rCtx()
	img, err := h.store.GetOSImage(ctx, id)
	if err != nil {
		return
	}

	// 临时挂载用于识别，完成后立即卸载——不常驻占用盘符/挂载点，需要时手动 Mount
	mountDir := h.mountDir(id)
	actualMount, err := osimage.MountISO(isoPath, mountDir)
	if err != nil {
		img.Status = "error"
		img.ErrorMessage = fmt.Sprintf("mount failed: %v", err)
		h.store.UpdateOSImage(ctx, img)
		h.publishEvent(models.EventError, fmt.Sprintf("系统镜像挂载失败: %s (%v)", img.Filename, err))
		return
	}
	defer func() {
		osimage.UnmountISO(actualMount, isoPath)
		os.RemoveAll(mountDir)
	}()

	meta, err := osimage.DetectDistro(isoPath, actualMount)
	if err != nil {
		img.ErrorMessage = fmt.Sprintf("detection failed: %v", err)
	}

	img.Distro = meta.Distro
	img.Version = meta.Version
	img.Arch = meta.Arch
	img.Checksum = meta.Checksum

	// 识别出发行版后按发行版定位 kernel/initrd；识别不出或定位失败时用通用兜底扫描
	if meta.Distro != "" {
		kernel, initrd := osimage.FindKernelInitrd(actualMount, meta.Distro)
		if rel, ok := existingRelPath(actualMount, kernel); ok {
			img.KernelPath = rel
		}
		if initrd != "" {
			if rel, ok := existingRelPath(actualMount, initrd); ok {
				img.InitrdPath = rel
			}
		}
	}
	if img.KernelPath == "" {
		if kernel, initrd := osimage.FindKernelInitrdFallback(actualMount); kernel != "" {
			if rel, ok := existingRelPath(actualMount, kernel); ok {
				img.KernelPath = rel
			}
			if rel, ok := existingRelPath(actualMount, initrd); ok {
				img.InitrdPath = rel
			}
		}
	}

	if meta.Distro != "" {
		img.Status = "ready"
		h.publishEvent(models.EventInfo, fmt.Sprintf("系统镜像就绪: %s (%s %s %s)", img.Filename, meta.Distro, meta.Version, meta.Arch))
	} else {
		img.Status = "ready"
		img.ErrorMessage = "distro not recognized, manual assignment needed"
		h.publishEvent(models.EventWarn, fmt.Sprintf("系统镜像已上传但未识别出发行版: %s，请手动指定", img.Filename))
	}

	h.store.UpdateOSImage(ctx, img)
}

// existingRelPath 返回 path 相对 base 的路径；文件不存在或无法计算相对路径时 ok=false
func existingRelPath(base, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	rel, err := filepath.Rel(base, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func (h *OSImageHandler) publishEvent(level models.EventLevel, message string) {
	if h.eventBus == nil {
		return
	}
	h.eventBus.Publish("event", models.Event{
		Type:    models.EventOSImage,
		Level:   level,
		Message: message,
	})
}

// ServeFile 以 HTTP 下载原始 ISO 文件（导入的外部镜像也可通过此接口网络可达）
func (h *OSImageHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
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
	path := h.isoPathOf(img)
	if _, err := os.Stat(path); err != nil {
		Error(w, http.StatusNotFound, "iso file missing")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", img.Filename))
	http.ServeFile(w, r, path)
}

// Reprocess 重新执行识别流程（发行版/kernel/initrd 探测逻辑更新后，旧记录可借此刷新）
func (h *OSImageHandler) Reprocess(w http.ResponseWriter, r *http.Request) {
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
	img.Status = "validating"
	img.ErrorMessage = ""
	h.store.UpdateOSImage(ctx, img)
	go h.processImage(img.ID, h.isoPathOf(img))
	OK(w, img)
}

func (h *OSImageHandler) Extract(w http.ResponseWriter, r *http.Request) {
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
	if img.Status != "ready" {
		Error(w, http.StatusBadRequest, "image is not ready")
		return
	}
	// 提取到 boot 服务树下的 isos 目录：HTTP /boot/isos/*、TFTP、NFS 默认导出立即可达，
	// kickstart/应答模板可直接用 http://<server>/boot/isos/<dir>/ 作为安装源
	extractDir := filepath.Join(h.bootIsosDir(), fmt.Sprintf("%d-%s", img.ID, sanitizeDirName(img.Name)))
	if err := osimage.ExtractISO(h.isoPathOf(img), extractDir); err != nil {
		Error(w, http.StatusInternalServerError, fmt.Sprintf("extract failed: %v", err))
		return
	}
	img.ExtractedTo = extractDir
	h.store.UpdateOSImage(ctx, img)
	OK(w, img)
}

func (h *OSImageHandler) Mount(w http.ResponseWriter, r *http.Request) {
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
	mountDir := h.mountDir(img.ID)
	actualMount, err := osimage.MountISO(h.isoPathOf(img), mountDir)
	if err != nil {
		Error(w, http.StatusInternalServerError, fmt.Sprintf("mount failed: %v", err))
		return
	}
	img.MountPoint = actualMount
	h.store.UpdateOSImage(ctx, img)
	OK(w, img)
}

func (h *OSImageHandler) Unmount(w http.ResponseWriter, r *http.Request) {
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
	if img.MountPoint == "" {
		Error(w, http.StatusBadRequest, "not mounted")
		return
	}
	if err := osimage.UnmountISO(img.MountPoint, h.isoPathOf(img)); err != nil {
		Error(w, http.StatusInternalServerError, fmt.Sprintf("unmount failed: %v", err))
		return
	}
	// 只删除我们创建的空挂载目录（Windows 下 MountPoint 是盘符，绝不能 RemoveAll）
	os.RemoveAll(h.mountDir(img.ID))
	img.MountPoint = ""
	h.store.UpdateOSImage(ctx, img)
	OK(w, img)
}

func (h *OSImageHandler) imageDir() string {
	dd := h.config.Global.DataDir
	if dd == "" {
		dd = ".pxelab"
	}
	return filepath.Join(dd, "isos")
}

// mountDir 是我们为镜像创建的空挂载目录（isos/mnt/<id>）
func (h *OSImageHandler) mountDir(id uint) string {
	return filepath.Join(h.imageDir(), "mnt", fmt.Sprintf("%d", id))
}

// bootIsosDir 是 boot 服务树下的 isos 目录（HTTP /boot/isos/*、TFTP、NFS 默认导出均指向这里）
func (h *OSImageHandler) bootIsosDir() string {
	root := h.config.Boot.RootDir
	if root == "" {
		root = filepath.Join(h.config.Global.DataDir, "boot")
	}
	return filepath.Join(root, "isos")
}

// sanitizeDirName 把镜像名清洗为安全的目录名（仅保留字母数字和 -_.）
func sanitizeDirName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteRune('-')
		}
	}
	s := b.String()
	if s == "" {
		s = "image"
	}
	return s
}

// isoPathOf 返回镜像 ISO 文件的实际路径：导入的外部文件用 SourcePath，否则用托管目录拼接
func (h *OSImageHandler) isoPathOf(img *models.OSImage) string {
	if img.SourcePath != "" {
		return img.SourcePath
	}
	return h.imagePath(img.Filename)
}

type importRequest struct {
	Dir       string `json:"dir"`
	Recursive bool   `json:"recursive"`
}

// Import 扫描服务器本地目录中的 *.iso 并登记为镜像（不复制文件，SourcePath 指向原位置）
func (h *OSImageHandler) Import(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.Dir == "" {
		Error(w, http.StatusBadRequest, "dir required")
		return
	}
	abs, err := filepath.Abs(req.Dir)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid dir")
		return
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		Error(w, http.StatusBadRequest, "dir not found")
		return
	}

	// 收集目录中的 ISO 文件
	var found []string
	if req.Recursive {
		filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".iso") {
				found = append(found, path)
			}
			return nil
		})
	} else {
		entries, _ := os.ReadDir(abs)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".iso") {
				found = append(found, filepath.Join(abs, e.Name()))
			}
		}
	}

	// 跳过已登记（同 SourcePath 或托管目录同名文件）
	existing, _ := h.store.ListOSImages(r.Context())
	registered := map[string]bool{}
	for _, img := range existing {
		if img.SourcePath != "" {
			registered[img.SourcePath] = true
		} else {
			registered[img.Filename] = true
		}
	}

	type pending struct {
		id   uint
		path string
	}
	var imported []pending
	skipped := 0
	for _, src := range found {
		base := filepath.Base(src)
		if registered[src] || registered[base] {
			skipped++
			continue
		}
		fi, err := os.Stat(src)
		if err != nil {
			continue
		}
		img := &models.OSImage{
			Name:       strings.TrimSuffix(base, filepath.Ext(base)),
			Filename:   base,
			SourcePath: src,
			Size:       fi.Size(),
			Status:     "validating",
		}
		if err := h.store.CreateOSImage(r.Context(), img); err != nil {
			continue
		}
		imported = append(imported, pending{id: img.ID, path: src})
	}

	// 串行处理，避免同时挂载多个 ISO
	go func() {
		for _, p := range imported {
			h.processImage(p.id, p.path)
		}
	}()

	OK(w, map[string]int{"imported": len(imported), "skipped": skipped})
}

type osImageUpdateRequest struct {
	Name       *string `json:"name"`
	Distro     *string `json:"distro"`
	Version    *string `json:"version"`
	Arch       *string `json:"arch"`
	KernelPath *string `json:"kernel_path"`
	InitrdPath *string `json:"initrd_path"`
}

// Update 手动编辑镜像元数据（发行版识别失败时的手动指定入口）
func (h *OSImageHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	var req osImageUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.Name != nil && *req.Name != "" {
		img.Name = *req.Name
	}
	if req.Distro != nil {
		img.Distro = *req.Distro
		if *req.Distro != "" && strings.Contains(img.ErrorMessage, "not recognized") {
			img.ErrorMessage = ""
		}
	}
	if req.Version != nil {
		img.Version = *req.Version
	}
	if req.Arch != nil {
		img.Arch = *req.Arch
	}
	if req.KernelPath != nil {
		img.KernelPath = *req.KernelPath
	}
	if req.InitrdPath != nil {
		img.InitrdPath = *req.InitrdPath
	}
	if err := h.store.UpdateOSImage(ctx, img); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	img.FilePath = h.isoPathOf(img)
	OK(w, img)
}

func (h *OSImageHandler) imagePath(filename string) string {
	return filepath.Join(h.imageDir(), filename)
}
