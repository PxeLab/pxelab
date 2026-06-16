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

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer, spaHandler http.Handler) *Server {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(CORSMiddleware)
	if cfg.Auth.Token != "" {
		r.Use(AuthMiddleware(cfg.Auth.Token))
	}

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	apiHandler := api.NewHandler(cfg, st, bus, bootFS)
	apiHandler.RegisterRoutes(r)

	// 启动文件 HTTP 服务（iPXE 等通过网络引导）
	if bootFS != nil {
		r.Get("/boot/*", func(w http.ResponseWriter, r *http.Request) {
			filePath := chi.URLParam(r, "*")
			if filePath == "" {
				filePath = r.URL.Query().Get("path")
			}
			data, err := bootFS.Read(filePath)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(data)
		})
	}

	// 前端 SPA
	if spaHandler != nil {
		r.Handle("/*", spaHandler)
	}

	return &Server{
		name:   "HTTP",
		cfg:    cfg,
		router: r,
		bootFS: bootFS,
		api:    apiHandler,
	}
}

func (s *Server) Name() string { return s.name }

// API 返回 API Handler，用于外部设置服务状态等
func (s *Server) API() *api.Handler { return s.api }

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
