package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/store"
)

type ScriptHandler struct {
	store  store.Interface
	sm     *boot.ScriptManager
}

func NewScriptHandler(st store.Interface, sm *boot.ScriptManager) *ScriptHandler {
	return &ScriptHandler{store: st, sm: sm}
}

// GET /api/v1/scripts
func (h *ScriptHandler) List(w http.ResponseWriter, r *http.Request) {
	scripts, err := h.sm.ListScripts()
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, scripts)
}

// GET /api/v1/scripts/{id}
func (h *ScriptHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	content, err := h.sm.Load(id)
	if err != nil {
		Error(w, http.StatusNotFound, "script not found")
		return
	}
	meta, _ := h.sm.GetMeta(id)
	OK(w, struct {
		ID      string          `json:"id"`
		Content string          `json:"content"`
		Meta    *boot.ScriptMeta `json:"meta,omitempty"`
	}{ID: id, Content: content, Meta: meta})
}

// PUT /api/v1/scripts/{id}
func (h *ScriptHandler) Save(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		Content string `json:"content"`
		Comment string `json:"comment,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	meta, _ := h.sm.GetMeta(id)
	profileID := ""
	label := id
	entryType := "custom"
	if meta != nil {
		profileID = meta.ProfileID
		label = meta.Label
		entryType = meta.Type
	}
	ver, err := h.sm.Save(id, profileID, label, entryType, req.Content, req.Comment)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, ver)
}

// POST /api/v1/scripts/sync — sync/create a script from a profile entry
func (h *ScriptHandler) Sync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID string `json:"profile_id"`
		Label     string `json:"label"`
		Type      string `json:"type"`
		Content   string `json:"content"`
		Comment   string `json:"comment,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.ProfileID == "" || req.Label == "" {
		Error(w, http.StatusBadRequest, "profile_id and label are required")
		return
	}
	scriptID := sanitizeScriptID(req.ProfileID, req.Label)
	ver, changed, err := h.sm.SaveIfChanged(scriptID, req.ProfileID, req.Label, req.Type, req.Content, req.Comment)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !changed {
		OK(w, map[string]any{"changed": false})
		return
	}
	OK(w, map[string]any{"changed": true, "version": ver})
}

func sanitizeScriptID(profileID, label string) string {
	id := profileID + "-" + label
	// Replace characters that are unsafe in filenames
	id = strings.NewReplacer(
		"/", "_", "\\", "_", " ", "_",
		"..", "_", "~", "_", ":", "_",
	).Replace(id)
	return id
}

// GET /api/v1/scripts/{id}/versions
func (h *ScriptHandler) ListVersions(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	versions, err := h.sm.ListVersions(id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, versions)
}

// GET /api/v1/scripts/{id}/versions/{ver}
func (h *ScriptHandler) GetVersion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	verID := chi.URLParam(r, "ver")
	ver, err := h.sm.GetVersion(id, verID)
	if err != nil {
		Error(w, http.StatusNotFound, "version not found")
		return
	}
	OK(w, ver)
}

// GET /api/v1/scripts/{id}/diff/{ver}
func (h *ScriptHandler) Diff(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	verID := chi.URLParam(r, "ver")
	diff, err := h.sm.Diff(id, verID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]string{"diff": diff})
}

// POST /api/v1/scripts/{id}/rollback/{ver}
func (h *ScriptHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	verID := chi.URLParam(r, "ver")
	ver, err := h.sm.Rollback(id, verID, "rollback to "+verID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, ver)
}
