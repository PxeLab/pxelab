package httpd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/pxego/pxego/internal/api"
	"github.com/pxego/pxego/internal/boot"
	"github.com/pxego/pxego/internal/boot/ipxe"
	"github.com/pxego/pxego/internal/config"
	"github.com/pxego/pxego/internal/eventbus"
	"github.com/pxego/pxego/internal/netboot"
	"github.com/pxego/pxego/internal/session"
	"github.com/pxego/pxego/internal/store"
)

type Server struct {
	name       string
	cfg        *config.Config
	router     chi.Router
	srv        *http.Server
	bootFS     *boot.BootFileServer
	api        *api.Handler
	netbootMgr *netboot.Manager
	clientInfo func(ip string) (arch, platform string, ok bool)
	sessions   *session.Store
}

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer, spaHandler http.Handler, reloader api.SubnetReloader, netbootMgr *netboot.Manager, clientInfo func(ip string) (arch, platform string, ok bool), svcController api.ServiceController, sessions *session.Store) *Server {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(CORSMiddleware)
	r.Use(AuthMiddleware(cfg, sessions))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

		apiHandler := api.NewHandler(cfg, st, bus, bootFS, reloader, netbootMgr, svcController, sessions)
	apiHandler.RegisterRoutes(r)

	// iPXE 引导脚本端点（配置驱动决策树）
		r.Get("/boot/ipxe/script", func(w http.ResponseWriter, r *http.Request) {
			mac := r.URL.Query().Get("mac")
			script, err := generateIPXEScript(cfg, st, mac, r.Host, r.Context())
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

		// 启动文件 HTTP 服务 — 带 chain_to_ipxe 配置文件拦截
	if bootFS != nil {
		ci := clientInfo // capture for closure
		cfgLocal := cfg  // capture for closure
		r.Get("/boot/*", func(w http.ResponseWriter, r *http.Request) {
			filePath := chi.URLParam(r, "*")
			if filePath == "" {
				filePath = r.URL.Query().Get("path")
			}

			// chain_to_ipxe：拦截 PXELinux/GRUB2 配置文件，返回 chainload 配置
			if ci != nil && (filePath == "pxelinux.cfg/default" || filePath == "grub.cfg") {
				clientIP := r.RemoteAddr
				if host, _, err := net.SplitHostPort(clientIP); err == nil {
					clientIP = host
				}
				arch, platform, ok := ci(clientIP)
				if !ok {
					arch, platform = "x86", "pc"
				}

				if chainToIPXEFallback(cfgLocal, filePath) {
					var config string
					if filePath == "grub.cfg" {
						config = boot.GRUB2ChainloadConfig(r.Host)
					} else {
						config = boot.PXELinuxChainloadConfig(r.Host, arch, platform)
					}
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.Header().Set("Content-Length", strconv.Itoa(len(config)))
					w.Write([]byte(config))
					return
				}
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

	// Netboot OS catalog menu + HTTPS proxy for boot files
	if netbootMgr != nil && cfg.Netboot.Enabled {
		r.Get("/netboot/menu.ipxe", func(w http.ResponseWriter, r *http.Request) {
			serverAddr := r.Host
			arch := r.URL.Query().Get("arch")
			platform := r.URL.Query().Get("platform")
			script := netboot.GenerateNetbootScript(netbootMgr.Catalog(), serverAddr, arch, platform,
				cfg.Netboot.Boot.CatalogDisplay.Title,
				cfg.Netboot.Boot.CatalogDisplay.Groups)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Content-Length", strconv.Itoa(len(script)))
			w.Write([]byte(script))
		})

		// Proxy GitHub HTTPS assets to HTTP for iPXE (which lacks HTTPS support)
		r.Get("/boot/netboot/proxy/*", func(w http.ResponseWriter, r *http.Request) {
			proxyPath := chi.URLParam(r, "*")
			var targetURL string
			switch {
			case strings.HasPrefix(proxyPath, "https/"):
				targetURL = "https://" + strings.TrimPrefix(proxyPath, "https/")
			case strings.HasPrefix(proxyPath, "http/"):
				targetURL = "http://" + strings.TrimPrefix(proxyPath, "http/")
			default:
				http.Error(w, "unsupported proxy scheme", http.StatusBadRequest)
				return
			}
			resp, err := http.Get(targetURL)
			if err != nil {
				slog.Error("netboot proxy fetch failed", "url", targetURL, "error", err)
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
			if resp.ContentLength > 0 {
				w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
			}
			w.WriteHeader(resp.StatusCode)
			if _, err := io.Copy(w, resp.Body); err != nil {
				slog.Error("netboot proxy copy error", "error", err)
			}
		})
	}

	// 前端 SPA
	if spaHandler != nil {
		r.Handle("/*", spaHandler)
	}

	return &Server{
		name:       "HTTP",
		cfg:        cfg,
		router:     r,
		bootFS:     bootFS,
		api:        apiHandler,
		netbootMgr: netbootMgr,
		clientInfo: clientInfo,
		sessions:   sessions,
	}
}

func (s *Server) Name() string { return s.name }

// API 返回 API Handler，用于外部设置服务状态等
func (s *Server) API() *api.Handler { return s.api }

// generateIPXEScript 根据配置生成 iPXE 引导脚本（配置驱动决策树）
func generateIPXEScript(cfg *config.Config, st store.Interface, mac, serverAddr string, ctx context.Context) (string, error) {
	// 1. 自定义脚本逃生口
	if tmpl := cfg.Netboot.ScriptTemplate; tmpl != "" {
		return renderTemplate(tmpl, serverAddr, mac)
	}

	engine := ipxe.New()

	// 2. 主机 Profile 菜单
	if mac != "" {
		host, err := st.GetHostByMAC(ctx, mac)
		if err == nil && host != nil && host.ProfileID != nil {
			profile, err := st.GetProfile(ctx, *host.ProfileID)
			if err == nil && profile != nil {
				bootMenu, err := profile.GetMenu()
				if err == nil && len(bootMenu.Entries) > 0 {
					to := cfg.Netboot.Boot.DefaultMenu.Timeout
					pm := &ipxe.MenuData{Title: profile.Name, Timeout: to, Default: 0, TimeoutDS: to * 1000}
					var entries []ipxe.MenuEntryData
					for _, e := range bootMenu.Entries {
						entry := ipxe.MenuEntryData{
							Label: e.Label,
							Type:  ipxe.BootType(e.Type),
							Kernel:  urlJoin(serverAddr, e.Kernel),
							Initrd:  urlJoin(serverAddr, e.Initrd),
							Cmdline: ptrStr(e.Cmdline),
							URL:     ptrStr(e.URL),
							WIM:     ptrStr(e.WIM),
						}
						entries = append(entries, entry)
					}

					pb := cfg.Netboot.Boot.ProfileBehavior
					if pb.AppendLocal {
						localEntry := ipxe.MenuEntryData{Label: "Boot from local disk", Type: ipxe.BootLocal}
						if pb.AppendPosition == "first" {
							entries = append([]ipxe.MenuEntryData{localEntry}, entries...)
						} else {
							entries = append(entries, localEntry)
						}
					}
					if pb.AppendNetboot && cfg.Netboot.Enabled {
						netbootEntry := ipxe.MenuEntryData{Label: "[OS] Netboot OS Install Catalog", Type: ipxe.BootNetboot}
						entries = append(entries, netbootEntry)
					}

					pm.Entries = entries
					return engine.Render("menu", ipxe.TemplateData{
						MAC:  mac,
						Menu: pm,
						URL:  "http://" + serverAddr,
					})
				}
			}
		}
	}

	// 3. 安装目录跳转
	cr := cfg.Netboot.Boot.CatalogRedirect
	if cr.Enabled && cfg.Netboot.Enabled {
		var b strings.Builder
		b.WriteString("#!ipxe\n")
		if cr.DetectArch {
			b.WriteString("cpuid --ext 29 && set arch x86_64 || set arch x86\n")
			b.WriteString("iseq ${buildarch} arm64 && set arch arm64 ||\n")
			b.WriteString("iseq ${buildarch} armhf && set arch armhf ||\n")
			b.WriteString("platform --is efi && set platform efi || set platform pc\n")
		}
		if cr.Preamble != "" {
			b.WriteString(cr.Preamble)
			if !strings.HasSuffix(cr.Preamble, "\n") {
				b.WriteString("\n")
			}
		}
		targetURL := strings.ReplaceAll(cr.TargetURL, "{{.URL}}", "http://"+serverAddr)
		b.WriteString(fmt.Sprintf("chain %s\n", targetURL))
		return b.String(), nil
	}

	// 4. 默认引导菜单
	dm := cfg.Netboot.Boot.DefaultMenu
	if len(dm.Entries) == 0 {
		// 后向兼容：最小菜单（保留用户设定的超时时间）
		dm.Title = "PxeGo Boot Menu"
		dm.Default = 0
		dm.Entries = []config.MenuEntry{{Label: "Boot from local disk", Type: "local"}}
	}

	var ipxeEntries []ipxe.MenuEntryData
	for _, e := range dm.Entries {
		entry := ipxe.MenuEntryData{
			Label:   e.Label,
			Type:    ipxe.BootType(e.Type),
			Kernel:  urlJoin(serverAddr, e.Kernel),
			Initrd:  urlJoin(serverAddr, e.Initrd),
			Cmdline: ptrStr(e.Cmdline),
			URL:     ptrStr(e.URL),
			WIM:     ptrStr(e.WIM),
		}
		ipxeEntries = append(ipxeEntries, entry)
	}

	return engine.Render("menu", ipxe.TemplateData{
		MAC: mac,
		Menu: &ipxe.MenuData{
			Title:     dm.Title,
			Timeout:   dm.Timeout,
			TimeoutDS: dm.Timeout * 1000,
			Default:   dm.Default,
			Entries:   ipxeEntries,
		},
		URL: "http://" + serverAddr,
	})
}

func renderTemplate(tmpl, serverAddr, mac string) (string, error) {
	tplData := struct {
		URL string
		MAC string
	}{
		URL: "http://" + serverAddr,
		MAC: mac,
	}
	var buf bytes.Buffer
	t, err := template.New("ipxe").Parse(tmpl)
	if err != nil {
		return "", err
	}
	if err := t.Execute(&buf, tplData); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func urlJoin(serverAddr string, ptr *string) string {
	if ptr == nil || *ptr == "" {
		return ""
	}
	return "http://" + serverAddr + "/boot/" + *ptr
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// chainToIPXEFallback 检查是否应该为该客户端返回 chain-load 配置
func chainToIPXEFallback(cfg *config.Config, filePath string) bool {
	for _, iface := range cfg.Interfaces {
		if !iface.ChainToIPXE {
			continue
		}
		switch filePath {
		case "grub.cfg":
			if iface.Bootloader == "grub2" {
				return true
			}
		case "pxelinux.cfg/default":
			if iface.Bootloader == "pxelinux" {
				return true
			}
		}
	}
	return false
}

func (s *Server) Start(ctx context.Context) error {
	addr := s.cfg.Global.ListenAddr
	if addr == "" {
		addr = ":8080"
	}
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
