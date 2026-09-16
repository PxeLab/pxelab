package api

// Webhook 通知设置（R3）：/api/v1/settings/webhooks CRUD + 发送测试消息。
// 配置存 config.yaml 的 notify.webhooks，保存即写回；管理只走 UI。

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/notify"
)

// webhookReq 是创建/更新 webhook 的请求体（id 由服务端管理）。
type webhookReq struct {
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Events  []string `json:"events"`
	Format  string   `json:"format"`
	Secret  string   `json:"secret"`
	Enabled bool     `json:"enabled"`
}

// validateWebhookReq 校验请求字段，返回规范化后的 format 与错误文案。
func validateWebhookReq(req *webhookReq) (string, string) {
	if strings.TrimSpace(req.Name) == "" {
		return "", "名称不能为空"
	}
	if strings.TrimSpace(req.URL) == "" {
		return "", "URL 不能为空"
	}
	format := req.Format
	if format == "" {
		format = "generic"
	}
	switch format {
	case "generic", "dingtalk", "feishu":
	default:
		return "", "format 无效（仅支持 generic / dingtalk / feishu）"
	}
	if len(req.Events) == 0 {
		return "", "至少订阅一个事件"
	}
	return format, ""
}

// ListWebhooks 返回全部 webhook 配置。
func (h *SettingsHandler) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	webhooks := h.cfg.Notify.Webhooks
	h.mu.Unlock()
	if webhooks == nil {
		webhooks = []config.WebhookConfig{}
	}
	OK(w, map[string]any{"webhooks": webhooks})
}

// CreateWebhook 新增 webhook 并保存配置。
func (h *SettingsHandler) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req webhookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	format, msg := validateWebhookReq(&req)
	if msg != "" {
		Error(w, http.StatusBadRequest, msg)
		return
	}
	wh := config.WebhookConfig{
		ID:      "wh_" + uuid.New().String(),
		Name:    strings.TrimSpace(req.Name),
		URL:     strings.TrimSpace(req.URL),
		Events:  req.Events,
		Format:  format,
		Secret:  req.Secret,
		Enabled: req.Enabled,
	}

	h.mu.Lock()
	h.cfg.Notify.Webhooks = append(h.cfg.Notify.Webhooks, wh)
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "webhook", wh.ID, remoteIP(r), "新建 webhook: "+wh.Name)
	Created(w, wh)
}

// UpdateWebhook 更新指定 webhook 并保存配置。
func (h *SettingsHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req webhookReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	format, msg := validateWebhookReq(&req)
	if msg != "" {
		Error(w, http.StatusBadRequest, msg)
		return
	}

	h.mu.Lock()
	idx := -1
	for i, wh := range h.cfg.Notify.Webhooks {
		if wh.ID == id {
			idx = i
			break
		}
	}
	if idx >= 0 {
		h.cfg.Notify.Webhooks[idx] = config.WebhookConfig{
			ID:      id,
			Name:    strings.TrimSpace(req.Name),
			URL:     strings.TrimSpace(req.URL),
			Events:  req.Events,
			Format:  format,
			Secret:  req.Secret,
			Enabled: req.Enabled,
		}
	}
	h.mu.Unlock()
	if idx < 0 {
		Error(w, http.StatusNotFound, "webhook 未找到")
		return
	}

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "webhook", id, remoteIP(r), "更新 webhook: "+req.Name)
	OK(w, h.cfg.Notify.Webhooks[idx])
}

// DeleteWebhook 删除指定 webhook 并保存配置。
func (h *SettingsHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	h.mu.Lock()
	filtered := h.cfg.Notify.Webhooks[:0]
	found := false
	for _, wh := range h.cfg.Notify.Webhooks {
		if wh.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, wh)
	}
	h.cfg.Notify.Webhooks = filtered
	h.mu.Unlock()
	if !found {
		Error(w, http.StatusNotFound, "webhook 未找到")
		return
	}

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "webhook", id, remoteIP(r), "删除 webhook: "+id)
	w.WriteHeader(http.StatusNoContent)
}

// TestWebhook 向指定 webhook 发送一条测试消息，同步返回投递结果。
func (h *SettingsHandler) TestWebhook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	h.mu.Lock()
	var target *config.WebhookConfig
	for i, wh := range h.cfg.Notify.Webhooks {
		if wh.ID == id {
			cp := h.cfg.Notify.Webhooks[i]
			target = &cp
			break
		}
	}
	h.mu.Unlock()
	if target == nil {
		Error(w, http.StatusNotFound, "webhook 未找到")
		return
	}

	evt := notify.Event{
		Event:  "test",
		Time:   time.Now(),
		Host:   notify.HostInfo{MAC: "00:00:00:00:00:00", Name: "test-host", IP: "0.0.0.0"},
		Detail: "这是一条来自 PxeLab 的 webhook 测试消息",
	}
	// 测试消息单次投递（不重试），快速反馈结果
	d := notify.NewDispatcher(nil, h.store)
	if err := d.DeliverOnce(*target, evt); err != nil {
		RecordAudit(r.Context(), h.store, models.AuditUpdate, "webhook", id, remoteIP(r), "webhook 测试消息投递失败: "+target.Name)
		Error(w, http.StatusBadGateway, "投递失败: "+err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "webhook", id, remoteIP(r), "webhook 测试消息投递成功: "+target.Name)
	OK(w, map[string]any{"delivered": true})
}
