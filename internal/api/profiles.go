package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

// isPrintableASCII reports whether s contains only printable ASCII characters
// (0x20-0x7E). The iPXE BIOS console font only contains ASCII glyphs, so menu
// names containing non-ASCII characters (e.g. Chinese) would render as garbage
// on the boot menu.
func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7E {
			return false
		}
	}
	return true
}

type ProfileHandler struct {
	store     store.Interface
	netbootMgr *netboot.Manager
}

// menuEntryFromVersion maps a netboot catalog version to a boot menu entry.
// Remote URLs are preferred over Local paths for portability across deployments.
// Shared by CreateFromNetboot and the store netboot_distro importer.
func menuEntryFromVersion(v *netboot.Version, label string) models.MenuEntry {
	var kernel, initrd string
	if v.Remote != nil {
		kernel = v.Remote.Kernel
		initrd = v.Remote.Initrd
	} else if v.Local != nil {
		kernel = v.Local.Kernel
		initrd = v.Local.Initrd
	}
	entry := models.MenuEntry{
		Label:   label,
		Cmdline: strPtr(v.Cmdline),
	}
	switch v.BootType {
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
		entry.SANAction = v.SANAction
		entry.SANNoDescribe = v.SANNoDescribe
		entry.SANDrive = v.SANDrive
		entry.SANKeepSAN = v.SANKeepSAN
	default:
		entry.Type = "custom"
		s := fmt.Sprintf("# %s — please configure boot parameters manually", v.BootType)
		entry.Script = &s
	}
	return entry
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
	if profile.Name == "" {
		Error(w, http.StatusBadRequest, "菜单名称不能为空")
		return
	}
	if !isPrintableASCII(profile.Name) {
		Error(w, http.StatusBadRequest, "菜单名称只能包含英文、数字和符号（中文无法在启动菜单中显示）")
		return
	}
	profile.ID = uuid.New().String()
	if err := h.store.CreateProfile(r.Context(), &profile); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "profile", profile.Name, remoteIP(r), "新建引导配置: "+profile.Name+" ("+profile.Arch+")")
	Created(w, profile)
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	
	// 保存旧值
	oldProfile, _ := h.store.GetProfile(r.Context(), id)

	var profile models.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if profile.Name == "" {
		Error(w, http.StatusBadRequest, "菜单名称不能为空")
		return
	}
	if !isPrintableASCII(profile.Name) {
		Error(w, http.StatusBadRequest, "菜单名称只能包含英文、数字和符号（中文无法在启动菜单中显示）")
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

	// 构建变更详情
	var changes []string
	if oldProfile != nil {
		if oldProfile.Name != profile.Name {
			changes = append(changes, fmt.Sprintf("名称: %s→%s", oldProfile.Name, profile.Name))
		}
		if oldProfile.Description != profile.Description {
			changes = append(changes, fmt.Sprintf("描述: %s→%s", oldProfile.Description, profile.Description))
		}
		if oldProfile.Arch != profile.Arch {
			changes = append(changes, fmt.Sprintf("架构: %s→%s", oldProfile.Arch, profile.Arch))
		}
		if oldProfile.IsDefault != profile.IsDefault {
			changes = append(changes, fmt.Sprintf("默认: %t→%t", oldProfile.IsDefault, profile.IsDefault))
		}
		if oldProfile.MenuJSON != profile.MenuJSON {
			oldMenu, _ := oldProfile.GetMenu()
			newMenu, _ := profile.GetMenu()
			if oldMenu != nil && newMenu != nil {
				oldLabels := make([]string, 0, len(oldMenu.Entries))
				for _, e := range oldMenu.Entries {
					oldLabels = append(oldLabels, e.Label)
				}
				newLabels := make([]string, 0, len(newMenu.Entries))
				for _, e := range newMenu.Entries {
					newLabels = append(newLabels, e.Label)
				}
				if len(oldMenu.Entries) != len(newMenu.Entries) {
					changes = append(changes, fmt.Sprintf("菜单条目: %d→%d", len(oldMenu.Entries), len(newMenu.Entries)))
				}
				oldSet := make(map[string]bool, len(oldLabels))
				for _, l := range oldLabels { oldSet[l] = true }
				newSet := make(map[string]bool, len(newLabels))
				for _, l := range newLabels { newSet[l] = true }
				var added, removed []string
				for _, l := range newLabels {
					if !oldSet[l] {
						added = append(added, l)
					}
				}
				for _, l := range oldLabels {
					if !newSet[l] {
						removed = append(removed, l)
					}
				}
				if len(added) > 0 {
					changes = append(changes, fmt.Sprintf("新增: %s", strings.Join(added, ", ")))
				}
				if len(removed) > 0 {
					changes = append(changes, fmt.Sprintf("移除: %s", strings.Join(removed, ", ")))
				}
				if len(added) == 0 && len(removed) == 0 && len(oldMenu.Entries) == len(newMenu.Entries) {
					changes = append(changes, "菜单内容已变更")
				}
			} else {
				changes = append(changes, "菜单已变更")
			}
		}
	}
	detail := "更新引导配置"
	if len(changes) > 0 {
		detail = strings.Join(changes, "; ")
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "profile", profile.Name, remoteIP(r), detail)
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
	// 保存旧值用于审计
	oldProfile, _ := h.store.GetProfile(r.Context(), id)
	if err := h.store.DeleteProfile(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail := "删除引导配置"
	if oldProfile != nil && oldProfile.Name != "" {
		detail = fmt.Sprintf("删除引导配置 %s", oldProfile.Name)
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "profile", id, remoteIP(r), detail)
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
	if !isPrintableASCII(req.ProfileName) {
		Error(w, http.StatusBadRequest, "菜单名称只能包含英文、数字和符号（中文无法在启动菜单中显示）")
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

	// Map catalog BootType to engine entry type and populate type-specific fields
	entry := menuEntryFromVersion(matchedVersion, req.ProfileName)

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
