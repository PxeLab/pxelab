package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

type Server struct {
	name    string
	addr    string
	handler *Handler
	conn    *net.UDPConn
}

func NewServer(addr string, handler *Handler) *Server {
	return &Server{
		name:    "DHCP",
		addr:    addr,
		handler: handler,
	}
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", s.addr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", s.addr, err)
	}

	slog.Info("DHCP 服务启动", "service", "DHCP", "addr", s.addr)

	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, peer, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil || isClosedConn(err) {
					return nil
				}
				slog.Error("DHCP 读取错误", "service", "DHCP", "error", err)
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

func isClosedConn(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "use of closed network connection")
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("DHCP 服务关闭", "service", "DHCP")
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
