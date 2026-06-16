package api

import (
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/ipmi"
	"github.com/pxego/pxego/internal/store"
)

type Handler struct {
	Host     *HostHandler
	Profile  *ProfileHandler
	Event    *EventHandler
	File     *FileHandler
	WOL      *WOLHandler
	IPMI     *IPMIHandler
	Lease    *LeaseHandler
	Settings *SettingsHandler
	Services map[string]string
}

func NewHandler(cfg *config.Config, st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer) *Handler {
	h := &Handler{
		Host:     &HostHandler{store: st},
		Profile:  &ProfileHandler{store: st},
		Event:    &EventHandler{store: st, eventBus: bus},
		File:     &FileHandler{bootFS: bootFS},
		WOL:      &WOLHandler{store: st},
		IPMI:     &IPMIHandler{store: st, ipmiClient: ipmi.NewClient()},
		Lease:    &LeaseHandler{store: st},
		Settings: NewSettingsHandler(cfg),
		Services: map[string]string{},
	}
	for i, iface := range cfg.Interfaces {
		name := iface.Name
		if name == "" {
			name = fmt.Sprintf("接口%d", i+1)
		}
		status := "enabled"
		if iface.DHCP == "off" {
			status = "disabled"
		}
		h.Services["DHCP/"+name] = status
		if iface.TFTP {
			h.Services["TFTP/"+name] = "enabled"
		}
		if iface.HTTP {
			h.Services["HTTP/"+name] = "enabled"
		}
		if iface.DNS {
			h.Services["DNS/"+name] = "enabled"
		}
	}
	return h
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/status", h.Status)

		r.Get("/events", h.Event.List)
		r.Get("/events/stream", h.Event.Stream)

		r.Get("/hosts", h.Host.List)
		r.Post("/hosts", h.Host.Create)
		r.Get("/hosts/{id}", h.Host.Get)
		r.Put("/hosts/{id}", h.Host.Update)
		r.Delete("/hosts/{id}", h.Host.Delete)
		r.Post("/hosts/{id}/wake", h.WOL.Wake)
		r.Post("/hosts/{id}/power", h.IPMI.PowerAction)

		r.Get("/profiles", h.Profile.List)
		r.Post("/profiles", h.Profile.Create)
		r.Get("/profiles/{id}", h.Profile.Get)
		r.Put("/profiles/{id}", h.Profile.Update)
		r.Delete("/profiles/{id}", h.Profile.Delete)

		r.Get("/files", h.File.List)
		r.Post("/files/upload", h.File.Upload)
		r.Delete("/files", h.File.Delete)

		r.Get("/leases", h.Lease.List)

		r.Get("/settings", h.Settings.Get)
		r.Put("/settings", h.Settings.Update)

		r.Get("/interfaces", h.ListInterfaces)
	})
}
