package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
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
	RecordAudit(r.Context(), h.store, models.AuditCreate, "profile", profile.Name, remoteIP(r), "新建引导配置")
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
	// Auto-create script version snapshot if custom entry script changed
	if menu, err := profile.GetMenu(); err == nil && len(menu.Entries) > 0 && menu.Entries[0].Type == "custom" {
		if script := menu.Entries[0].Script; script != nil && *script != "" {
			h.createScriptVersion(r.Context(), id, *script)
		}
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "profile", profile.Name, remoteIP(r), "更新引导配置")
	OK(w, profile)
}

func scriptChecksum(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

func (h *ProfileHandler) createScriptVersion(ctx context.Context, profileID, content string) {
	latest, err := h.store.GetLatestScriptVersion(ctx, profileID)
	cs := scriptChecksum(content)
	if err == nil && latest.Checksum == cs {
		return // content unchanged, skip
	}
	_ = h.store.CreateScriptVersion(ctx, &models.ProfileScriptVersion{
		ProfileID: profileID,
		Content:   content,
		Checksum:  cs,
		Comment:   "auto-saved on profile update",
	})
}

func (h *ProfileHandler) ListScriptVersions(w http.ResponseWriter, r *http.Request) {
	profileID := chi.URLParam(r, "profileId")
	versions, err := h.store.ListScriptVersions(r.Context(), profileID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, versions)
}

func (h *ProfileHandler) GetScriptVersion(w http.ResponseWriter, r *http.Request) {
	verID, err := strconv.ParseUint(chi.URLParam(r, "verId"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的版本 ID")
		return
	}
	ver, err := h.store.GetScriptVersion(r.Context(), uint(verID))
	if err != nil {
		Error(w, http.StatusNotFound, "版本未找到")
		return
	}
	OK(w, ver)
}

func (h *ProfileHandler) DiffScriptVersion(w http.ResponseWriter, r *http.Request) {
	profileID := chi.URLParam(r, "profileId")
	verID, err := strconv.ParseUint(chi.URLParam(r, "verId"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的版本 ID")
		return
	}
	ver, err := h.store.GetScriptVersion(r.Context(), uint(verID))
	if err != nil {
		Error(w, http.StatusNotFound, "版本未找到")
		return
	}
	// Current content from profile
	profile, err := h.store.GetProfile(r.Context(), profileID)
	if err != nil {
		Error(w, http.StatusNotFound, "配置未找到")
		return
	}
	menu, _ := profile.GetMenu()
	current := ""
	if menu != nil && len(menu.Entries) > 0 && menu.Entries[0].Script != nil {
		current = *menu.Entries[0].Script
	}
	diff := simpleDiff(ver.Content, current)
	OK(w, map[string]string{"diff": diff})
}

func (h *ProfileHandler) RollbackScriptVersion(w http.ResponseWriter, r *http.Request) {
	profileID := chi.URLParam(r, "profileId")
	verID, err := strconv.ParseUint(chi.URLParam(r, "verId"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的版本 ID")
		return
	}
	ver, err := h.store.GetScriptVersion(r.Context(), uint(verID))
	if err != nil {
		Error(w, http.StatusNotFound, "版本未找到")
		return
	}
	// Update profile's script content to the version content
	profile, err := h.store.GetProfile(r.Context(), profileID)
	if err != nil {
		Error(w, http.StatusNotFound, "配置未找到")
		return
	}
	menu, err := profile.GetMenu()
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(menu.Entries) > 0 && menu.Entries[0].Type == "custom" {
		menu.Entries[0].Script = &ver.Content
	}
	if err := profile.SetMenu(menu); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.store.UpdateProfile(r.Context(), profile); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Create a version snapshot for the rollback
	h.createScriptVersion(r.Context(), profileID, ver.Content)
	OK(w, profile)
}

// simpleDiff produces a line-by-line diff with "+"/"-"/" " prefixes.
func simpleDiff(oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)
	oldMap := make(map[string]int)
	for _, l := range oldLines {
		oldMap[l]++
	}
	newMap := make(map[string]int)
	for _, l := range newLines {
		newMap[l]++
	}
	var result string
	i, j := 0, 0
	for i < len(oldLines) || j < len(newLines) {
		if i < len(oldLines) && j < len(newLines) && oldLines[i] == newLines[j] {
			result += " " + oldLines[i] + "\n"
			i++
			j++
		} else if j < len(newLines) && (i >= len(oldLines) || newMap[newLines[j]] > 0 && oldMap[newLines[j]] == 0) {
			result += "+" + newLines[j] + "\n"
			newMap[newLines[j]]--
			j++
		} else if i < len(oldLines) {
			result += "-" + oldLines[i] + "\n"
			oldMap[oldLines[i]]--
			i++
		} else {
			result += "+" + newLines[j] + "\n"
			j++
		}
	}
	return result
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func (h *ProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteProfile(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "profile", id, remoteIP(r), "删除引导配置")
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
	RecordAudit(r.Context(), h.store, models.AuditCreate, "profile", profile.Name, remoteIP(r), "从网络引导目录新建: "+req.DistroName+"/"+req.VersionCodename)
	Created(w, profile)
}
