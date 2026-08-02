package dns

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/miekg/dns"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/store"
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

	addr := fmt.Sprintf(":%d", s.port)
	// 同步绑定 UDP 端口：端口被占用等错误立即返回，避免"假成功"（服务显示运行但实际没监听）
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		return fmt.Errorf("监听 UDP %s 失败: %w", addr, err)
	}

	s.dnsSrv = &dns.Server{
		PacketConn: pc,
		Handler:    mux,
	}

	slog.Info("DNS 服务启动", "service", "DNS", "addr", addr)
	go func() {
		if err := s.dnsSrv.ActivateAndServe(); err != nil {
			slog.Error("DNS 服务异常退出", "service", "DNS", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("DNS 服务关闭", "service", "DNS")
	if s.dnsSrv == nil {
		return nil
	}
	return s.dnsSrv.Shutdown()
}
