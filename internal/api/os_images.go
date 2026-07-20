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

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/osimage"
	"github.com/pxelab/pxelab/internal/store"
)

var rCtx = context.Background

type OSImageHandler struct {
	store  store.Interface
	config *config.Config
}

func NewOSImageHandler(st store.Interface, cfg *config.Config) *OSImageHandler {
	return &OSImageHandler{store: st, config: cfg}
}

func (h *OSImageHandler) List(w http.ResponseWriter, r *http.Request) {
	imgs, err := h.store.ListOSImages(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
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
		if err := osimage.UnmountISO(img.MountPoint, h.imagePath(img.Filename)); err != nil {
			Error(w, http.StatusInternalServerError, fmt.Sprintf("unmount failed: %v", err))
			return
		}
		os.RemoveAll(img.MountPoint)
	}
	if img.ExtractedTo != "" {
		os.RemoveAll(img.ExtractedTo)
	}
	fullPath := h.imagePath(img.Filename)
	os.Remove(fullPath)

	if err := h.store.DeleteOSImage(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "os_image", strconv.FormatUint(id, 10), remoteIP(r), "")
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
	RecordAudit(r.Context(), h.store, models.AuditCreate, "os_image", strconv.FormatUint(uint64(img.ID), 10), remoteIP(r), filename)
	Created(w, img)
}

func (h *OSImageHandler) processImage(id uint, isoPath string) {
	ctx := rCtx()
	img, err := h.store.GetOSImage(ctx, id)
	if err != nil {
		return
	}

	mountPoint := filepath.Join(h.imageDir(), "mnt", fmt.Sprintf("%d", id))
	actualMount, err := osimage.MountISO(isoPath, mountPoint)
	if err != nil {
		img.Status = "error"
		img.ErrorMessage = fmt.Sprintf("mount failed: %v", err)
		h.store.UpdateOSImage(ctx, img)
		return
	}

	meta, err := osimage.DetectDistro(isoPath, actualMount)
	if err != nil {
		img.ErrorMessage = fmt.Sprintf("detection failed: %v", err)
	}

	img.Distro = meta.Distro
	img.Version = meta.Version
	img.Arch = meta.Arch
	img.Checksum = meta.Checksum
	img.MountPoint = mountPoint

	if meta.Distro != "" {
		img.Status = "ready"
	} else {
		img.Status = "ready"
		img.ErrorMessage = "distro not recognized, manual assignment needed"
	}

	h.store.UpdateOSImage(ctx, img)
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
	extractDir := filepath.Join(h.imageDir(), "extracted", fmt.Sprintf("%d", id))
	if err := osimage.ExtractISO(h.imagePath(img.Filename), extractDir); err != nil {
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
	mountPoint := filepath.Join(h.imageDir(), "mnt", fmt.Sprintf("%d", id))
	_, err = osimage.MountISO(h.imagePath(img.Filename), mountPoint)
	if err != nil {
		Error(w, http.StatusInternalServerError, fmt.Sprintf("mount failed: %v", err))
		return
	}
	img.MountPoint = mountPoint
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
	if err := osimage.UnmountISO(img.MountPoint, h.imagePath(img.Filename)); err != nil {
		Error(w, http.StatusInternalServerError, fmt.Sprintf("unmount failed: %v", err))
		return
	}
	os.RemoveAll(img.MountPoint)
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

func (h *OSImageHandler) imagePath(filename string) string {
	return filepath.Join(h.imageDir(), filename)
}
