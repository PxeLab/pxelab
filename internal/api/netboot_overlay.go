package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

type NetbootOverlayHandler struct {
	store store.Interface
}

func (h *NetbootOverlayHandler) List(w http.ResponseWriter, r *http.Request) {
	overlays, err := h.store.ListNetbootOverlays(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"overlays": overlays})
}

func (h *NetbootOverlayHandler) Get(w http.ResponseWriter, r *http.Request) {
	distro := chi.URLParam(r, "distro")
	overlay, err := h.store.GetNetbootOverlay(r.Context(), distro)
	if err != nil {
		Error(w, http.StatusNotFound, "覆盖配置未找到")
		return
	}
	OK(w, overlay)
}

func (h *NetbootOverlayHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	distro := chi.URLParam(r, "distro")
	var overlay models.NetbootOverlay
	if err := json.NewDecoder(r.Body).Decode(&overlay); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	overlay.DistroName = distro
	if err := h.store.UpsertNetbootOverlay(r.Context(), &overlay); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "netboot_overlay", distro, remoteIP(r), "更新网络引导覆盖: "+distro)
	OK(w, overlay)
}

func (h *NetbootOverlayHandler) Delete(w http.ResponseWriter, r *http.Request) {
	distro := chi.URLParam(r, "distro")
	if err := h.store.DeleteNetbootOverlay(r.Context(), distro); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "netboot_overlay", distro, remoteIP(r), "删除网络引导覆盖: "+distro)
	w.WriteHeader(http.StatusNoContent)
}
