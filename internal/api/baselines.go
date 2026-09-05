package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/baseline"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

// BaselineHandler 管理安全基线及其脚本。
type BaselineHandler struct {
	store        store.Interface
	identityAttr string // "mac"（默认）或 "sn"：机器拉取初始化基线时用哪个主机身份键
}

func NewBaselineHandler(st store.Interface, identityAttr string) *BaselineHandler {
	if identityAttr == "" {
		identityAttr = "mac"
	}
	return &BaselineHandler{store: st, identityAttr: identityAttr}
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

// baselineScriptAssignmentDTO 是基线关联脚本的 API 响应。
type baselineScriptAssignmentDTO struct {
	ScriptID uint `json:"script_id"`
	Seq      int  `json:"seq"`
	*scriptDTO
}

// setScriptsReq 是批量设置基线关联脚本的请求体。
type setScriptsReq struct {
	Scripts []struct {
		ScriptID uint `json:"script_id"`
		Seq      int  `json:"seq"`
	} `json:"scripts"`
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

// ListScripts 列出基线关联的所有脚本（含脚本详情）。
func (h *BaselineHandler) ListScripts(w http.ResponseWriter, r *http.Request) {
	blID := chi.URLParam(r, "id")
	assignments, err := h.store.ListBaselineScripts(r.Context(), blID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]baselineScriptAssignmentDTO, 0, len(assignments))
	for _, a := range assignments {
		sc, err := h.store.GetScript(r.Context(), a.ScriptID)
		if err != nil {
			continue // script might have been deleted
		}
		sd := toScriptDTO(sc)
		dtos = append(dtos, baselineScriptAssignmentDTO{
			ScriptID:  a.ScriptID,
			Seq:       a.Seq,
			scriptDTO: &sd,
		})
	}
	// 按 seq 排序
	sort.Slice(dtos, func(i, j int) bool { return dtos[i].Seq < dtos[j].Seq })
	OK(w, map[string]any{"scripts": dtos})
}

// SetScripts 批量设置基线关联的脚本（全量替换）。
func (h *BaselineHandler) SetScripts(w http.ResponseWriter, r *http.Request) {
	blID := chi.URLParam(r, "id")
	var req setScriptsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	assignments := make([]models.BaselineScriptAssignment, len(req.Scripts))
	for i, s := range req.Scripts {
		assignments[i] = models.BaselineScriptAssignment{
			BaselineID: blID,
			ScriptID:   s.ScriptID,
			Seq:        s.Seq,
		}
	}
	if err := h.store.SetBaselineScripts(r.Context(), blID, assignments); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "baseline", blID, remoteIP(r), "更新基线脚本关联: "+blID)
	OK(w, map[string]any{"scripts": req.Scripts})
}

// inlineScriptReq creates a new library script and appends it to the baseline.
type inlineScriptReq struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Description string `json:"description"`
}

// CreateAndAddScript creates a script in the shared script library and appends
// it to the end of the baseline's ordered script list in one user action
// (L2 inline create — no separate trip to the 脚本库 view needed).
func (h *BaselineHandler) CreateAndAddScript(w http.ResponseWriter, r *http.Request) {
	blID := chi.URLParam(r, "id")
	if _, err := h.store.GetBaseline(r.Context(), blID); err != nil {
		Error(w, http.StatusNotFound, "脚本集未找到")
		return
	}
	var req inlineScriptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		Error(w, http.StatusBadRequest, "脚本名称不能为空")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		Error(w, http.StatusBadRequest, "脚本内容不能为空")
		return
	}
	scriptType := req.Type
	if scriptType == "" {
		scriptType = "shell"
	}
	sc := &models.Script{
		Name:        strings.TrimSpace(req.Name),
		Type:        scriptType,
		Content:     req.Content,
		Description: req.Description,
	}
	if err := h.store.CreateScript(r.Context(), sc); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	assignments, err := h.store.ListBaselineScripts(r.Context(), blID)
	if err != nil {
		h.store.DeleteScript(r.Context(), sc.ID)
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	nextSeq := 1
	for _, a := range assignments {
		if a.Seq >= nextSeq {
			nextSeq = a.Seq + 1
		}
	}
	assignments = append(assignments, models.BaselineScriptAssignment{
		BaselineID: blID,
		ScriptID:   sc.ID,
		Seq:        nextSeq,
	})
	if err := h.store.SetBaselineScripts(r.Context(), blID, assignments); err != nil {
		h.store.DeleteScript(r.Context(), sc.ID)
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	RecordAudit(r.Context(), h.store, models.AuditCreate, "script", fmt.Sprintf("%d", sc.ID), remoteIP(r), "新建脚本: "+sc.Name)
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "baseline", blID, remoteIP(r), "新增脚本到脚本集: "+sc.Name)
	Created(w, map[string]any{
		"baseline_id": blID,
		"seq":         nextSeq,
		"script":      toScriptDTO(sc),
	})
}

