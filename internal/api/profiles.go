package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

type ProfileHandler struct {
	store store.Interface
}

func (h *ProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.store.ListProfiles(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, profiles)
}

func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	profile, err := h.store.GetProfile(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "配置未找到")
		return
	}
	OK(w, profile)
}

func (h *ProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
	var profile models.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	profile.ID = uuid.New().String()
	if err := h.store.CreateProfile(r.Context(), &profile); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Created(w, profile)
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var profile models.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	profile.ID = id
	if err := h.store.UpdateProfile(r.Context(), &profile); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, profile)
}

func (h *ProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteProfile(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}


