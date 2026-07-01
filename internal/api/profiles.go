package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/netboot"
	"github.com/pxego/pxego/internal/store"
)

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

type ProfileHandler struct {
	store     store.Interface
	netbootMgr *netboot.Manager
}

func (h *ProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.store.ListProfiles(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, profiles)
}

func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	profile, err := h.store.GetProfile(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "配置未找到")
		return
	}
	OK(w, profile)
}

func (h *ProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
	var profile models.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	profile.ID = uuid.New().String()
	if err := h.store.CreateProfile(r.Context(), &profile); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Created(w, profile)
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var profile models.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	profile.ID = id
	if err := h.store.UpdateProfile(r.Context(), &profile); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, profile)
}

func (h *ProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteProfile(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
// CreateFromNetboot creates a profile from a netboot.xyz catalog entry.
func (h *ProfileHandler) CreateFromNetboot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DistroName      string `json:"distro_name"`
		VersionCodename string `json:"version_codename"`
		Arch            string `json:"arch"`
		ProfileName     string `json:"profile_name"`
		Description     string `json:"description,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.DistroName == "" || req.VersionCodename == "" || req.ProfileName == "" {
		Error(w, http.StatusBadRequest, "distro_name, version_codename, profile_name 为必填项")
		return
	}

	distro := h.netbootMgr.GetDistro(req.DistroName)
	if distro == nil {
		Error(w, http.StatusNotFound, fmt.Sprintf("发行版 %s 未找到", req.DistroName))
		return
	}

	arch := req.Arch
	if arch == "" {
		arch = "amd64"
	}
	var matchedVersion *netboot.Version
	for _, v := range distro.Versions {
		if v.Codename == req.VersionCodename && (v.Arch == arch || v.Arch == "") {
			matchedVersion = v
			break
		}
	}
	if matchedVersion == nil {
		Error(w, http.StatusNotFound, fmt.Sprintf("版本 %s (%s) 未找到", req.VersionCodename, arch))
		return
	}

	// Determine boot files — prefer Remote (full URLs) over Local (relative paths)
	// for portability across deployments. Local paths can't be used directly
	// since the PXE server address is not known at API handler time.
	var kernel, initrd string
	if matchedVersion.Remote != nil {
		kernel = matchedVersion.Remote.Kernel
		initrd = matchedVersion.Remote.Initrd
	} else if matchedVersion.Local != nil {
		kernel = matchedVersion.Local.Kernel
		initrd = matchedVersion.Local.Initrd
	}
	cmdline := matchedVersion.Cmdline

	// Map catalog BootType to engine entry type and populate type-specific fields
	entry := models.MenuEntry{
		Label:    req.ProfileName,
		Cmdline:  strPtr(cmdline),
	}
	switch matchedVersion.BootType {
	case netboot.BootKernel, netboot.BootMemdisk, netboot.BootMemtest, "":
		entry.Type = "direct"
		entry.Kernel = strPtr(kernel)
		entry.Initrd = strPtr(initrd)
	case netboot.BootWimboot:
		entry.Type = "wds"
		entry.WIM = strPtr(initrd)
		entry.URL = strPtr(kernel)
	case netboot.BootSanboot:
		entry.Type = "sanboot"
		entry.URL = strPtr(kernel)
		entry.SANAction = matchedVersion.SANAction
		entry.SANNoDescribe = matchedVersion.SANNoDescribe
		entry.SANDrive = matchedVersion.SANDrive
		entry.SANKeepSAN = matchedVersion.SANKeepSAN
	default:
		entry.Type = "custom"
		s := fmt.Sprintf("# %s — please configure boot parameters manually", matchedVersion.BootType)
		entry.Script = &s
	}

	profile := &models.Profile{
		ID:          uuid.New().String(),
		Name:        req.ProfileName,
		Description: req.Description,
	}
	if err := profile.SetMenu(&models.BootMenu{Entries: []models.MenuEntry{entry}}); err != nil {
		Error(w, http.StatusInternalServerError, fmt.Sprintf("创建引导菜单失败: %v", err))
		return
	}
	if err := h.store.CreateProfile(r.Context(), profile); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Created(w, profile)
}
