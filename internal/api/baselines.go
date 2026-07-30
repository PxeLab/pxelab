package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/baseline"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

// BaselineHandler 管理安全基线及其脚本。
type BaselineHandler struct {
	store store.Interface
}

// baselineDTO 是 API 层的基线响应结构。
type baselineDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OSFilter    string `json:"os_filter"`
	Variables   string `json:"variables,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// baselineReq 是创建/更新基线的请求体。
type baselineReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	OSFilter    string `json:"os_filter"`
	Variables   string `json:"variables,omitempty"`
}

// baselineScriptDTO 是 API 层的脚本响应结构。
type baselineScriptDTO struct {
	ID          uint   `json:"id"`
	BaselineID  string `json:"baseline_id"`
	Seq         int    `json:"seq"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Description string `json:"description"`
}

// baselineScriptReq 是创建/更新脚本的请求体。
type baselineScriptReq struct {
	Seq         int    `json:"seq"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Description string `json:"description"`
}

// assignedScriptDTO 是机器拉取端点返回的脚本结构（已渲染）。
type assignedScriptDTO struct {
	Seq         int    `json:"seq"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Content     string `json:"content"` // 已渲染
	Description string `json:"description"`
}

// List 列出所有基线。
func (h *BaselineHandler) List(w http.ResponseWriter, r *http.Request) {
	baselines, err := h.store.ListBaselines(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]baselineDTO, 0, len(baselines))
	for i := range baselines {
		dtos = append(dtos, baselineToDTO(&baselines[i]))
	}
	OK(w, map[string]any{"baselines": dtos})
}

// Get 获取单个基线详情。
func (h *BaselineHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	bl, err := h.store.GetBaseline(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "基线未找到")
		return
	}
	OK(w, baselineToDTO(bl))
}

// Create 创建新基线。
func (h *BaselineHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req baselineReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Name == "" {
		Error(w, http.StatusBadRequest, "名称不能为空")
		return
	}

	bl := &models.Baseline{
		ID:          req.Name, // 使用 name 作为 ID（短名称风格如 "bl-sec"）
		Name:        req.Name,
		Description: req.Description,
		OSFilter:    req.OSFilter,
	}
	if req.Variables != "" {
		bl.Variables = req.Variables
	}

	if err := h.store.CreateBaseline(r.Context(), bl); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "baseline", bl.ID, remoteIP(r), "创建基线: "+bl.Name)
	Created(w, baselineToDTO(bl))
}

// Update 更新基线。
func (h *BaselineHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	bl, err := h.store.GetBaseline(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "基线未找到")
		return
	}

	var req baselineReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Name != "" {
		bl.Name = req.Name
	}
	if req.Description != "" {
		bl.Description = req.Description
	}
	bl.OSFilter = req.OSFilter
	bl.Variables = req.Variables

	if err := h.store.UpdateBaseline(r.Context(), bl); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "baseline", bl.ID, remoteIP(r), "更新基线: "+bl.Name)
	OK(w, baselineToDTO(bl))
}

// Delete 删除基线（级联删除关联脚本）。
func (h *BaselineHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteBaseline(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "baseline", id, remoteIP(r), "删除基线: "+id)
	w.WriteHeader(http.StatusNoContent)
}

// ListScripts 列出基线下的所有脚本。
func (h *BaselineHandler) ListScripts(w http.ResponseWriter, r *http.Request) {
	blID := chi.URLParam(r, "id")
	scripts, err := h.store.ListBaselineScripts(r.Context(), blID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]baselineScriptDTO, 0, len(scripts))
	for i := range scripts {
		dtos = append(dtos, scriptToDTO(&scripts[i]))
	}
	// 按 seq 排序
	sort.Slice(dtos, func(i, j int) bool { return dtos[i].Seq < dtos[j].Seq })
	OK(w, map[string]any{"scripts": dtos})
}