// GetAssigned 根据 MAC 地址获取机器关联的所有基线脚本（已渲染变量）。
// GET /baselines/assigned?mac=xx:xx:xx:xx:xx
// 语义：主机未注册 → 404（调试用）；已注册但无任何配置 → 200 空列表。
func (h *BaselineHandler) GetAssigned(w http.ResponseWriter, r *http.Request) {
	mac := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mac")))
	if mac == "" {
		Error(w, http.StatusBadRequest, "mac 参数不能为空")
		return
	}
	host, err := h.store.GetHostByMAC(r.Context(), mac)
	if err != nil {
		Error(w, http.StatusNotFound, "未找到该 MAC 对应的主机")
		return
	}
	scripts, err := h.collectScripts(r.Context(), host, "")
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"scripts": scripts})
}

// PullScripts 是给自动应答/装机钩子用的拉取接口（无配置一律 200 空列表）。
// GET /baselines/pull?mac=xx|sn=xx [&type=shell|bat|powershell]
func (h *BaselineHandler) PullScripts(w http.ResponseWriter, r *http.Request) {
	host, found := h.resolveHost(r)
	if !found {
		OK(w, map[string]any{"scripts": []assignedScriptDTO{}})
		return
	}
	typeFilter := strings.TrimSpace(r.URL.Query().Get("type"))
	scripts, err := h.collectScripts(r.Context(), host, typeFilter)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"scripts": scripts})
}

// resolveHost 按配置的身份键（mac/sn）查找主机；查不到返回 found=false。
func (h *BaselineHandler) resolveHost(r *http.Request) (*models.Host, bool) {
	ctx := r.Context()
	if h.identityAttr == "sn" {
		sn := strings.TrimSpace(r.URL.Query().Get("sn"))
		if sn == "" {
			return nil, false
		}
		host, err := h.store.GetHostBySN(ctx, sn)
		if err != nil {
			return nil, false
		}
		return host, true
	}
	mac := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mac")))
	if mac == "" {
		return nil, false
	}
	host, err := h.store.GetHostByMAC(ctx, mac)
	if err != nil {
		return nil, false
	}
	return host, true
}

// collectScripts 汇总主机应执行的初始化脚本：
//
//	生效脚本集 = 所属 Profile 的基线（继承）+ 主机额外勾选的基线（追加，去重）
//	生效单脚本 = 主机直接勾选的脚本库脚本（追加）
//
// 顺序：基线按（Profile 列表 → 主机追加列表）展开、集内按 seq；最后是直选脚本。
// 全程按脚本 ID 全局去重；可选 type 过滤；变量按 baseline > profile 合并渲染。
func (h *BaselineHandler) collectScripts(ctx context.Context, host *models.Host, typeFilter string) ([]assignedScriptDTO, error) {
	// 1. 生效脚本集（继承 + 追加，去重保序）：先 Profile 继承、后主机追加
	var baselineIDs []string
	seenBl := map[string]bool{}
	addBL := func(ids []string) {
		for _, id := range ids {
			if id != "" && !seenBl[id] {
				seenBl[id] = true
				baselineIDs = append(baselineIDs, id)
			}
		}
	}

	profileVars := map[string]string{}
	if host.ProfileID != nil && *host.ProfileID != "" {
		if prof, err := h.store.GetProfile(ctx, *host.ProfileID); err == nil {
			if pid, err := prof.GetBaselines(); err == nil {
				addBL(pid) // 先继承
			}
			if pv, err := prof.GetVariablesMap(); err == nil && pv != nil {
				profileVars = pv
			}
		}
	}
	if hostExtra, err := host.GetBaselineIDs(); err == nil {
		addBL(hostExtra) // 后追加
	}

	// 2. 直选单脚本（先取出 ID，追加在基线脚本之后）
	directIDs, err := host.GetScriptIDs()
	if err != nil {
		return nil, err
	}

	var out []assignedScriptDTO
	seenScript := map[uint]bool{}

	seq := 1
	for _, blID := range baselineIDs {
		bl, err := h.store.GetBaseline(ctx, blID)
		if err != nil {
			continue
		}
		blVars, _ := bl.GetVariablesMap()
		assignments, err := h.store.ListBaselineScripts(ctx, blID)
		if err != nil {
			continue
		}
		for _, a := range assignments {
			sc, err := h.store.GetScript(ctx, a.ScriptID)
			if err != nil || seenScript[sc.ID] {
				continue
			}
			if typeFilter != "" && sc.Type != typeFilter {
				continue
			}
			seenScript[sc.ID] = true
			rendered := sc.Content
			if r2, err := baseline.RenderScript(sc.Content, blVars, profileVars, nil); err == nil {
				rendered = r2
			}
			out = append(out, assignedScriptDTO{Seq: seq, Name: sc.Name, Type: sc.Type, Content: rendered, Description: sc.Description})
			seq++
		}
	}
	for _, sid := range directIDs {
		sc, err := h.store.GetScript(ctx, sid)
		if err != nil || seenScript[sc.ID] {
			continue
		}
		if typeFilter != "" && sc.Type != typeFilter {
			continue
		}
		seenScript[sc.ID] = true
		rendered := sc.Content
		if r2, err := baseline.RenderScript(sc.Content, nil, profileVars, nil); err == nil {
			rendered = r2
		}
		out = append(out, assignedScriptDTO{Seq: seq, Name: sc.Name, Type: sc.Type, Content: rendered, Description: sc.Description})
		seq++
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
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
