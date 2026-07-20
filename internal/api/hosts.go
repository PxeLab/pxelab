package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxelab/pxelab/internal/boot/configgen"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

type HostHandler struct {
	store  store.Interface
	config *config.Config
}

func (h *HostHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	search := r.URL.Query().Get("search")
	if page < 1 { page = 1 }
	if size < 1 || size > 100 { size = 20 }
	hosts, total, err := h.store.ListHosts(r.Context(), search, page, size)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"hosts": hosts, "meta": Meta{Page: page, Size: size, Total: total}})
}

func (h *HostHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host, err := h.store.GetHost(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "主机未找到")
		return
	}
	OK(w, host)
}

func (h *HostHandler) Create(w http.ResponseWriter, r *http.Request) {
	var host models.Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	host.ID = uuid.New().String()
	if err := h.store.CreateHost(r.Context(), &host); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "host", host.ID, remoteIP(r), host.Name)
	Created(w, host)
}

func (h *HostHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var host models.Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	host.ID = id
	if err := h.store.UpdateHost(r.Context(), &host); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "host", id, remoteIP(r), host.Name)
	OK(w, host)
}

func (h *HostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteHost(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "host", id, remoteIP(r), "")
	w.WriteHeader(http.StatusNoContent)
}
func (h *HostHandler) PreviewBootConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	format := configgen.Format(r.URL.Query().Get("format"))
	if format == "" {
		format = configgen.FormatPXELinux
	}
	if format != configgen.FormatPXELinux && format != configgen.FormatGRUB2 {
		Error(w, http.StatusBadRequest, "不支持的格式，支持: pxelinux, grub2")
		return
	}

	host, err := h.store.GetHost(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "主机未找到")
		return
	}

	if host.ProfileID == nil || *host.ProfileID == "" {
		Error(w, http.StatusNotFound, "主机未分配 Profile")
		return
	}

	profile, err := h.store.GetProfile(r.Context(), *host.ProfileID)
	if err != nil {
		Error(w, http.StatusNotFound, "Profile 未找到")
		return
	}

	menu, err := profile.GetMenu()
	if err != nil {
		Error(w, http.StatusInternalServerError, "读取菜单失败")
		return
	}

	if len(menu.Entries) == 0 {
		Error(w, http.StatusNotFound, "Profile 无引导条目")
		return
	}

	configStr, err := configgen.Generate(menu.Entries, format, r.Host, host.MAC)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(configStr))
}
