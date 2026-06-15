package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/store"
)

type Handler struct {
	Host    *HostHandler
	Profile *ProfileHandler
	Event   *EventHandler
	File    *FileHandler
}

func NewHandler(st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer) *Handler {
	return &Handler{
		Host:    &HostHandler{store: st},
		Profile: &ProfileHandler{store: st},
		Event:   &EventHandler{store: st, eventBus: bus},
		File:    &FileHandler{bootFS: bootFS},
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/status", StatusHandler)

		r.Get("/events", h.Event.List)
		r.Get("/events/stream", h.Event.Stream)

		r.Get("/hosts", h.Host.List)
		r.Post("/hosts", h.Host.Create)
		r.Get("/hosts/{id}", h.Host.Get)
		r.Put("/hosts/{id}", h.Host.Update)
		r.Delete("/hosts/{id}", h.Host.Delete)

		r.Get("/profiles", h.Profile.List)
		r.Post("/profiles", h.Profile.Create)
		r.Get("/profiles/{id}", h.Profile.Get)
		r.Put("/profiles/{id}", h.Profile.Update)
		r.Delete("/profiles/{id}", h.Profile.Delete)

		r.Get("/files", h.File.List)
		r.Delete("/files", h.File.Delete)
	})
}
