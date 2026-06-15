package httpd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/pxego/pxego/internal/api"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/store"
)

type Server struct {
	name   string
	cfg    *config.Config
	router chi.Router
	srv    *http.Server
	bootFS *boot.BootFileServer
	api    *api.Handler
}

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer) *Server {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(CORSMiddleware)
	if cfg.Auth.Token != "" {
		r.Use(AuthMiddleware(cfg.Auth.Token))
	}

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	apiHandler := api.NewHandler(st, bus, bootFS)
	apiHandler.RegisterRoutes(r)

	return &Server{
		name:   "HTTP",
		cfg:    cfg,
		router: r,
		bootFS: bootFS,
		api:    apiHandler,
	}
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", 8080)
	s.srv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	slog.Info("HTTP 服务启动", "addr", addr)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务异常退出", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("HTTP 服务关闭")
	return s.srv.Shutdown(ctx)
}