// UpsertScript 创建或更新基线下的脚本。
func (h *BaselineHandler) UpsertScript(w http.ResponseWriter, r *http.Request) {
	blID := chi.URLParam(r, "id")

	var req baselineScriptReq
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

	sc := &models.BaselineScript{
		BaselineID:  blID,
		Seq:         req.Seq,
		Name:        req.Name,
		Type:        req.Type,
		Content:     req.Content,
		Description: req.Description,
	}

	if err := h.store.UpsertBaselineScript(r.Context(), sc); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "baseline_script", blID, remoteIP(r), "更新脚本: "+sc.Name+" in baseline "+blID)
	OK(w, scriptToDTO(sc))
}

// DeleteScript 删除基线下的单个脚本。
func (h *BaselineHandler) DeleteScript(w http.ResponseWriter, r *http.Request) {
	scriptIDStr := chi.URLParam(r, "scriptId")
	// 简单解析 id（暂时只支持数字 uint）
	var id uint
	if _, err := fmt.Sscanf(scriptIDStr, "%d", &id); err != nil {
		Error(w, http.StatusBadRequest, "无效的脚本 ID")
		return
	}
	if err := h.store.DeleteBaselineScript(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetAssigned 根据 MAC 地址获取机器关联的所有基线脚本（已渲染变量）。
// GET /baselines/assigned?mac=xx:xx:xx:xx:xx
func (h *BaselineHandler) GetAssigned(w http.ResponseWriter, r *http.Request) {
	mac := r.URL.Query().Get("mac")
	if mac == "" {
		Error(w, http.StatusBadRequest, "mac 参数不能为空")
		return
	}

	// 1. 通过 MAC 查找 Host
	host, err := h.store.GetHostByMAC(r.Context(), mac)
	if err != nil {
		Error(w, http.StatusNotFound, "未找到该 MAC 对应的主机")
		return
	}

	// 2. 通过 Host 的 ProfileID 查找 Profile
	if host.ProfileID == nil || *host.ProfileID == "" {
		Error(w, http.StatusNotFound, "该主机未关联任何 Profile")
		return
	}
	prof, err := h.store.GetProfile(r.Context(), *host.ProfileID)
	if err != nil {
		Error(w, http.StatusNotFound, "关联的 Profile 未找到")
		return
	}

	// 3. 获取 profile 关联的 baseline IDs
	baselineIDs, err := prof.GetBaselines()
	if err != nil || len(baselineIDs) == 0 {
		OK(w, map[string]any{"scripts": []assignedScriptDTO{}})
		return
	}

	// 4. 收集所有关联的基线 + 脚本
	profileVars, _ := prof.GetVariablesMap()

	var allScripts []assignedScriptDTO
	for _, blID := range baselineIDs {
		bl, err := h.store.GetBaseline(r.Context(), blID)
		if err != nil {
			continue
		}
		baselineVars, _ := bl.GetVariablesMap()

		scripts, err := h.store.ListBaselineScripts(r.Context(), blID)
		if err != nil {
			continue
		}

		for _, sc := range scripts {
			rendered, err := baseline.RenderScript(sc.Content, baselineVars, profileVars, nil)
			if err != nil {
				rendered = sc.Content // 渲染失败时回退到原始内容
			}

			allScripts = append(allScripts, assignedScriptDTO{
				Seq:         sc.Seq,
				Name:        sc.Name,
				Type:        sc.Type,
				Content:     rendered,
				Description: sc.Description,
			})
		}
	}

	// 按 seq 排序
	sort.Slice(allScripts, func(i, j int) bool { return allScripts[i].Seq < allScripts[j].Seq })

	OK(w, map[string]any{"scripts": allScripts})
}

// --- helper ---

func baselineToDTO(bl *models.Baseline) baselineDTO {
	return baselineDTO{
		ID:          bl.ID,
		Name:        bl.Name,
		Description: bl.Description,
		OSFilter:    bl.OSFilter,
		Variables:   bl.Variables,
		CreatedAt:   bl.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   bl.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func scriptToDTO(sc *models.BaselineScript) baselineScriptDTO {
	scriptType := sc.Type
	if scriptType == "" {
		scriptType = "shell"
	}
	return baselineScriptDTO{
		ID:          sc.ID,
		BaselineID:  sc.BaselineID,
		Seq:         sc.Seq,
		Name:        sc.Name,
		Type:        scriptType,
		Content:     sc.Content,
		Description: sc.Description,
	}
}
