package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

type NetbootOverlayHandler struct {
	store store.Interface
}

// netbootOverlayDTO 是 API 层的请求/响应结构：version_overrides 以数组形式与前端交互，
// DB 中仍存为 JSON 字符串（models.NetbootOverlay.VersionOverrides，列不变）
type netbootOverlayDTO struct {
	ID               uint                     `json:"id"`
	DistroName       string                   `json:"distro_name"`
	Enabled          bool                     `json:"enabled"`
	Mirror           string                   `json:"mirror"`
	LocalBase        string                   `json:"local_base"`
	KernelParams     string                   `json:"kernel_params"`
	VersionOverrides []models.VersionOverride `json:"version_overrides"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
}

func overlayToDTO(o *models.NetbootOverlay) netbootOverlayDTO {
	ovs, err := o.GetVersionOverrides()
	if err != nil || ovs == nil {
		ovs = []models.VersionOverride{}
	}
	return netbootOverlayDTO{
		ID:               o.ID,
		DistroName:       o.DistroName,
		Enabled:          o.Enabled,
		Mirror:           o.Mirror,
		LocalBase:        o.LocalBase,
		KernelParams:     o.KernelParams,
		VersionOverrides: ovs,
		CreatedAt:        o.CreatedAt,
		UpdatedAt:        o.UpdatedAt,
	}
}

func (dto *netbootOverlayDTO) toModel() (*models.NetbootOverlay, error) {
	o := &models.NetbootOverlay{
		ID:           dto.ID,
		DistroName:   dto.DistroName,
		Enabled:      dto.Enabled,
		Mirror:       dto.Mirror,
		LocalBase:    dto.LocalBase,
		KernelParams: dto.KernelParams,
	}
	if len(dto.VersionOverrides) > 0 {
		if err := o.SetVersionOverrides(dto.VersionOverrides); err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (h *NetbootOverlayHandler) List(w http.ResponseWriter, r *http.Request) {
	overlays, err := h.store.ListNetbootOverlays(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]netbootOverlayDTO, 0, len(overlays))
	for i := range overlays {
		dtos = append(dtos, overlayToDTO(&overlays[i]))
	}
	OK(w, map[string]any{"overlays": dtos})
}

func (h *NetbootOverlayHandler) Get(w http.ResponseWriter, r *http.Request) {
	distro := chi.URLParam(r, "distro")
	overlay, err := h.store.GetNetbootOverlay(r.Context(), distro)
	if err != nil {
		Error(w, http.StatusNotFound, "覆盖配置未找到")
		return
	}
	OK(w, overlayToDTO(overlay))
}

func (h *NetbootOverlayHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	distro := chi.URLParam(r, "distro")
	var dto netbootOverlayDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	dto.DistroName = distro
	dto.ID = 0 // ID 由 store 按 distro_name 决定，忽略请求体中的值
	overlay, err := dto.toModel()
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 version_overrides")
		return
	}
	if err := h.store.UpsertNetbootOverlay(r.Context(), overlay); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "netboot_overlay", distro, remoteIP(r), "更新网络引导覆盖: "+distro)
	OK(w, overlayToDTO(overlay))
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
