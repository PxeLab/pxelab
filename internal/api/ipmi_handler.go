package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/ipmi"
	"github.com/pxego/pxego/internal/store"
)

type IPMIHandler struct {
	store      store.Interface
	ipmiClient *ipmi.Client
}

type powerRequest struct {
	Action string `json:"action"`
}

func (h *IPMIHandler) PowerAction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host, err := h.store.GetHost(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "主机未找到")
		return
	}

	var req powerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	switch req.Action {
	case "on":
		if err := h.ipmiClient.PowerOn(host); err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		OK(w, map[string]string{"status": "on"})
	case "off":
		if err := h.ipmiClient.PowerOff(host); err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		OK(w, map[string]string{"status": "off"})
	case "cycle":
		if err := h.ipmiClient.PowerCycle(host); err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		OK(w, map[string]string{"status": "cycle"})
	case "status":
		status, err := h.ipmiClient.PowerStatus(host)
		if err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		OK(w, map[string]string{"status": status})
	default:
		Error(w, http.StatusBadRequest, "无效的电源操作，可用值: on, off, cycle, status")
	}
}
