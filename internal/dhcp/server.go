package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/store"
)

type Server struct {
	name    string
	addr    string
	handler *Handler
	conn    *net.UDPConn
}

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus) (*Server, error) {
	handler := NewHandler(cfg, st, bus)
	handler.InitSubnets()

	addr := fmt.Sprintf("0.0.0.0:%d", config.DefaultPortDHCP)
	return &Server{
		name:    "DHCP",
		addr:    addr,
		handler: handler,
	}, nil
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", s.addr, err)
	}

	slog.Info("DHCP 服务启动", "addr", s.addr)

	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, peer, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				slog.Error("DHCP 读取错误", "error", err)
				continue
			}
			pkt, err := dhcpv4.FromBytes(buf[:n])
			if err != nil {
				continue
			}
			s.handler.Handle(ctx, s.conn, peer, pkt)
		}
	}
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("DHCP 服务关闭")
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
