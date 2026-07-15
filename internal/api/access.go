package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
	"gorm.io/gorm"
)

var macRe = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`)

type AccessHandler struct {
	store  store.Interface
	logger *slog.Logger
}

func NewAccessHandler(st store.Interface) *AccessHandler {
	return &AccessHandler{store: st, logger: slog.With("service", "HTTP")}
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
	mac := strings.TrimSpace(req.MAC)
	if !macRe.MatchString(mac) {
		Error(w, http.StatusBadRequest, "MAC 地址格式无效，请使用 00:11:22:33:44:55 格式")
		return
	}
	if _, err := net.ParseMAC(mac); err != nil {
		Error(w, http.StatusBadRequest, "MAC 地址格式无效")
		return
	}
	entry := &models.BlacklistEntry{
		MAC:    mac,
		Reason: req.Reason,
		Source: "手动添加",
	}
	if err := h.store.CreateBlacklist(r.Context(), entry); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			Error(w, http.StatusConflict, "该 MAC 已存在于黑名单")
		} else {
			h.logger.Error("创建黑名单条目失败", "mac", mac, "error", err)
			Error(w, http.StatusInternalServerError, "创建失败")
		}
		return
	}
	h.logger.Info("黑名单已添加", "mac", mac)
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(w, http.StatusNotFound, "条目未找到")
		} else {
			h.logger.Error("删除黑名单条目失败", "id", id, "error", err)
			Error(w, http.StatusInternalServerError, "删除失败")
		}
		return
	}
	h.logger.Info("黑名单已删除", "id", id)
	OK(w, map[string]string{"status": "deleted"})
}

func (h *AccessHandler) DeleteWhitelist(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.store.DeleteWhitelist(r.Context(), uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			Error(w, http.StatusNotFound, "条目未找到")
		} else {
			h.logger.Error("删除白名单条目失败", "id", id, "error", err)
			Error(w, http.StatusInternalServerError, "删除失败")
		}
		return
	}
	h.logger.Info("白名单已删除", "id", id)
	OK(w, map[string]string{"status": "deleted"})
}

// ── Unauthorized Devices ──

func (h *AccessHandler) ListUnauthorizedDevices(w http.ResponseWriter, r *http.Request) {
	entries, err := h.store.ListUnauthorizedDevices(r.Context())
	if err != nil {
		h.logger.Error("查询未授权设备失败", "error", err)
		Error(w, http.StatusInternalServerError, "查询未授权设备失败")
		return
	}
	OK(w, entries)
}

type addToWhitelistFromUnauthorizedRequest struct {
	MAC        string `json:"mac"`
	SubnetCIDR string `json:"subnet_cidr"`
}

func (h *AccessHandler) AddToWhitelistFromUnauthorized(w http.ResponseWriter, r *http.Request) {
	var req addToWhitelistFromUnauthorizedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	mac := strings.TrimSpace(req.MAC)
	if !macRe.MatchString(mac) {
		Error(w, http.StatusBadRequest, "MAC 地址格式无效")
		return
	}
	cidr := strings.TrimSpace(req.SubnetCIDR)
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		Error(w, http.StatusBadRequest, "子网 CIDR 格式无效")
		return
	}
	entry := &models.WhitelistEntry{
		MAC:        mac,
		SubnetCIDR: cidr,
		Source:     "未授权转白",
	}
	if err := h.store.CreateWhitelist(r.Context(), entry); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			Error(w, http.StatusConflict, "该 MAC 已在此子网的白名单中")
		} else {
			h.logger.Error("创建白名单条目失败", "mac", mac, "error", err)
			Error(w, http.StatusInternalServerError, "添加失败")
		}
		return
	}
	// 从未授权设备列表中移除
	if err := h.store.DeleteUnauthorizedDeviceByMAC(r.Context(), mac, cidr); err != nil {
		h.logger.Error("清除未授权设备记录失败", "error", err)
	}
	h.logger.Info("未授权设备已加白", "mac", mac, "cidr", cidr)
	OK(w, entry)
}

func (h *AccessHandler) AddToBlacklistFromUnauthorized(w http.ResponseWriter, r *http.Request) {
	var req addToWhitelistFromUnauthorizedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	mac := strings.TrimSpace(req.MAC)
	if !macRe.MatchString(mac) {
		Error(w, http.StatusBadRequest, "MAC 地址格式无效")
		return
	}
	entry := &models.BlacklistEntry{
		MAC:    mac,
		Reason: "从未授权设备添加",
		Source: "未授权转黑",
	}
	if err := h.store.CreateBlacklist(r.Context(), entry); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			Error(w, http.StatusConflict, "该 MAC 已存在于黑名单")
		} else {
			h.logger.Error("创建黑名单条目失败", "mac", mac, "error", err)
			Error(w, http.StatusInternalServerError, "添加失败")
		}
		return
	}
	// 从未授权设备列表中移除
	if err := h.store.DeleteUnauthorizedDeviceByMAC(r.Context(), mac, req.SubnetCIDR); err != nil {
		h.logger.Error("清除未授权设备记录失败", "error", err)
	}
	h.logger.Info("未授权设备已拉黑", "mac", mac)
	OK(w, entry)
}

func (h *AccessHandler) DeleteUnauthorizedDevice(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.store.DeleteUnauthorizedDevice(r.Context(), uint(id)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			Error(w, http.StatusNotFound, "条目未找到")
		} else {
			h.logger.Error("删除未授权设备记录失败", "id", id, "error", err)
			Error(w, http.StatusInternalServerError, "删除失败")
		}
		return
	}
	h.logger.Info("未授权设备记录已删除", "id", id)
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
	mac := strings.TrimSpace(req.MAC)
	if !macRe.MatchString(mac) {
		Error(w, http.StatusBadRequest, "MAC 地址格式无效，请使用 00:11:22:33:44:55 格式")
		return
	}
	if _, err := net.ParseMAC(mac); err != nil {
		Error(w, http.StatusBadRequest, "MAC 地址格式无效")
		return
	}
	cidr := strings.TrimSpace(req.SubnetCIDR)
	if cidr != "" {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			Error(w, http.StatusBadRequest, "子网 CIDR 格式无效")
			return
		}
	}
	entry := &models.WhitelistEntry{
		MAC:        mac,
		SubnetCIDR: cidr,
		Reason:     req.Reason,
		Source:     "手动添加",
	}
	if err := h.store.CreateWhitelist(r.Context(), entry); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			Error(w, http.StatusConflict, "该 MAC 已在此子网的白名单中")
		} else {
			h.logger.Error("创建白名单条目失败", "mac", mac, "error", err)
			Error(w, http.StatusInternalServerError, "创建失败")
		}
		return
	}
	h.logger.Info("白名单已添加", "mac", mac, "cidr", cidr)
	OK(w, entry)
}
