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

type ProxyServer4011 struct {
	name    string
	addr    string
	handler *Handler
	conn    *net.UDPConn
}

func NewProxyServer4011(cfg *config.Config, st store.Interface, bus *eventbus.Bus) (*ProxyServer4011, error) {
	handler := NewHandler(cfg, st, bus)
	addr := fmt.Sprintf("0.0.0.0:%d", config.DefaultPortDHCP4011)
	return &ProxyServer4011{
		name:    "ProxyDHCP-4011",
		addr:    addr,
		handler: handler,
	}, nil
}

func (s *ProxyServer4011) Name() string { return s.name }

func (s *ProxyServer4011) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("监听 :4011 失败: %w", err)
	}

	slog.Info("ProxyDHCP 服务启动", "addr", s.addr)

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
				slog.Error("ProxyDHCP 读取错误", "error", err)
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
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
