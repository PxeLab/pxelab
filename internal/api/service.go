package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/servicemanager"
)

type ServiceHandler struct {
	ctrl ServiceController
}

func NewServiceHandler(ctrl ServiceController) *ServiceHandler {
	return &ServiceHandler{ctrl: ctrl}
}

// ListServices 返回所有服务状态
func (h *ServiceHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	if h.ctrl == nil {
		OK(w, []servicemanager.ServiceInfo{})
		return
	}
	OK(w, h.ctrl.List())
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

	var result servicemanager.BatchResult
	switch action {
	case "start":
		result = h.ctrl.BatchStart(req.Names)
	case "stop":
		result = h.ctrl.BatchStop(req.Names)
	case "restart":
		result = h.ctrl.BatchRestart(req.Names)
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
