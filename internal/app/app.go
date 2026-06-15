package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

type App struct {
	servers []Server
	config  interface{}
	db      interface{}
	eventBus interface{}
}

func New() *App {
	return &App{}
}

func (a *App) Register(srv Server) {
	a.servers = append(a.servers, srv)
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	for _, srv := range a.servers {
		s := srv
		g.Go(func() error {
			slog.Info("启动服务", "name", s.Name())
			if err := s.Start(ctx); err != nil {
				return fmt.Errorf("%s: %w", s.Name(), err)
			}
			return nil
		})
	}

	// 信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		slog.Info("收到信号", "signal", sig)
	}

	// 反向顺序优雅关闭
	for i := len(a.servers) - 1; i >= 0; i-- {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		slog.Info("关闭服务", "name", a.servers[i].Name())
		a.servers[i].Stop(shutdownCtx)
		cancel()
	}

	return g.Wait()
}
