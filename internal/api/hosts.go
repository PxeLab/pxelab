package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

type HostHandler struct {
	store store.Interface
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
	OK(w, host)
}

func (h *HostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteHost(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
