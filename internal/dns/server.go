package dns

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/miekg/dns"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/store"
)

type Server struct {
	name    string
	port    int
	handler *Handler
	dnsSrv  *dns.Server
	cfg     *config.Config
	store   store.Interface
	bus     *eventbus.Bus
}

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus) *Server {
	return &Server{
		name:    "DNS",
		port:    cfg.DNS.Port,
		handler: NewHandler(cfg, st, bus),
		cfg:     cfg,
		store:   st,
		bus:     bus,
	}
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	// Re-create handler with fresh config so upstream changes take effect
	s.handler = NewHandler(s.cfg, s.store, s.bus)
	mux := dns.NewServeMux()
	mux.Handle(".", s.handler)

	s.dnsSrv = &dns.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Net:     "udp",
		Handler: mux,
	}

	slog.Info("DNS 服务启动", "service", "DNS", "addr", s.dnsSrv.Addr)
	go func() {
		if err := s.dnsSrv.ListenAndServe(); err != nil {
			slog.Error("DNS 服务异常退出", "service", "DNS", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("DNS 服务关闭", "service", "DNS")
	return s.dnsSrv.Shutdown()
}
