package tftp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/pin/tftp"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/models"
)

type Server struct {
	name        string
	port        int
	server      *tftp.Server
	bootFS      *boot.BootFileServer
	maxConns    int
	activeConns chan struct{}
	eventBus    *eventbus.Bus
}

func NewServer(port int, bootFS *boot.BootFileServer, bus *eventbus.Bus) *Server {
	s := &Server{
		name:        "TFTP",
		port:        port,
		bootFS:      bootFS,
		maxConns:    50,
		activeConns: make(chan struct{}, 50),
		eventBus:    bus,
	}

	readHandler := func(filename string, rf io.ReaderFrom) error {
		s.activeConns <- struct{}{}
		defer func() { <-s.activeConns }()

		slog.Debug("TFTP 读取请求", "file", filename)

		s.eventBus.Publish("event", models.Event{
			Type:    models.EventTFTP,
			Level:   models.EventInfo,
			Message: fmt.Sprintf("TFTP 读取: %s", filename),
		})

		data, err := s.bootFS.Read(filename)
		if err != nil {
			slog.Warn("TFTP 文件未找到", "file", filename)
			return fmt.Errorf("文件未找到: %s", filename)
		}

		_, err = rf.ReadFrom(io.NopCloser(bytes.NewReader(data)))
		return err
	}

	s.server = tftp.NewServer(readHandler, nil)
	s.server.SetTimeout(5 * time.Second)
	return s
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("TFTP 服务启动", "addr", addr)
	go func() {
		if err := s.server.ListenAndServe(addr); err != nil {
			slog.Error("TFTP 服务异常退出", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("TFTP 服务关闭")
	s.server.Shutdown()
	return nil
}
