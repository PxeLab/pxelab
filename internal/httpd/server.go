package httpd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/pxelab/pxelab/internal/api"
	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/boot/ipxe"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/netboot/menus"
	"github.com/pxelab/pxelab/internal/session"
	"github.com/pxelab/pxelab/internal/store"
	"os"
	"path/filepath"
	"time"
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

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer, spaHandler http.Handler, reloader api.SubnetReloader, netbootMgr *netboot.Manager, clientInfo func(ip string) (arch, platform string, ok bool), svcController api.ServiceController, sessions *session.Store, setNFSAllowIPs func(ips []string)) *Server {
	r := chi.NewRouter()
	r.Use(slogMiddleware)
	r.Use(chimw.Recoverer)
	r.Use(CORSMiddleware)
	r.Use(AuthMiddleware(cfg, sessions))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	apiHandler := api.NewHandler(cfg, st, bus, bootFS, reloader, netbootMgr, svcController, sessions, setNFSAllowIPs)
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
			if ci != nil && (filePath == cfgLocal.Boot.PXEConfigFile || filePath == cfgLocal.Boot.GRUBConfigFile) {
				clientIP := r.RemoteAddr
				if host, _, err := net.SplitHostPort(clientIP); err == nil {
					clientIP = host
				}
				arch, platform, ok := ci(clientIP)
				if !ok {
					arch, platform = "x86", "pc"
				}

				if chainToIPXEFallback(cfgLocal, filePath, clientIP) {
					var config string
					if filePath == cfgLocal.Boot.GRUBConfigFile {
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
		// Chain wrapper --- overrides boot_domain and chains to the hosted menu
		r.Get("/netboot/menu.ipxe", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintf(w, "#!ipxe\n")
			fmt.Fprintf(w, "set boot_domain %s/netboot/menu\n", r.Host)
			fmt.Fprintf(w, "set site_name PxeLab Netboot\n")
			fmt.Fprintf(w, "chain http://${boot_domain}/menu.ipxe\n")
		})

		// Static file serving for netboot.xyz menu files
		// boot.cfg is templated at runtime to set the local boot_domain
		menuFS := menus.Open()
		r.Get("/netboot/menu/*", func(w http.ResponseWriter, r *http.Request) {
			filePath := chi.URLParam(r, "*")
			filePath = strings.TrimPrefix(filePath, "/")
			if filePath == "" {
				http.Redirect(w, r, "/netboot/menu/menu.ipxe", http.StatusFound)
				return
			}

			// Template boot.cfg with the local server's boot_domain
			if filePath == "boot.cfg" {
				data, err := fs.ReadFile(menuFS, "boot.cfg")
				if err != nil {
				http.NotFound(w, r)
				return
				}
				s := strings.Replace(string(data),
					"set boot_domain boot.netboot.xyz/3.0.2",
					"set boot_domain "+r.Host+"/netboot/menu", 1)
				s = strings.Replace(s,
					"set sigs_enabled true",
					"set sigs_enabled false", 1)
				s = strings.Replace(s,
					"isset ${live_endpoint} || set live_endpoint https://github.com/netbootxyz",
					"set live_endpoint http://"+r.Host+"/boot/netboot/proxy/https/github.com/netbootxyz", 1)
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				w.Write([]byte(s))
				return
			}

			http.StripPrefix("/netboot/menu/", http.FileServer(http.FS(menuFS))).ServeHTTP(w, r)
		})

		// Proxy GitHub HTTPS assets to HTTP for iPXE (which lacks HTTPS support)
		// Caches downloaded files to disk for subsequent requests.
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

			cacheDir := netbootCacheDir(cfg)
			cacheFile := cachedFilePath(cacheDir, targetURL)

			// Return cached file if available
			if cfg.Netboot.CacheEnabled {
				if f, err := os.Open(cacheFile); err == nil {
					defer f.Close()
					st, _ := f.Stat()
					w.Header().Set("Content-Type", "application/octet-stream")
					http.ServeContent(w, r, "", st.ModTime(), f)
					slog.Debug("netboot cache hit", "service", "HTTP", "url", targetURL)
					return
				}
			}

			// Download upstream
			resp, err := http.Get(targetURL)
			if err != nil {
				slog.Error("netboot proxy fetch failed", "service", "HTTP", "url", targetURL, "error", err)
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()

			// Stream to client, optionally saving to disk cache
			if cfg.Netboot.CacheEnabled {
				os.MkdirAll(cacheDir, 0755)
				tmpPath := cacheFile + "." + strconv.FormatInt(time.Now().UnixNano(), 36) + ".tmp"
				cacheOut, err := os.Create(tmpPath)
				if err != nil {
					slog.Warn("netboot cache write failed, falling back to passthrough", "service", "HTTP", "error", err)
				}

				var src io.Reader = resp.Body
				if cacheOut != nil {
					defer cacheOut.Close()
					src = io.TeeReader(resp.Body, cacheOut)
				}

				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
				w.WriteHeader(resp.StatusCode)

				_, copyErr := io.Copy(w, src)

				if cacheOut != nil {
					if copyErr != nil {
						os.Remove(tmpPath)
					} else {
						os.Rename(tmpPath, cacheFile)
					}
				}
				if copyErr != nil {
					slog.Error("netboot proxy copy error", "service", "HTTP", "error", copyErr)
				}
			} else {
				w.Header().Set("Content-Type", "application/octet-stream")
				w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
				w.WriteHeader(resp.StatusCode)
				io.Copy(w, resp.Body)
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
							Kernel:  urlJoin(serverAddr, replaceBootVars(ptrStr(e.Kernel), serverAddr, mac)),
							Initrd:  urlJoin(serverAddr, replaceBootVars(ptrStr(e.Initrd), serverAddr, mac)),
							Cmdline: replaceBootVars(ptrStr(e.Cmdline), serverAddr, mac),
							URL:     replaceBootVars(ptrStr(e.URL), serverAddr, mac),
							WIM:     ptrStr(e.WIM),
							SANAction:     e.SANAction,
							SANNoDescribe: e.SANNoDescribe,
							SANDrive:      e.SANDrive,
							SANKeepSAN:    e.SANKeepSAN,
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
	if cr.Enabled {
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
	menuTitle := "PxeLab Boot Menu"

	if defProfile, err := st.GetDefaultProfile(ctx); err == nil && defProfile != nil {
		if bootMenu, err := defProfile.GetMenu(); err == nil && len(bootMenu.Entries) > 0 {
			entry := bootMenu.Entries[0]
			makeEntry := func(label string, e models.MenuEntry) ipxe.MenuEntryData {
				return ipxe.MenuEntryData{
					Label:   label,
					Type:    ipxe.BootType(e.Type),
					Kernel:  urlJoin(serverAddr, replaceBootVars(ptrStr(e.Kernel), serverAddr, mac)),
					Initrd:  urlJoin(serverAddr, replaceBootVars(ptrStr(e.Initrd), serverAddr, mac)),
					Cmdline: replaceBootVars(ptrStr(e.Cmdline), serverAddr, mac),
					URL:     replaceBootVars(ptrStr(e.URL), serverAddr, mac),
					WIM:     ptrStr(e.WIM),
					SANAction:     e.SANAction,
					SANNoDescribe: e.SANNoDescribe,
					SANDrive:      e.SANDrive,
					SANKeepSAN:    e.SANKeepSAN,
				}
			}

			if cfg.Netboot.Boot.DefaultMenu.ListAllProfiles {
				// 全部展示模式：列出所有 Profile，默认 Profile 排第一
				menuTitle = "PxeLab Boot Menu"
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

func urlJoin(serverAddr string, path string) string {
	if path == "" {
		return ""
	}
	// absolute URL (http/https) passes through unchanged
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return "http://" + serverAddr + "/boot/" + path
}

// replaceBootVars replaces {{.URL}}, {{.MAC}}, {{.NextServer}} template vars with actual values.
func replaceBootVars(s, serverAddr, mac string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "{{.URL}}", "http://"+serverAddr)
	s = strings.ReplaceAll(s, "{{.MAC}}", mac)
	s = strings.ReplaceAll(s, "{{.NextServer}}", stripPort(serverAddr))
	return s
}

// stripPort removes port number from host:port string.
func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// chainToIPXEFallback 根据客户端所在子网决定是否返回 chain-load 配置
func chainToIPXEFallback(cfg *config.Config, filePath, clientIP string) bool {
	for _, iface := range cfg.Interfaces {
		for _, sn := range iface.Subnets {
			if !sn.ChainToIPXE {
				continue
			}
			if !ipInCIDR(clientIP, sn.CIDR) {
				continue
			}
			switch filePath {
			case "grub2/grub.cfg":
				if iface.Bootloader == "grub2" {
					return true
				}
			case "pxelinux.cfg/default":
				if iface.Bootloader == "pxelinux" {
					return true
				}
			}
		}
	}
	return false
}

// ipInCIDR checks whether an IP string falls within a CIDR notation.
func ipInCIDR(ipStr, cidr string) bool {
	_, cidrNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return cidrNet.Contains(ip)
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
	b.WriteString("menu PxeLab Failsafe Menu\n")
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

// extractPXEMac extracts MAC address from a PXELinux or GRUB2 MAC-based config file path.
// Returns "" if the path is not a MAC-specific config.
// PXELinux format: pxelinux.cfg/01-aa-bb-cc-dd-ee-ff
// GRUB2 format:   grub2/grub.cfg-01-aa-bb-cc-dd-ee-ff (derived from GRUBConfigFile path)
func extractPXEMac(filePath string, cfg *config.Config) string {
	var prefix string
	if strings.HasPrefix(filePath, cfg.Boot.PXEConfigFile) {
		// PXELinux MAC file: same dir as default, e.g. "pxelinux.cfg/01-aa-bb-cc-dd-ee-ff"
		idx := strings.LastIndex(cfg.Boot.PXEConfigFile, "/")
		if idx < 0 {
			prefix = "01-"
		} else {
			prefix = cfg.Boot.PXEConfigFile[:idx+1] + "01-"
		}
	} else if strings.Contains(filePath, cfg.Boot.GRUBConfigFile+"-") {
		// GRUB2 MAC file: e.g. "grub2/grub.cfg-01-aa-bb-cc-dd-ee-ff"
		idx := strings.Index(filePath, cfg.Boot.GRUBConfigFile+"-")
		if idx >= 0 {
			prefix = cfg.Boot.GRUBConfigFile + "-01-"
		}
	}
	if prefix == "" {
		return ""
	}
	if !strings.HasPrefix(filePath, prefix) {
		return ""
	}
	macPart := strings.TrimPrefix(filePath, prefix)
	// Validate: 17 chars (xx-xx-xx-xx-xx-xx) with hex digits
	if len(macPart) != 17 {
		return ""
	}
	parts := strings.SplitN(macPart, "-", 7)
	if len(parts) != 6 {
		return ""
	}
	for _, p := range parts {
		if len(p) != 2 {
			return ""
		}
		for _, c2 := range p {
			if !((c2 >= '0' && c2 <= '9') || (c2 >= 'a' && c2 <= 'f') || (c2 >= 'A' && c2 <= 'F')) {
				return ""
			}
		}
	}
	// Rejoin with colons for store lookup
	return strings.Join(parts, ":")
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

// netbootCacheDir returns the disk cache directory for netboot proxy assets.
func netbootCacheDir(cfg *config.Config) string {
	dd := cfg.Global.DataDir
	if dd == "" {
		dd = ".pxelab"
	}
	return filepath.Join(dd, "cache", "netboot")
}

// cachedFilePath returns the local cache file path for a given target URL.
func cachedFilePath(cacheDir, targetURL string) string {
	h := sha256.Sum256([]byte(targetURL))
	return filepath.Join(cacheDir, hex.EncodeToString(h[:]))
}
