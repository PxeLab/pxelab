package dns

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/miekg/dns"
	"github.com/pxego/pxego/internal/eventbus"
)

type Server struct {
	name    string
	port    int
	handler *Handler
	dnsSrv  *dns.Server
}

func NewServer(port int, upstream string, bus *eventbus.Bus) *Server {
	return &Server{
		name:    "DNS",
		port:    port,
		handler: NewHandler(upstream, bus),
	}
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	mux := dns.NewServeMux()
	mux.Handle(".", s.handler)

	s.dnsSrv = &dns.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Net:     "udp",
		Handler: mux,
	}

	slog.Info("DNS 服务启动", "addr", s.dnsSrv.Addr)
	go func() {
		if err := s.dnsSrv.ListenAndServe(); err != nil {
			slog.Error("DNS 服务异常退出", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("DNS 服务关闭")
	return s.dnsSrv.Shutdown()
}
