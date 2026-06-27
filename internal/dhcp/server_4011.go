package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

type ProxyServer4011 struct {
	name    string
	addr    string
	handler *Handler
	conn    *net.UDPConn
}

func NewProxyServer4011(addr string, handler *Handler) *ProxyServer4011 {
	return &ProxyServer4011{
		name:    "ProxyDHCP-4011",
		addr:    addr,
		handler: handler,
	}
}

func (s *ProxyServer4011) Name() string { return s.name }

func (s *ProxyServer4011) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp4", s.addr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp4", udpAddr)
	if err != nil {
		return fmt.Errorf("监听 :4011 失败: %w", err)
	}

	slog.Info("ProxyDHCP 服务启动", "service", "DHCP", "addr", s.addr)

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
				slog.Error("ProxyDHCP 读取错误", "service", "DHCP", "error", err)
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

func (s *ProxyServer4011) Stop(ctx context.Context) error {
	slog.Info("ProxyDHCP 服务关闭", "service", "DHCP")
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
