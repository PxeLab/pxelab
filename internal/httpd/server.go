package httpd

import (
	"bytes"
	"context"
	"embed"
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
	"github.com/pxego/pxego/internal/models"
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
	r.Use(slogMiddleware)
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
	// 当 failsafe 启用时，返回 autoexec 包装层，通过 chain 跳转到 /boot/ipxe/menu
	r.Get("/boot/ipxe/script", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Netboot.FailsafePrompt {
			tplData := struct{ Server string }{Server: r.Host}
			slog.Debug("autoexec chain URL", "service", "HTTP",
				"chain_url", "http://"+r.Host+"/boot/ipxe/menu?mac=${net0/mac}")
			var buf bytes.Buffer
			t, err := template.New("autoexec.ipxe").ParseFS(autoexecIPXEFS, "autoexec.ipxe")
			if err != nil {
				slog.Error("解析 autoexec 模板失败", "service", "HTTP", "error", err)
				http.Error(w, "template error", http.StatusInternalServerError)
				return
			}
			if err := t.Execute(&buf, tplData); err != nil {
				slog.Error("执行 autoexec 模板失败", "service", "HTTP", "error", err)
				http.Error(w, "template error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write(buf.Bytes())
			return
		}
		// failsafe 未启用——直接返回引导菜单
		mac := r.URL.Query().Get("mac")
		script, err := generateBootMenu(cfg, st, mac, r.Host, r.Context())
		if err != nil {
			slog.Error("生成 iPXE 菜单失败", "service", "HTTP", "error", err)
			http.Error(w, "script error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(script)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(script))
	})

	// iPXE 引导菜单端点（不含 failsafe，供 autoexec chain 调用）
	r.Get("/boot/ipxe/menu", func(w http.ResponseWriter, r *http.Request) {
		mac := r.URL.Query().Get("mac")
		script, err := generateBootMenu(cfg, st, mac, r.Host, r.Context())
		if err != nil {
			slog.Error("生成 iPXE 菜单失败", "service", "HTTP", "error", err)
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
				cfg.Netboot.Boot.CatalogDisplay.Groups, nil)
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
				slog.Error("netboot proxy fetch failed", "service", "HTTP", "url", targetURL, "error", err)
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
				slog.Error("netboot proxy copy error", "service", "HTTP", "error", err)
			}
		})
	}

		// Failsafe 菜单端点 — 提供自包含的故障恢复 iPXE 菜单
		if cfg.Netboot.FailsafePrompt {
			r.Get("/boot/ipxe/failsafe", func(w http.ResponseWriter, r *http.Request) {
				mac := r.URL.Query().Get("mac")
				script := generateFailsafeScript(r.Host, mac)
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

// generateBootMenu 根据配置生成 iPXE 引导菜单（不含 failsafe 包装）
func generateBootMenu(cfg *config.Config, st store.Interface, mac, serverAddr string, ctx context.Context) (string, error) {
	// 1. 自定义脚本逃生口
	if tmpl := cfg.Netboot.ScriptTemplate; tmpl != "" {
		return renderTemplate(tmpl, serverAddr, mac)
	}

	engine := ipxe.New()
	var script string

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
							Kernel:  replaceBootVars(urlJoin(serverAddr, e.Kernel), serverAddr, mac),
							Initrd:  replaceBootVars(urlJoin(serverAddr, e.Initrd), serverAddr, mac),
							Cmdline: replaceBootVars(ptrStr(e.Cmdline), serverAddr, mac),
							URL:     replaceBootVars(ptrStr(e.URL), serverAddr, mac),
							WIM:     ptrStr(e.WIM),
						}
						entries = append(entries, entry)
					}


					if cfg.Netboot.FailsafePrompt {
						entries = appendFailsafeEntry(entries, serverAddr, mac)
					}
					pm.Entries = entries
					script, err = engine.Render("menu", ipxe.TemplateData{
						MAC:  mac,
						Menu: pm,
						URL:  "http://" + serverAddr,
					})
					if err != nil {
						return "", err
					}
					return script, nil
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
		fmt.Fprintf(&b, "chain %s\n", targetURL)
		return b.String(), nil
	}

	// 4. 默认引导菜单
	var ipxeEntries []ipxe.MenuEntryData
	menuTitle := "PxeGo Boot Menu"

	if defProfile, err := st.GetDefaultProfile(ctx); err == nil && defProfile != nil {
		if bootMenu, err := defProfile.GetMenu(); err == nil && len(bootMenu.Entries) > 0 {
			entry := bootMenu.Entries[0]
			makeEntry := func(label string, e models.MenuEntry) ipxe.MenuEntryData {
				return ipxe.MenuEntryData{
					Label:   label,
					Type:    ipxe.BootType(e.Type),
					Kernel:  replaceBootVars(urlJoin(serverAddr, e.Kernel), serverAddr, mac),
					Initrd:  replaceBootVars(urlJoin(serverAddr, e.Initrd), serverAddr, mac),
					Cmdline: replaceBootVars(ptrStr(e.Cmdline), serverAddr, mac),
					URL:     replaceBootVars(ptrStr(e.URL), serverAddr, mac),
					WIM:     ptrStr(e.WIM),
				}
			}

			if cfg.Netboot.Boot.DefaultMenu.ListAllProfiles {
				// 全部展示模式：列出所有 Profile，默认 Profile 排第一
				menuTitle = "PxeGo Boot Menu"
				ipxeEntries = append(ipxeEntries, makeEntry(defProfile.Name, entry))
				if allProfiles, err := st.ListProfiles(ctx); err == nil {
					for _, p := range allProfiles {
						if p.ID == defProfile.ID {
							continue
						}
						if pm, err := p.GetMenu(); err == nil && len(pm.Entries) > 0 {
							ipxeEntries = append(ipxeEntries, makeEntry(p.Name, pm.Entries[0]))
						}
					}
				}
			} else {
				// 仅默认模式：只显示默认 Profile 的引导项
				menuTitle = defProfile.Name
				ipxeEntries = append(ipxeEntries, makeEntry(defProfile.Name, entry))
			}
		}
	}

	// 无可用的默认 Profile 时硬编码后备
	if len(ipxeEntries) == 0 {
		ipxeEntries = append(ipxeEntries, ipxe.MenuEntryData{Label: "Boot from local disk", Type: ipxe.BootLocal})
	}

	if cfg.Netboot.FailsafePrompt {
		ipxeEntries = appendFailsafeEntry(ipxeEntries, serverAddr, mac)
	}

	dm := cfg.Netboot.Boot.DefaultMenu
	script, err := engine.Render("menu", ipxe.TemplateData{
		MAC: mac,
		Menu: &ipxe.MenuData{
			Title:     menuTitle,
			Timeout:   dm.Timeout,
			TimeoutDS: dm.Timeout * 1000,
			Default:   0,
			Entries:   ipxeEntries,
		},
		URL: "http://" + serverAddr,
	})
	if err != nil {
		return "", err
	}
	return script, nil
}

//go:embed autoexec.ipxe
var autoexecIPXEFS embed.FS

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
	// absolute URL (http/https) passes through unchanged
	if strings.HasPrefix(*ptr, "http://") || strings.HasPrefix(*ptr, "https://") {
		return *ptr
	}
	return "http://" + serverAddr + "/boot/" + *ptr
}

// replaceBootVars replaces {{.URL}} and {{.MAC}} template vars with actual values.
func replaceBootVars(s, serverAddr, mac string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "{{.URL}}", "http://"+serverAddr)
	s = strings.ReplaceAll(s, "{{.MAC}}", mac)
	return s
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

// appendFailsafeEntry appends a "Failsafe Recovery Menu" entry at the end of the entries list.
func appendFailsafeEntry(entries []ipxe.MenuEntryData, serverAddr, mac string) []ipxe.MenuEntryData {
	failsafeURL := "http://" + serverAddr + "/boot/ipxe/failsafe"
	if mac != "" {
		failsafeURL += "?mac=" + mac
	}
	return append(entries, ipxe.MenuEntryData{
		Label: "Failsafe Recovery Menu",
		Type:  ipxe.BootChain,
		URL:   failsafeURL,
	})
}

// generateFailsafeScript returns a self-contained iPXE failsafe menu script.
func generateFailsafeScript(serverAddr, mac string) string {
	var b strings.Builder
	b.WriteString("#!ipxe\n")
	chainURL := fmt.Sprintf("http://%s/boot/ipxe/menu?mac=%s", serverAddr, mac)
	fmt.Fprintf(&b, "set chain-url %s\n", chainURL)
	b.WriteString("\n:fs_failsafe\n")
	b.WriteString("menu PxeGo Failsafe Menu\n")
	b.WriteString("item --gap System Operations\n")
	b.WriteString("item fs_localboot    Boot from local drive\n")
	b.WriteString("item fs_netconfig    Manual network configuration\n")
	b.WriteString("item fs_retry        Retry boot\n")
	b.WriteString("item fs_debug        iPXE Debug Shell\n")
	b.WriteString("item fs_reboot       Reboot System\n")
	b.WriteString("choose fs_choice || exit\n")
	b.WriteString("goto ${fs_choice}\n")
	b.WriteString("\n:fs_localboot\n")
	b.WriteString("exit\n")
	b.WriteString("\n:fs_netconfig\n")
	b.WriteString("echo\n")
	b.WriteString("echo Network Configuration:\n")
	b.WriteString("echo Available interfaces...\n")
	b.WriteString("ifstat\n")
	b.WriteString("imgfree\n")
	b.WriteString("echo -n Set network interface number [0 for net0, defaults to 0]: && read fs_net\n")
	b.WriteString("isset ${fs_net} || set fs_net 0\n")
	b.WriteString("echo -n IP: && read net${fs_net}/ip\n")
	b.WriteString("echo -n Subnet mask: && read net${fs_net}/netmask\n")
	b.WriteString("echo -n Gateway: && read net${fs_net}/gateway\n")
	b.WriteString("echo -n DNS: && read dns\n")
	b.WriteString("ifopen net${fs_net}\n")
	b.WriteString("echo Attempting chainload...\n")
	b.WriteString("goto fs_retry\n")
	b.WriteString("\n:fs_retry\n")
	fmt.Fprintf(&b, "chain %s || goto fs_failsafe\n", chainURL)
	b.WriteString("\n:fs_debug\n")
	b.WriteString("echo Type \"exit\" to return to menu\n")
	b.WriteString("shell\n")
	b.WriteString("goto fs_failsafe\n")
	b.WriteString("\n:fs_reboot\n")
	b.WriteString("reboot\n")
	return b.String()
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
	slog.Info("HTTP 服务启动", "service", "HTTP", "addr", addr)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务异常退出", "service", "HTTP", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("HTTP 服务关闭", "service", "HTTP")
	return s.srv.Shutdown(ctx)
}
