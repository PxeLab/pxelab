package tftp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/pin/tftp/v3"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/models"
)

// optStrippingConn wraps a net.PacketConn and can optionally strip TFTP
// option extensions from RRQ packets to prevent OACK responses.
type optStrippingConn struct {
	net.PacketConn
	readCh   chan packet
	ctx      context.Context
	cancel   context.CancelFunc
	stripOpt bool
}

type packet struct {
	data []byte
	addr net.Addr
}

func newOptStrippingConn(inner net.PacketConn, stripOpt bool) *optStrippingConn {
	ctx, cancel := context.WithCancel(context.Background())
	c := &optStrippingConn{
		PacketConn: inner,
		readCh:     make(chan packet, 100),
		ctx:        ctx,
		cancel:     cancel,
		stripOpt:   stripOpt,
	}
	go c.readLoop()
	return c
}

func (c *optStrippingConn) ReadFrom(b []byte) (int, net.Addr, error) {
	select {
	case pkt := <-c.readCh:
		n := copy(b, pkt.data)
		return n, pkt.addr, nil
	case <-c.ctx.Done():
		return 0, nil, net.ErrClosed
	}
}

func (c *optStrippingConn) Close() error {
	c.cancel()
	return c.PacketConn.Close()
}

func (c *optStrippingConn) readLoop() {
	buf := make([]byte, 1500)
	for {
		n, addr, err := c.PacketConn.ReadFrom(buf)
		if err != nil {
			select {
			case <-c.ctx.Done():
			default:
				slog.Warn("TFTP 读取错误", "error", err)
			}
			return
		}
		if n < 2 {
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		if c.stripOpt && binary.BigEndian.Uint16(buf[:2]) == 1 { // opRRQ
			slog.Info("TFTP RRQ 原始", "len", n, "hex", fmt.Sprintf("%x", buf[:min(n, 80)]))
			data = stripOptions(data)
			slog.Info("TFTP RRQ 剥离后", "stripped_len", len(data))
		}

		select {
		case c.readCh <- packet{data: data, addr: addr}:
		case <-c.ctx.Done():
			return
		}
	}
}

func stripOptions(pkt []byte) []byte {
	pos := 2
	for pos < len(pkt) {
		if pkt[pos] == 0 {
			pos++
			break
		}
		pos++
	}
	for pos < len(pkt) {
		if pkt[pos] == 0 {
			pos++
			return pkt[:pos]
		}
		pos++
	}
	return pkt
}

type Server struct {
	name     string
	port     int
	bootFS   *boot.BootFileServer
	eventBus *eventbus.Bus
	conn     *optStrippingConn
	tftpSrv  *tftp.Server
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewServer(port int, bootFS *boot.BootFileServer, bus *eventbus.Bus) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		name:     "TFTP",
		port:     port,
		bootFS:   bootFS,
		eventBus: bus,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return fmt.Errorf("地址解析失败: %w", err)
	}
	innerConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("监听端口 %d 失败（需要管理员权限?）: %w", s.port, err)
	}

	// false = don't strip options — let pin/tftp send OACK
	// OACK will come from ephemeral port (non-single-port mode),
	// which UEFI firmware might accept
	s.conn = newOptStrippingConn(innerConn, false)

	s.tftpSrv = tftp.NewServer(s.readHandler, nil)

	slog.Info("TFTP 服务启动（非单端口模式，允许 OACK 协商）", "addr", addr)

	go func() {
		if err := s.tftpSrv.Serve(s.conn); err != nil {
			select {
			case <-s.ctx.Done():
			default:
				slog.Error("TFTP Serve 异常退出", "error", err)
			}
		}
	}()

	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("TFTP 服务关闭")
	s.cancel()
	if s.conn != nil {
		s.conn.Close()
	}
	return nil
}

func (s *Server) readHandler(filename string, rf io.ReaderFrom) error {
	slog.Info("TFTP 请求", "file", filename)

	s.eventBus.Publish("event", models.Event{
		Type:    models.EventTFTP,
		Level:   models.EventInfo,
		Message: fmt.Sprintf("TFTP 请求: %s", filename),
	})

	data, err := s.bootFS.Read(filename)
	if err != nil {
		slog.Warn("TFTP 文件未找到", "file", filename)
		return fmt.Errorf("file not found: %s", filename)
	}

	slog.Info("TFTP 开始发送", "file", filename, "size", len(data))

	if _, err := rf.ReadFrom(bytes.NewReader(data)); err != nil {
		slog.Warn("TFTP 传输失败", "file", filename, "error", err)
		return err
	}

	slog.Info("TFTP 传输完成", "file", filename, "size", len(data))
	return nil
}
