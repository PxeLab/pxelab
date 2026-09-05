package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

// ScriptHandler manages standalone Scripts.
type ScriptHandler struct {
	store store.Interface
}

func NewScriptHandler(st store.Interface) *ScriptHandler {
	return &ScriptHandler{store: st}
}

// scriptDTO is the API response for a Script.
type scriptDTO struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // shell / bat / powershell
	Content     string `json:"content"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// scriptReq is the create/update request for a Script.
type scriptReq struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Description string `json:"description"`
}

// List returns all scripts.
func (h *ScriptHandler) List(w http.ResponseWriter, r *http.Request) {
	scripts, err := h.store.ListScripts(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]scriptDTO, 0, len(scripts))
	for i := range scripts {
		dtos = append(dtos, toScriptDTO(&scripts[i]))
	}
	OK(w, map[string]any{"scripts": dtos})
}

// Get returns a single script by ID.
func (h *ScriptHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的脚本 ID")
		return
	}
	sc, err := h.store.GetScript(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "脚本未找到")
		return
	}
	OK(w, toScriptDTO(sc))
}

// Create creates a new script.
func (h *ScriptHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req scriptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Name == "" {
		Error(w, http.StatusBadRequest, "脚本名称不能为空")
		return
	}
	if req.Content == "" {
		Error(w, http.StatusBadRequest, "脚本内容不能为空")
		return
	}
	scriptType := req.Type
	if scriptType == "" {
		scriptType = "shell"
	}
	sc := &models.Script{
		Name:        req.Name,
		Type:        scriptType,
		Content:     req.Content,
		Description: req.Description,
	}
	if err := h.store.CreateScript(r.Context(), sc); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "script", fmt.Sprintf("%d", sc.ID), remoteIP(r), "创建脚本: "+sc.Name)
	Created(w, toScriptDTO(sc))
}

// Update updates an existing script.
func (h *ScriptHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的脚本 ID")
		return
	}
	sc, err := h.store.GetScript(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, "脚本未找到")
		return
	}
	var req scriptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Name != "" {
		sc.Name = req.Name
	}
	if req.Type != "" {
		sc.Type = req.Type
	}
	if req.Content != "" {
		sc.Content = req.Content
	}
	sc.Description = req.Description

	if err := h.store.UpdateScript(r.Context(), sc); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "script", fmt.Sprintf("%d", sc.ID), remoteIP(r), "更新脚本: "+sc.Name)
	OK(w, toScriptDTO(sc))
}

// Delete deletes a script.
func (h *ScriptHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的脚本 ID")
		return
	}
	// 引用保护：脚本仍被任一初始化脚本（脚本集）或主机直接引用时禁止删除，
	// 避免静默破坏引用它的脚本集 / 主机初始化配置。
	refBaselines, _ := h.referencingBaselines(r.Context(), uint(id))
	hostRefs, _ := h.store.CountHostsByScript(r.Context(), uint(id))
	refCount := len(refBaselines) + int(hostRefs)
	if refCount > 0 {
		Error(w, http.StatusConflict,
			fmt.Sprintf("该脚本正被 %d 处引用（脚本集 %d 处、主机 %d 处），请先解除引用后再删除",
				refCount, len(refBaselines), hostRefs))
		return
	}
	if err := h.store.DeleteScript(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "script", fmt.Sprintf("%d", id), remoteIP(r), "删除脚本")
	w.WriteHeader(http.StatusNoContent)
}

// referencingBaselines returns the IDs of baselines that reference the script.
func (h *ScriptHandler) referencingBaselines(ctx context.Context, scriptID uint) ([]string, error) {
	baselines, err := h.store.ListBaselines(ctx)
	if err != nil {
		return nil, err
	}
	var refs []string
	for i := range baselines {
		assignments, err := h.store.ListBaselineScripts(ctx, baselines[i].ID)
		if err != nil {
			continue
		}
		for _, a := range assignments {
			if a.ScriptID == scriptID {
				refs = append(refs, baselines[i].ID)
				break
			}
		}
	}
	return refs, nil
}

// --- helpers ---

func toScriptDTO(sc *models.Script) scriptDTO {
	scriptType := sc.Type
	if scriptType == "" {
		scriptType = "shell"
	}
	return scriptDTO{
		ID:          sc.ID,
		Name:        sc.Name,
		Type:        scriptType,
		Content:     sc.Content,
		Description: sc.Description,
		CreatedAt:   sc.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   sc.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
