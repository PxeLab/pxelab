package tftp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"

	"github.com/pin/tftp/v3"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/models"
)

type Server struct {
	name     string
	port     int
	bootFS   *boot.BootFileServer
	eventBus *eventbus.Bus
	conn     *net.UDPConn
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
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("监听端口 %d 失败（需要管理员权限?）: %w", s.port, err)
	}
	s.conn = conn

	s.tftpSrv = tftp.NewServer(s.readHandler, nil)

	slog.Info("TFTP 服务启动", "addr", addr)

	go func() {
		if err := s.tftpSrv.Serve(conn); err != nil {
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
