package api

import (
	"net/http"

	"github.com/pxego/pxego/internal/store"
)

type LeaseHandler struct {
	store store.Interface
}

func (h *LeaseHandler) List(w http.ResponseWriter, r *http.Request) {
	leases, err := h.store.ListLeases(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, leases)
}
