package api

import (
	"net/http"

	"github.com/pxelab/pxelab/internal/netboot"
)

type NetbootHandler struct {
	manager *netboot.Manager
}

func NewNetbootHandler(manager *netboot.Manager) *NetbootHandler {
	return &NetbootHandler{manager: manager}
}

func (h *NetbootHandler) GetCatalog(w http.ResponseWriter, r *http.Request) {
	OK(w, h.manager.Catalog())
}

func (h *NetbootHandler) GetDistro(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("distro")
	if name == "" {
		name = r.URL.Query().Get("name")
	}
	d := h.manager.GetDistro(name)
	if d == nil {
		Error(w, http.StatusNotFound, "distro not found")
		return
	}
	OK(w, d)
}

func (h *NetbootHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	OK(w, h.manager.Groups())
}

func (h *NetbootHandler) CheckFiles(w http.ResponseWriter, r *http.Request) {
	statuses := make([]FileStatus, 0)
	for _, d := range h.manager.Catalog().Distros {
		for _, v := range d.Versions {
			if !v.Enabled {
				continue
			}
			status := FileStatus{
				Distro:   d.Name,
				Version:  v.Name,
				Arch:     v.Arch,
				HasLocal: v.Local != nil,
			}
			statuses = append(statuses, status)
		}
	}
	OK(w, statuses)
}

type FileStatus struct {
	Distro   string `json:"distro"`
	Version  string `json:"version"`
	Arch     string `json:"arch"`
	HasLocal bool   `json:"has_local"`
}
