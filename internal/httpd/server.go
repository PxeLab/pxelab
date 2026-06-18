package httpd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/pxego/pxego/internal/api"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/boot/ipxe"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/netboot"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/store"
)

type Server struct {
	name   string
	cfg    *config.Config
	router chi.Router
	srv    *http.Server
	bootFS *boot.BootFileServer
	api         *api.Handler
	netbootMgr  *netboot.Manager
}

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer, spaHandler http.Handler, reloader api.SubnetReloader, netbootMgr *netboot.Manager) *Server {
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

	apiHandler := api.NewHandler(cfg, st, bus, bootFS, reloader, netbootMgr)
	apiHandler.RegisterRoutes(r)

	// iPXE 引导脚本端点
	r.Get("/boot/ipxe/script", func(w http.ResponseWriter, r *http.Request) {
		mac := r.URL.Query().Get("mac")
		engine := ipxe.New()
		menuData := &ipxe.MenuData{
			Title:   "PxeGo Boot Menu",
			Timeout: 5000,
			Default: 0,
			Entries: buildDefaultEntries(cfg.Netboot.Enabled),
		}
		if mac != "" {
			host, err := st.GetHostByMAC(r.Context(), mac)
			if err == nil && host != nil && host.ProfileID != nil {
				profile, err := st.GetProfile(r.Context(), *host.ProfileID)
				if err == nil && profile != nil {
					bootMenu, err := profile.GetMenu()
					if err == nil && len(bootMenu.Entries) > 0 {
						menuData.Title = profile.Name
						var entries []ipxe.MenuEntryData
						for _, e := range bootMenu.Entries {
							entry := ipxe.MenuEntryData{
								Label: e.Label,
								Type:  ipxe.BootType(e.Type),
							}
							if e.Kernel != nil {
								entry.Kernel = "http://" + r.Host + "/boot/" + *e.Kernel
							}
							if e.Initrd != nil {
								entry.Initrd = "http://" + r.Host + "/boot/" + *e.Initrd
							}
							if e.Cmdline != nil {
								entry.Cmdline = *e.Cmdline
							}
							if e.URL != nil {
								entry.URL = *e.URL
							}
							if e.WIM != nil {
								entry.WIM = *e.WIM
							}
							entries = append(entries, entry)
						}
						entries = append(entries, ipxe.MenuEntryData{Label: "Boot from local disk", Type: ipxe.BootLocal})
						if cfg.Netboot.Enabled {
							entries = append(entries, ipxe.MenuEntryData{Label: "[OS] Netboot OS Install Catalog", Type: ipxe.BootNetboot})
						}
						menuData.Entries = entries
					}
				}
			}
		}
		script, err := engine.Render("menu", ipxe.TemplateData{
			MAC:  mac,
			Menu: menuData,
			URL:  "http://" + r.Host,
		})
		if err != nil {
			slog.Error("生成 iPXE 脚本失败", "error", err)
			http.Error(w, "script error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(script)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(script))
	})
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

	// Netboot OS catalog menu
	if netbootMgr != nil && cfg.Netboot.Enabled {
		r.Get("/netboot/menu.ipxe", func(w http.ResponseWriter, r *http.Request) {
			serverAddr := r.Host
			script := netboot.GenerateNetbootScript(netbootMgr.Catalog(), serverAddr)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Length", strconv.Itoa(len(script)))
			w.Write([]byte(script))
		})
	}

	// 前端 SPA
	if spaHandler != nil {
		r.Handle("/*", spaHandler)
	}

	return &Server{
		name:        "HTTP",
		cfg:         cfg,
		router:      r,
		bootFS:      bootFS,
		api:         apiHandler,
		netbootMgr:  netbootMgr,
	}
}

func buildDefaultEntries(netbootEnabled bool) []ipxe.MenuEntryData {
	entries := []ipxe.MenuEntryData{
		{Label: "Boot from local disk", Type: ipxe.BootLocal},
	}
	if netbootEnabled {
		entries = append(entries, ipxe.MenuEntryData{Label: "[OS] 网络安装操作系统目录", Type: ipxe.BootNetboot})
	}
	return entries
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
