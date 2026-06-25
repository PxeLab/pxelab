package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/servicemanager"
)

type ServiceHandler struct {
	ctrl   ServiceController
	cfg    *config.Config
	saveFn func() error
}

func NewServiceHandler(ctrl ServiceController, cfg *config.Config, saveFn func() error) *ServiceHandler {
	return &ServiceHandler{ctrl: ctrl, cfg: cfg, saveFn: saveFn}
}

// ListServices 返回所有服务状态
func (h *ServiceHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	if h.ctrl == nil {
		OK(w, []servicemanager.ServiceInfo{})
		return
	}
	OK(w, h.ctrl.List())
}

func (h *ServiceHandler) rejectProtected(name string) bool {
	if h.ctrl == nil {
		return true
	}
	svc, ok := h.ctrl.Get(name)
	return ok && svc.Protected
}

// StartService 启动单个服务
func (h *ServiceHandler) StartService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		Error(w, http.StatusBadRequest, "missing service name")
		return
	}
	if h.ctrl == nil {
		Error(w, http.StatusBadRequest, "service manager not available")
		return
	}
	if h.rejectProtected(name) {
		Error(w, http.StatusForbidden, "service is protected and cannot be managed")
		return
	}
	if err := h.ctrl.Start(name); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	svc, _ := h.ctrl.Get(name)
	OK(w, svc)
}

// StopService 停止单个服务
func (h *ServiceHandler) StopService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		Error(w, http.StatusBadRequest, "missing service name")
		return
	}
	if h.ctrl == nil {
		Error(w, http.StatusBadRequest, "service manager not available")
		return
	}
	if h.rejectProtected(name) {
		Error(w, http.StatusForbidden, "service is protected and cannot be managed")
		return
	}
	if err := h.ctrl.Stop(name); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	svc, _ := h.ctrl.Get(name)
	OK(w, svc)
}

// RestartService 重启单个服务
func (h *ServiceHandler) RestartService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		Error(w, http.StatusBadRequest, "missing service name")
		return
	}
	if h.ctrl == nil {
		Error(w, http.StatusBadRequest, "service manager not available")
		return
	}
	if h.rejectProtected(name) {
		Error(w, http.StatusForbidden, "service is protected and cannot be managed")
		return
	}
	if err := h.ctrl.Restart(name); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	svc, _ := h.ctrl.Get(name)
	OK(w, svc)
}

type batchRequest struct {
	Names []string `json:"names"`
}

func (h *ServiceHandler) BatchOperation(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")

	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.Names) == 0 {
		Error(w, http.StatusBadRequest, "empty names list")
		return
	}
	if h.ctrl == nil {
		Error(w, http.StatusBadRequest, "service manager not available")
		return
	}

	// Filter out protected services from batch
	var filteredNames []string
	for _, name := range req.Names {
		if !h.rejectProtected(name) {
			filteredNames = append(filteredNames, name)
		}
	}
	if len(filteredNames) == 0 {
		Error(w, http.StatusBadRequest, "all requested services are protected")
		return
	}

	var result servicemanager.BatchResult
	switch action {
	case "start":
		result = h.ctrl.BatchStart(filteredNames)
	case "stop":
		result = h.ctrl.BatchStop(filteredNames)
	case "restart":
		result = h.ctrl.BatchRestart(filteredNames)
	default:
		Error(w, http.StatusBadRequest, "invalid action: use start/stop/restart")
		return
	}

	failed := false
	for _, err := range result {
		if err != "" {
			failed = true
			break
		}
	}

	OK(w, map[string]any{
		"success": !failed,
		"result":  result,
	})
}

type autoStartRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateAutoStart 切换服务的自动启动标志
func (h *ServiceHandler) UpdateAutoStart(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		Error(w, http.StatusBadRequest, "missing service name")
		return
	}
	if h.ctrl == nil {
		Error(w, http.StatusBadRequest, "service manager not available")
		return
	}

	var req autoStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.ctrl.SetAutoStart(name, req.Enabled); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 持久化到配置文件
	if h.cfg != nil && h.saveFn != nil {
		h.persistAutoStart(name, req.Enabled)
		if err := h.saveFn(); err != nil {
			// 仅日志记录，不阻塞响应
		}
	}

	svc, _ := h.ctrl.Get(name)
	OK(w, svc)
}

func (h *ServiceHandler) persistAutoStart(name string, enabled bool) {
	if h.cfg == nil {
		return
	}
	// DHCP/ProxyDHCP 服务: dhcp/eth0, proxy/eth0
	if name == "tftp" {
		h.cfg.ServiceAutoStart.TFTP = enabled
	} else if name == "http" {
		h.cfg.ServiceAutoStart.HTTP = enabled
	} else if name == "dns" {
		h.cfg.ServiceAutoStart.DNS = enabled
	} else {
		// Interface-level services: dhcp/{name}, proxy/{name}
		for i := range h.cfg.Interfaces {
			if "dhcp/"+h.cfg.Interfaces[i].Name == name || "proxy/"+h.cfg.Interfaces[i].Name == name {
				h.cfg.Interfaces[i].AutoStart = enabled
				return
			}
		}
	}
}
