package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/store"
)

type Handler struct{}

func NewHandler(st store.Interface, bus *eventbus.Bus) *Handler {
	return &Handler{}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	// TODO: 在 Task 16 中实现
}
