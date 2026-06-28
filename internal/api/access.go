package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

type AccessHandler struct {
	store store.Interface
}

func NewAccessHandler(st store.Interface) *AccessHandler {
	return &AccessHandler{store: st}
}

// ── Blacklist ──

func (h *AccessHandler) ListBlacklist(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.ListBlacklist(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "查询黑名单失败")
		return
	}
	OK(w, entries)
}

type createBlacklistRequest struct {
	MAC    string `json:"mac"`
	Reason string `json:"reason,omitempty"`
}

func (h *AccessHandler) CreateBlacklist(w http.ResponseWriter, r *http.Request) {
	var req createBlacklistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.MAC == "" {
		Error(w, http.StatusBadRequest, "MAC 地址不能为空")
		return
	}
	entry := &models.BlacklistEntry{
		MAC:    req.MAC,
		Reason: req.Reason,
		Source: "db",
	}
	if err := h.store.CreateBlacklist(r.Context(), entry); err != nil {
		Error(w, http.StatusConflict, "MAC 已存在于黑名单")
		return
	}
	OK(w, entry)
}

func (h *AccessHandler) DeleteBlacklist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.store.DeleteBlacklist(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusNotFound, "条目未找到")
		return
	}
	OK(w, map[string]string{"status": "deleted"})
}

// ── Whitelist ──

func (h *AccessHandler) ListWhitelist(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.ListWhitelist(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "查询白名单失败")
		return
	}
	OK(w, entries)
}

type createWhitelistRequest struct {
	MAC        string `json:"mac"`
	SubnetCIDR string `json:"subnet_cidr"`
	Reason     string `json:"reason,omitempty"`
}

func (h *AccessHandler) CreateWhitelist(w http.ResponseWriter, r *http.Request) {
	var req createWhitelistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.MAC == "" || req.SubnetCIDR == "" {
		Error(w, http.StatusBadRequest, "MAC 和子网 CIDR 不能为空")
		return
	}
	entry := &models.WhitelistEntry{
		MAC:        req.MAC,
		SubnetCIDR: req.SubnetCIDR,
		Reason:     req.Reason,
		Source:     "db",
	}
	if err := h.store.CreateWhitelist(r.Context(), entry); err != nil {
		Error(w, http.StatusConflict, "该 MAC 已在此子网的白名单中")
		return
	}
	OK(w, entry)
}

func (h *AccessHandler) DeleteWhitelist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.store.DeleteWhitelist(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusNotFound, "条目未找到")
		return
	}
	OK(w, map[string]string{"status": "deleted"})
}
