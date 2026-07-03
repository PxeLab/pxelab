package api

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/pxego/pxego/internal/config"
	"gopkg.in/yaml.v3"
)

// SubnetReloader 热重载子网配置
type SubnetReloader interface {
	ReloadSubnets()
}

type SettingsHandler struct {
	cfg      *config.Config
	reloader SubnetReloader
	mu       sync.Mutex
}

func NewSettingsHandler(cfg *config.Config, reloader SubnetReloader) *SettingsHandler {
	return &SettingsHandler{cfg: cfg, reloader: reloader}
}

type SettingsResponse struct {
	Server     ServerSettings      `json:"server"`
	DHCP       DHCPSettings        `json:"dhcp"`
	TFTP       TFTPSettings        `json:"tftp"`
	DNS        DNSSettings         `json:"dns"`
	IPMI       IPMISettings        `json:"ipmi"`
	Netboot    NetbootSettings     `json:"netboot"`
	WhitelistEnabled bool          `json:"whitelist_enabled"`
	LogLevel   string              `json:"log_level"`
	DataDir    string              `json:"data_dir"`
	Interfaces []InterfaceResponse `json:"interfaces"`
}

type ServerSettings struct {
	Name       string `json:"name"`
	AppMode    bool   `json:"app_mode"`
	Token      string `json:"token"`
	ListenAddr string `json:"listen_addr"`
	TokenSet   bool   `json:"token_set"`
}

type DHCPSettings struct {
	Enabled    bool   `json:"enabled"`
	Range      string `json:"range"`
	Gateway    string `json:"gateway"`
	Subnet     string `json:"subnet"`
	LeaseTime  int    `json:"lease_time"`
	DNSServers string `json:"dns_servers"`
}

type TFTPSettings struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Root    string `json:"root"`
}

type DNSSettings struct {
	Enabled       bool   `json:"enabled"`
	Port          int    `json:"port"`
	Upstream      string `json:"upstream"`
	LocalDomain   string `json:"local_domain"`
	DefaultRecord bool   `json:"default_record"`
}


type IPMISettings struct {
	Enabled bool `json:"enabled"`
	Timeout int  `json:"timeout"`
}

type NetbootSettings struct {
	Enabled        bool         `json:"enabled"`
	ScriptTemplate string       `json:"script_template"`
	Boot           BootSettings `json:"boot"`
}

type BootSettings struct {
	DefaultMenu     DefaultMenuSettings     `json:"default_menu"`
	CatalogRedirect CatalogRedirectSettings `json:"catalog_redirect"`
	CatalogDisplay  CatalogDisplaySettings  `json:"catalog_display"`
}

type DefaultMenuSettings struct {
	Title           string              `json:"title"`
	Timeout         int                 `json:"timeout"`
	Default         int                 `json:"default"`
	ListAllProfiles bool                `json:"list_all_profiles"`
	Entries         []MenuEntrySettings `json:"entries"`
}

type MenuEntrySettings struct {
	Label   string  `json:"label"`
	Type    string  `json:"type"`
	Kernel  *string `json:"kernel,omitempty"`
	Initrd  *string `json:"initrd,omitempty"`
	Cmdline *string `json:"cmdline,omitempty"`
	URL     *string `json:"url,omitempty"`
	WIM     *string `json:"wim,omitempty"`
}


type CatalogRedirectSettings struct {
	Enabled    bool   `json:"enabled"`
	TargetURL  string `json:"target_url"`
	DetectArch bool   `json:"detect_arch"`
	Preamble   string `json:"preamble"`
}

type CatalogDisplaySettings struct {
	Title string `json:"title"`
}

type SubnetSettings struct {
	CIDR       string   `json:"cidr"`
	DHCPMode   string   `json:"dhcp_mode"`
	Pools      []string `json:"pools"`
	Gateway    string   `json:"gateway"`
	DNSServers string   `json:"dns_servers"`
	LeaseTime  int      `json:"lease_time"`
	NextServer string   `json:"next_server"`
	ChainToIPXE bool     `json:"chain_to_ipxe"`
}

type InterfaceResponse struct {
	Name        string           `json:"name"`
	IP          string           `json:"ip"`
	DHCPMode    string           `json:"dhcp_mode"`
	Bootloader  string           `json:"bootloader"`
	ChainToIPXE bool             `json:"chain_to_ipxe"`
	Subnet      string           `json:"subnet"`      // 向后兼容：Subnets[0].CIDR
	Pools       []string         `json:"pools"`        // 向后兼容：Subnets[0].Pools
	Gateway     string           `json:"gateway"`      // 向后兼容：Subnets[0].Gateway
	DNSServers  string           `json:"dns_servers"`  // 向后兼容：Subnets[0].DNSServers
	LeaseTime   int              `json:"lease_time"`   // 向后兼容：Subnets[0].LeaseTime
	NextServer  string           `json:"next_server"`  // 向后兼容：Subnets[0].NextServer
	Subnets     []SubnetSettings `json:"subnets,omitempty"`
	TFTP        bool             `json:"tftp"`
	HTTP        bool             `json:"http"`
	AutoStart   bool             `json:"auto_start"`
}

// ── Sub-domain response types for individual settings endpoints ──

type GeneralSettingsResponse struct {
	ServerName       string              `json:"server_name"`
	AppMode          bool                `json:"app_mode"`
	Token            string              `json:"token"`
	TokenSet         bool                `json:"token_set"`
	ListenAddr       string              `json:"listen_addr"`
	LogLevel         string              `json:"log_level"`
	DataDir          string              `json:"data_dir"`
	WhitelistEnabled bool                `json:"whitelist_enabled"`
	ScriptTemplate   string              `json:"script_template"`
	DefaultMenu      DefaultMenuSettings `json:"default_menu"`
	PageSize         int                 `json:"page_size"`
	MigrateBoot      bool                `json:"migrate_boot"`
}

type InterfacesSettingsResponse struct {
	Interfaces []InterfaceResponse `json:"interfaces"`
}

type DHCPSettingsResponse struct {
	Enabled    bool   `json:"enabled"`
	Range      string `json:"range"`
	Gateway    string `json:"gateway"`
	Subnet     string `json:"subnet"`
	LeaseTime  int    `json:"lease_time"`
	DNSServers string `json:"dns_servers"`
}

type TFTPSettingsResponse struct {
	Enabled        bool   `json:"enabled"`
	Port           int    `json:"port"`
	Root           string `json:"root"`
	PXEConfigFile  string `json:"pxe_config_file"`
	GRUBConfigFile string `json:"grub_config_file"`
}

type DNSSettingsResponse struct {
	Enabled       bool   `json:"enabled"`
	Port          int    `json:"port"`
	Upstream      string `json:"upstream"`
	LocalDomain   string `json:"local_domain"`
	DefaultRecord bool   `json:"default_record"`
}

type NetbootSettingsResponse struct {
	Enabled         bool                    `json:"enabled"`
	ProxyHTTPS      bool                    `json:"proxy_https"`
	CacheEnabled    bool                    `json:"cache_enabled"`
	CatalogRedirect CatalogRedirectSettings `json:"catalog_redirect"`
	CatalogDisplay  CatalogDisplaySettings  `json:"catalog_display"`
}

func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func configPath(cfg *config.Config) string {
	if cfg.ConfigPath != "" {
		return cfg.ConfigPath
	}
	return filepath.Join(cfg.Global.DataDir, "config.yaml")
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg

	h.mu.Lock()
	if cfg.Auth.Token == "" && cfg.Auth.TokenHash == "" {
		cfg.Auth.Token = generateToken()
		hsh := sha256.Sum256([]byte(cfg.Auth.Token))
		cfg.Auth.TokenHash = hex.EncodeToString(hsh[:])
		if err := saveConfig(configPath(cfg), cfg); err != nil {
			slog.Error("保存 token 配置失败", "service", "HTTP", "error", err)
		}
		slog.Info("已生成初始 API 令牌（仅首次显示此日志，请妥善保管）", "service", "HTTP", "token", cfg.Auth.Token)
	}
	if cfg.Auth.TokenHash == "" && cfg.Auth.Token != "" {
		hsh := sha256.Sum256([]byte(cfg.Auth.Token))
		cfg.Auth.TokenHash = hex.EncodeToString(hsh[:])
		if err := saveConfig(configPath(cfg), cfg); err != nil {
			slog.Error("保存 token hash 失败", "service", "HTTP", "error", err)
		}
	}
	if cfg.Global.ServerName == "" {
		cfg.Global.ServerName = "pxego"
	}
	if cfg.Global.ListenAddr == "" {
		cfg.Global.ListenAddr = ":8080"
	}
	h.mu.Unlock()

	tokenSet := cfg.Auth.TokenHash != ""

	resp := SettingsResponse{
		LogLevel:         cfg.Log.Level,
		DataDir:          cfg.Global.DataDir,
		WhitelistEnabled: cfg.Global.WhitelistEnabled,
		Server: ServerSettings{
			Name:       cfg.Global.ServerName,
			AppMode:    cfg.Global.AppMode,
			Token:      cfg.Auth.Token,
			ListenAddr: cfg.Global.ListenAddr,
			TokenSet:   tokenSet,
		},
		DHCP: DHCPSettings{
			Enabled:    false,
			Range:      "",
			Gateway:    "",
			Subnet:     "",
			LeaseTime:  config.DefaultLeaseTime,
			DNSServers: "",
		},
		TFTP: TFTPSettings{
			Enabled: true,
			Port:    config.DefaultPortTFTP,
			Root:    cfg.Boot.RootDir,
		},
		DNS: DNSSettings{
			Enabled:       cfg.DNS.Enabled,
			Port:          cfg.DNS.Port,
			Upstream:      cfg.DNS.Upstream,
			LocalDomain:   cfg.DNS.LocalDomain,
			DefaultRecord: cfg.DNS.DefaultRecord,
		},
		Netboot: NetbootSettings{
			Enabled:        cfg.Netboot.Enabled,
			ScriptTemplate: cfg.Netboot.ScriptTemplate,
			Boot: BootSettings{
				DefaultMenu: DefaultMenuSettings{
					Title:   cfg.Netboot.Boot.DefaultMenu.Title,
					Timeout: cfg.Netboot.Boot.DefaultMenu.Timeout,
					Default: cfg.Netboot.Boot.DefaultMenu.Default,
					ListAllProfiles: cfg.Netboot.Boot.DefaultMenu.ListAllProfiles,
					Entries: convertMenuEntriesToAPI(cfg.Netboot.Boot.DefaultMenu.Entries),
				},
				CatalogRedirect: CatalogRedirectSettings{
					Enabled:    cfg.Netboot.Boot.CatalogRedirect.Enabled,
					TargetURL:  cfg.Netboot.Boot.CatalogRedirect.TargetURL,
					DetectArch: cfg.Netboot.Boot.CatalogRedirect.DetectArch,
					Preamble:   cfg.Netboot.Boot.CatalogRedirect.Preamble,
				},
				CatalogDisplay: CatalogDisplaySettings{
					Title:  cfg.Netboot.Boot.CatalogDisplay.Title,
	
				},
			},
		},
	}

	for _, iface := range cfg.Interfaces {
		ir := InterfaceResponse{
			Name:       iface.Name,
			IP:         iface.IP,
			Bootloader: iface.Bootloader,
			TFTP:       iface.TFTP,
			HTTP:       iface.HTTP,
			AutoStart:  iface.AutoStart,
			LeaseTime:  config.DefaultLeaseTime,
			DNSServers: "",
		}
		if len(iface.Subnets) > 0 {
			ir.DHCPMode = iface.Subnets[0].DHCP
		}
		if ir.DHCPMode == "" {
			ir.DHCPMode = "full"
		}
		if len(iface.Subnets) > 0 {
			sn := iface.Subnets[0]
			ir.Subnet = sn.CIDR
			if len(sn.Pools) > 0 {
				ir.Pools = sn.Pools
			} else if sn.Pool != "" {
				ir.Pools = []string{sn.Pool}
			}
			ir.Gateway = sn.Gateway
			ir.DNSServers = sn.DNSServers
			ir.NextServer = sn.NextServer
			if sn.LeaseTime > 0 {
				ir.LeaseTime = sn.LeaseTime
			}
			// 填充所有子网
			for _, sn := range iface.Subnets {
				dhcpMode := sn.DHCP
				if dhcpMode == "" {
					dhcpMode = "full"
				}
				pools := sn.Pools
				if len(pools) == 0 && sn.Pool != "" {
					pools = []string{sn.Pool}
				}
				ir.Subnets = append(ir.Subnets, SubnetSettings{
					CIDR:       sn.CIDR,
					DHCPMode:   dhcpMode,
					Pools:      pools,
					Gateway:    sn.Gateway,
					DNSServers: sn.DNSServers,
					LeaseTime:  sn.LeaseTime,
					NextServer: sn.NextServer,
					ChainToIPXE: sn.ChainToIPXE,
				})
			}
		}

		resp.Interfaces = append(resp.Interfaces, ir)
	}

	if len(cfg.Interfaces) > 0 && len(cfg.Interfaces[0].Subnets) > 0 {
		sn := cfg.Interfaces[0].Subnets[0]
		resp.DHCP.Enabled = cfg.Interfaces[0].Subnets[0].DHCP != "" && cfg.Interfaces[0].Subnets[0].DHCP != "off"
		if len(sn.Pools) > 0 {
			resp.DHCP.Range = sn.Pools[0]
		} else {
			resp.DHCP.Range = sn.Pool
		}
		resp.DHCP.Gateway = sn.Gateway
		resp.DHCP.Subnet = sn.CIDR
		resp.DHCP.LeaseTime = sn.LeaseTime
		resp.DHCP.DNSServers = sn.DNSServers
	}

	OK(w, resp)
}

func ipInCIDR(ipStr, cidr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return network.Contains(ip)
}

func poolsOverlap(a, b string) bool {
	parseRange := func(s string) (net.IP, net.IP) {
		parts := strings.SplitN(s, "-", 2)
		if len(parts) != 2 {
			return nil, nil
		}
		return net.ParseIP(strings.TrimSpace(parts[0])), net.ParseIP(strings.TrimSpace(parts[1]))
	}
	aStart, aEnd := parseRange(a)
	bStart, bEnd := parseRange(b)
	if aStart == nil || aEnd == nil || bStart == nil || bEnd == nil {
		return false
	}
	aStart4 := aStart.To4()
	aEnd4 := aEnd.To4()
	bStart4 := bStart.To4()
	bEnd4 := bEnd.To4()
	if aStart4 == nil || aEnd4 == nil || bStart4 == nil || bEnd4 == nil {
		return false
	}
	return bytes.Compare(aStart4, bEnd4) <= 0 && bytes.Compare(bStart4, aEnd4) <= 0
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req SettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	// 校验接口配置
	for i, ir := range req.Interfaces {
		// 如果没有子网数组，使用平铺字段向后兼容
		subnets := ir.Subnets
		if len(subnets) == 0 && ir.Subnet != "" {
			subnets = []SubnetSettings{{
				CIDR: ir.Subnet, DHCPMode: "full",
				Pools: ir.Pools, Gateway: ir.Gateway,
				DNSServers: ir.DNSServers, LeaseTime: ir.LeaseTime,
				NextServer: ir.NextServer,
				ChainToIPXE: false,
			}}
		}
		for si, s := range subnets {
			if s.DHCPMode == "" || s.DHCPMode == "off" || s.DHCPMode == "proxy" {
				continue
			}
			for pi, p := range s.Pools {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				parts := strings.SplitN(p, "-", 2)
				if len(parts) != 2 {
					Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d 子网 #%d: 地址池 #%d 格式无效", i+1, si+1, pi+1))
					return
				}
				startIP := strings.TrimSpace(parts[0])
				endIP := strings.TrimSpace(parts[1])
				if s.CIDR != "" {
					if !ipInCIDR(startIP, s.CIDR) {
						Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d 子网 #%d: 地址池 #%d 起始地址 %s 不属于子网 %s", i+1, si+1, pi+1, startIP, s.CIDR))
						return
					}
					if !ipInCIDR(endIP, s.CIDR) {
						Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d 子网 #%d: 地址池 #%d 结束地址 %s 不属于子网 %s", i+1, si+1, pi+1, endIP, s.CIDR))
						return
					}
				}
			}
			// 检测地址池冲突
			validPools := make([]string, 0, len(s.Pools))
			for _, p := range s.Pools {
				if strings.TrimSpace(p) != "" && strings.Contains(p, "-") {
					validPools = append(validPools, p)
				}
			}
			for pi := 0; pi < len(validPools); pi++ {
				for pj := pi + 1; pj < len(validPools); pj++ {
					if poolsOverlap(validPools[pi], validPools[pj]) {
						Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d 子网 #%d: 地址池 #%d 和 #%d 范围冲突", i+1, si+1, pi+1, pj+1))
						return
					}
				}
			}
		}
	}

	h.cfg.Log.Level = req.LogLevel
	h.cfg.Global.ServerName = req.Server.Name
	h.cfg.Global.AppMode = req.Server.AppMode
	h.cfg.Global.WhitelistEnabled = req.WhitelistEnabled
	if req.Server.ListenAddr != "" {
		h.cfg.Global.ListenAddr = req.Server.ListenAddr
	}
	h.cfg.DNS.Enabled = req.DNS.Enabled
	if req.DNS.Port > 0 {
		h.cfg.DNS.Port = req.DNS.Port
	}
	h.cfg.DNS.Upstream = req.DNS.Upstream
	h.cfg.DNS.LocalDomain = req.DNS.LocalDomain
	h.cfg.DNS.DefaultRecord = req.DNS.DefaultRecord
	if h.cfg.DNS.DefaultRecord && h.cfg.DNS.DefaultRecordIP == "" {
		// 从第一个接口自动获取服务器 IP
		for _, iface := range h.cfg.Interfaces {
			if iface.IP != "" && !net.ParseIP(iface.IP).IsLoopback() {
				h.cfg.DNS.DefaultRecordIP = iface.IP
				break
			}
		}
	}
	if !h.cfg.DNS.DefaultRecord {
		h.cfg.DNS.DefaultRecordIP = ""
	}
	if req.Server.Token != "" && req.Server.Token != h.cfg.Auth.Token && !strings.Contains(req.Server.Token, "...") {
		h.cfg.Auth.Token = req.Server.Token
		hsh := sha256.Sum256([]byte(req.Server.Token))
		h.cfg.Auth.TokenHash = hex.EncodeToString(hsh[:])
	}

	h.cfg.Netboot.Enabled = req.Netboot.Enabled
	h.cfg.Netboot.ScriptTemplate = req.Netboot.ScriptTemplate
	h.cfg.Netboot.Boot = config.BootConfig{
		RootDir: h.cfg.Boot.RootDir, // preserve existing root dir
		DefaultMenu: config.DefaultMenuConfig{
			Title:   req.Netboot.Boot.DefaultMenu.Title,
			Timeout: req.Netboot.Boot.DefaultMenu.Timeout,
			Default: req.Netboot.Boot.DefaultMenu.Default,
			ListAllProfiles: req.Netboot.Boot.DefaultMenu.ListAllProfiles,
			Entries: convertMenuEntriesFromAPI(req.Netboot.Boot.DefaultMenu.Entries),
		},
		CatalogRedirect: config.CatalogRedirectConfig{
			Enabled:    req.Netboot.Boot.CatalogRedirect.Enabled,
			TargetURL:  req.Netboot.Boot.CatalogRedirect.TargetURL,
			DetectArch: req.Netboot.Boot.CatalogRedirect.DetectArch,
			Preamble:   req.Netboot.Boot.CatalogRedirect.Preamble,
		},
		CatalogDisplay: config.CatalogDisplayConfig{
			Title: req.Netboot.Boot.CatalogDisplay.Title,
		},
	}

	if req.Interfaces != nil {
		h.cfg.Interfaces = make([]config.InterfaceConfig, 0, len(req.Interfaces))
		for _, ir := range req.Interfaces {
			iface := config.InterfaceConfig{
				Name: ir.Name,
				IP:   ir.IP,
				Bootloader: ir.Bootloader,
				TFTP: ir.TFTP,
				HTTP: ir.HTTP,
				AutoStart:  ir.AutoStart,
			}

			if len(ir.Subnets) > 0 {
				for _, s := range ir.Subnets {
					iface.Subnets = append(iface.Subnets, config.SubnetConfig{
						CIDR:       s.CIDR,
						DHCP:       s.DHCPMode,
						Pools:      s.Pools,
						Gateway:    s.Gateway,
						DNSServers: s.DNSServers,
						LeaseTime:  s.LeaseTime,
						NextServer: s.NextServer,
						ChainToIPXE: s.ChainToIPXE,
					})
				}
			} else if ir.Subnet != "" || len(ir.Pools) > 0 || ir.Gateway != "" || ir.NextServer != "" {
				iface.Subnets = []config.SubnetConfig{{
					CIDR:       ir.Subnet,
					DHCP:       "full",
					Pools:      ir.Pools,
					Gateway:    ir.Gateway,
					DNSServers: ir.DNSServers,
					NextServer: ir.NextServer,
					LeaseTime:  ir.LeaseTime,
					ChainToIPXE: false,
				}}
			}
			h.cfg.Interfaces = append(h.cfg.Interfaces, iface)
		}
	}

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	if h.reloader != nil {
		h.reloader.ReloadSubnets()
	}

	OK(w, map[string]string{"status": "saved"})
}

// ── General Settings ──

func (h *SettingsHandler) GetGeneral(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg

	h.mu.Lock()
	if cfg.Auth.Token == "" && cfg.Auth.TokenHash == "" {
		cfg.Auth.Token = generateToken()
		hsh := sha256.Sum256([]byte(cfg.Auth.Token))
		cfg.Auth.TokenHash = hex.EncodeToString(hsh[:])
		if err := saveConfig(configPath(cfg), cfg); err != nil {
			slog.Error("保存 token 配置失败", "service", "HTTP", "error", err)
		}
		slog.Info("已生成初始 API 令牌（仅首次显示此日志，请妥善保管）", "service", "HTTP", "token", cfg.Auth.Token)
	}
	if cfg.Auth.TokenHash == "" && cfg.Auth.Token != "" {
		hsh := sha256.Sum256([]byte(cfg.Auth.Token))
		cfg.Auth.TokenHash = hex.EncodeToString(hsh[:])
		if err := saveConfig(configPath(cfg), cfg); err != nil {
			slog.Error("保存 token hash 失败", "service", "HTTP", "error", err)
		}
	}
	if cfg.Global.ServerName == "" {
		cfg.Global.ServerName = "pxego"
	}
	if cfg.Global.ListenAddr == "" {
		cfg.Global.ListenAddr = ":8080"
	}
	h.mu.Unlock()

	resp := GeneralSettingsResponse{
		ServerName:       cfg.Global.ServerName,
		AppMode:          cfg.Global.AppMode,
		Token:            cfg.Auth.Token,
		TokenSet:         cfg.Auth.TokenHash != "",
		ListenAddr:       cfg.Global.ListenAddr,
		LogLevel:         cfg.Log.Level,
		DataDir:          cfg.Global.DataDir,
		WhitelistEnabled: cfg.Global.WhitelistEnabled,
		ScriptTemplate:   cfg.Netboot.ScriptTemplate,
		DefaultMenu: DefaultMenuSettings{
			Title:           cfg.Netboot.Boot.DefaultMenu.Title,
			Timeout:         cfg.Netboot.Boot.DefaultMenu.Timeout,
			Default:         cfg.Netboot.Boot.DefaultMenu.Default,
			ListAllProfiles: cfg.Netboot.Boot.DefaultMenu.ListAllProfiles,
			Entries:         convertMenuEntriesToAPI(cfg.Netboot.Boot.DefaultMenu.Entries),
		},
		PageSize: cfg.Global.PageSize,
	}

	OK(w, resp)
}

func (h *SettingsHandler) UpdateGeneral(w http.ResponseWriter, r *http.Request) {
	var req GeneralSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	oldDataDir := h.cfg.Global.DataDir

	// 校验 data_dir 路径是否可用
	if req.DataDir != "" {
		cleaned := filepath.Clean(req.DataDir)
		if err := os.MkdirAll(cleaned, 0755); err != nil {
			Error(w, http.StatusBadRequest, "数据目录无法创建: "+err.Error())
			return
		}
		testFile := filepath.Join(cleaned, ".pxego_write_test")
		if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
			Error(w, http.StatusBadRequest, "数据目录不可写: "+err.Error())
			return
		}
		os.Remove(testFile)
		req.DataDir = cleaned
	}

	h.mu.Lock()
	h.cfg.Global.ServerName = req.ServerName
	h.cfg.Global.AppMode = req.AppMode
	h.cfg.Global.WhitelistEnabled = req.WhitelistEnabled
	h.cfg.Global.PageSize = req.PageSize
	h.cfg.Global.DataDir = req.DataDir
	h.cfg.Log.Level = req.LogLevel
	if req.ListenAddr != "" {
		h.cfg.Global.ListenAddr = req.ListenAddr
	}
	if req.Token != "" && req.Token != h.cfg.Auth.Token && !strings.Contains(req.Token, "...") {
		h.cfg.Auth.Token = req.Token
		hsh := sha256.Sum256([]byte(req.Token))
		h.cfg.Auth.TokenHash = hex.EncodeToString(hsh[:])
	}
	// Netboot-extracted fields: script_template and default_menu
	h.cfg.Netboot.ScriptTemplate = req.ScriptTemplate
	h.cfg.Netboot.Boot.DefaultMenu = config.DefaultMenuConfig{
		Title:           req.DefaultMenu.Title,
		Timeout:         req.DefaultMenu.Timeout,
		Default:         req.DefaultMenu.Default,
		ListAllProfiles: req.DefaultMenu.ListAllProfiles,
		Entries:         convertMenuEntriesFromAPI(req.DefaultMenu.Entries),
	}
	// data_dir 变更时检查 boot.root_dir 是否默认值，按需迁移
	if req.DataDir != "" && req.DataDir != oldDataDir {
		oldBootDir := filepath.Join(oldDataDir, "boot")
		if h.cfg.Boot.RootDir == oldBootDir {
			newBootDir := filepath.Join(req.DataDir, "boot")
			if req.MigrateBoot {
				if err := copyDir(oldBootDir, newBootDir); err != nil {
					slog.Warn("迁移启动文件失败", "from", oldBootDir, "to", newBootDir, "error", err)
				}
			}
			h.cfg.Boot.RootDir = newBootDir
		}
	}
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	OK(w, map[string]string{"status": "saved"})
}

// ── Interfaces ──

func (h *SettingsHandler) GetInterfaces(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg
	var resp InterfacesSettingsResponse

	for _, iface := range cfg.Interfaces {
		ir := InterfaceResponse{
			Name:        iface.Name,
			IP:          iface.IP,
			Bootloader:  iface.Bootloader,
			TFTP:        iface.TFTP,
			HTTP:        iface.HTTP,
			AutoStart:   iface.AutoStart,
			LeaseTime:   config.DefaultLeaseTime,
			DNSServers:  "",
		}
		if len(iface.Subnets) > 0 {
			ir.DHCPMode = iface.Subnets[0].DHCP
		}
		if ir.DHCPMode == "" {
			ir.DHCPMode = "full"
		}
		if len(iface.Subnets) > 0 {
			sn := iface.Subnets[0]
			ir.Subnet = sn.CIDR
			if len(sn.Pools) > 0 {
				ir.Pools = sn.Pools
			} else if sn.Pool != "" {
				ir.Pools = []string{sn.Pool}
			}
			ir.Gateway = sn.Gateway
			ir.DNSServers = sn.DNSServers
			ir.NextServer = sn.NextServer
			if sn.LeaseTime > 0 {
				ir.LeaseTime = sn.LeaseTime
			}
			for _, sn := range iface.Subnets {
				dhcpMode := sn.DHCP
				if dhcpMode == "" {
					dhcpMode = "full"
				}
				pools := sn.Pools
				if len(pools) == 0 && sn.Pool != "" {
					pools = []string{sn.Pool}
				}
				ir.Subnets = append(ir.Subnets, SubnetSettings{
					CIDR:       sn.CIDR,
					DHCPMode:   dhcpMode,
					Pools:      pools,
					Gateway:    sn.Gateway,
					DNSServers: sn.DNSServers,
					LeaseTime:  sn.LeaseTime,
					NextServer: sn.NextServer,
					ChainToIPXE: sn.ChainToIPXE,
				})
			}
		}
		resp.Interfaces = append(resp.Interfaces, ir)
	}

	OK(w, resp)
}

func (h *SettingsHandler) UpdateInterfaces(w http.ResponseWriter, r *http.Request) {
	var req InterfacesSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	// Validation (same logic as Update)
	for i, ir := range req.Interfaces {
		subnets := ir.Subnets
		if len(subnets) == 0 && ir.Subnet != "" {
			subnets = []SubnetSettings{{
				CIDR: ir.Subnet, DHCPMode: "full",
				Pools: ir.Pools, Gateway: ir.Gateway,
				DNSServers: ir.DNSServers, LeaseTime: ir.LeaseTime,
				NextServer: ir.NextServer,
				ChainToIPXE: false,
			}}
		}
		for si, s := range subnets {
			if s.DHCPMode == "" || s.DHCPMode == "off" || s.DHCPMode == "proxy" {
				continue
			}
			for pi, p := range s.Pools {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				parts := strings.SplitN(p, "-", 2)
				if len(parts) != 2 {
					Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d 子网 #%d: 地址池 #%d 格式无效", i+1, si+1, pi+1))
					return
				}
				startIP := strings.TrimSpace(parts[0])
				endIP := strings.TrimSpace(parts[1])
				if s.CIDR != "" {
					if !ipInCIDR(startIP, s.CIDR) {
						Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d 子网 #%d: 地址池 #%d 起始地址 %s 不属于子网 %s", i+1, si+1, pi+1, startIP, s.CIDR))
						return
					}
					if !ipInCIDR(endIP, s.CIDR) {
						Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d 子网 #%d: 地址池 #%d 结束地址 %s 不属于子网 %s", i+1, si+1, pi+1, endIP, s.CIDR))
						return
					}
				}
			}
			validPools := make([]string, 0, len(s.Pools))
			for _, p := range s.Pools {
				if strings.TrimSpace(p) != "" && strings.Contains(p, "-") {
					validPools = append(validPools, p)
				}
			}
			for pi := 0; pi < len(validPools); pi++ {
				for pj := pi + 1; pj < len(validPools); pj++ {
					if poolsOverlap(validPools[pi], validPools[pj]) {
						Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d 子网 #%d: 地址池 #%d 和 #%d 范围冲突", i+1, si+1, pi+1, pj+1))
						return
					}
				}
			}
		}
	}

	h.mu.Lock()
	h.cfg.Interfaces = make([]config.InterfaceConfig, 0, len(req.Interfaces))
	for _, ir := range req.Interfaces {
		iface := config.InterfaceConfig{
			Name:       ir.Name,
			IP:         ir.IP,
			Bootloader: ir.Bootloader,
			TFTP:       ir.TFTP,
			HTTP:       ir.HTTP,
			AutoStart:  ir.AutoStart,
		}
		if len(ir.Subnets) > 0 {
			for _, s := range ir.Subnets {
				iface.Subnets = append(iface.Subnets, config.SubnetConfig{
					CIDR:       s.CIDR,
					DHCP:       s.DHCPMode,
					Pools:      s.Pools,
					Gateway:    s.Gateway,
					DNSServers: s.DNSServers,
					LeaseTime:  s.LeaseTime,
					NextServer: s.NextServer,
				})
			}
		} else if ir.Subnet != "" || len(ir.Pools) > 0 || ir.Gateway != "" || ir.NextServer != "" {
			iface.Subnets = []config.SubnetConfig{{
				CIDR:       ir.Subnet,
				DHCP:       "full",
				Pools:      ir.Pools,
				Gateway:    ir.Gateway,
				DNSServers: ir.DNSServers,
				NextServer: ir.NextServer,
				LeaseTime:  ir.LeaseTime,
			}}
		}
		h.cfg.Interfaces = append(h.cfg.Interfaces, iface)
	}
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	if h.reloader != nil {
		h.reloader.ReloadSubnets()
	}

	OK(w, map[string]string{"status": "saved"})
}

// ── TFTP ──

func (h *SettingsHandler) GetTFTP(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg
	OK(w, TFTPSettingsResponse{
		Enabled:        true,
		Port:           config.DefaultPortTFTP,
		Root:           cfg.Boot.RootDir,
		PXEConfigFile:  cfg.Boot.PXEConfigFile,
		GRUBConfigFile: cfg.Boot.GRUBConfigFile,
	})
}

func (h *SettingsHandler) UpdateTFTP(w http.ResponseWriter, r *http.Request) {
	var req TFTPSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	h.mu.Lock()
	h.cfg.Boot.RootDir = req.Root
	if req.PXEConfigFile != "" {
		h.cfg.Boot.PXEConfigFile = req.PXEConfigFile
	}
	if req.GRUBConfigFile != "" {
		h.cfg.Boot.GRUBConfigFile = req.GRUBConfigFile
	}
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	OK(w, map[string]string{"status": "saved"})
}

// ── DHCP (legacy single-interface) ──

func (h *SettingsHandler) GetDHCP(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg
	resp := DHCPSettingsResponse{
		Enabled:   false,
		Range:     "",
		Gateway:   "",
		Subnet:    "",
		LeaseTime: config.DefaultLeaseTime,
	}

	if len(cfg.Interfaces) > 0 && len(cfg.Interfaces[0].Subnets) > 0 {
		sn := cfg.Interfaces[0].Subnets[0]
		resp.Enabled = sn.DHCP != "" && sn.DHCP != "off"
		if len(sn.Pools) > 0 {
			resp.Range = sn.Pools[0]
		} else {
			resp.Range = sn.Pool
		}
		resp.Gateway = sn.Gateway
		resp.Subnet = sn.CIDR
		resp.LeaseTime = sn.LeaseTime
		resp.DNSServers = sn.DNSServers
	}

	OK(w, resp)
}

func (h *SettingsHandler) UpdateDHCP(w http.ResponseWriter, r *http.Request) {
	var req DHCPSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	h.mu.Lock()
	if len(h.cfg.Interfaces) > 0 && len(h.cfg.Interfaces[0].Subnets) > 0 {
		dhcpMode := "off"
		if req.Enabled {
			dhcpMode = "full"
		}
		h.cfg.Interfaces[0].Subnets[0].DHCP = dhcpMode
		if req.Enabled {
			if req.Range != "" {
				h.cfg.Interfaces[0].Subnets[0].Pools = []string{req.Range}
			}
			h.cfg.Interfaces[0].Subnets[0].Gateway = req.Gateway
			h.cfg.Interfaces[0].Subnets[0].CIDR = req.Subnet
			if req.LeaseTime > 0 {
				h.cfg.Interfaces[0].Subnets[0].LeaseTime = req.LeaseTime
			}
			h.cfg.Interfaces[0].Subnets[0].DNSServers = req.DNSServers
		}
	}
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	if h.reloader != nil {
		h.reloader.ReloadSubnets()
	}

	OK(w, map[string]string{"status": "saved"})
}

// ── DNS ──

func (h *SettingsHandler) GetDNS(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg
	OK(w, DNSSettingsResponse{
		Enabled:       cfg.DNS.Enabled,
		Port:          cfg.DNS.Port,
		Upstream:      cfg.DNS.Upstream,
		LocalDomain:   cfg.DNS.LocalDomain,
		DefaultRecord: cfg.DNS.DefaultRecord,
	})
}

func (h *SettingsHandler) UpdateDNS(w http.ResponseWriter, r *http.Request) {
	var req DNSSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	h.mu.Lock()
	h.cfg.DNS.Enabled = req.Enabled
	if req.Port > 0 {
		h.cfg.DNS.Port = req.Port
	}
	h.cfg.DNS.Upstream = req.Upstream
	h.cfg.DNS.LocalDomain = req.LocalDomain
	h.cfg.DNS.DefaultRecord = req.DefaultRecord
	if h.cfg.DNS.DefaultRecord && h.cfg.DNS.DefaultRecordIP == "" {
		for _, iface := range h.cfg.Interfaces {
			if iface.IP != "" && !net.ParseIP(iface.IP).IsLoopback() {
				h.cfg.DNS.DefaultRecordIP = iface.IP
				break
			}
		}
	}
	if !h.cfg.DNS.DefaultRecord {
		h.cfg.DNS.DefaultRecordIP = ""
	}
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	OK(w, map[string]string{"status": "saved"})
}

// ── Netboot (stripped down — no script_template or default_menu) ──

func (h *SettingsHandler) GetNetboot(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg
	OK(w, NetbootSettingsResponse{
		Enabled: cfg.Netboot.Enabled,
		ProxyHTTPS: cfg.Netboot.ProxyHTTPS,
		CacheEnabled: cfg.Netboot.CacheEnabled,
		CatalogRedirect: CatalogRedirectSettings{
			Enabled:    cfg.Netboot.Boot.CatalogRedirect.Enabled,
			TargetURL:  cfg.Netboot.Boot.CatalogRedirect.TargetURL,
			DetectArch: cfg.Netboot.Boot.CatalogRedirect.DetectArch,
			Preamble:   cfg.Netboot.Boot.CatalogRedirect.Preamble,
		},
		CatalogDisplay: CatalogDisplaySettings{
			Title:  cfg.Netboot.Boot.CatalogDisplay.Title,
	
		},
	})
}

func (h *SettingsHandler) UpdateNetboot(w http.ResponseWriter, r *http.Request) {
	var req NetbootSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	h.mu.Lock()
	h.cfg.Netboot.Enabled = req.Enabled
	h.cfg.Netboot.ProxyHTTPS = req.ProxyHTTPS
	h.cfg.Netboot.CacheEnabled = req.CacheEnabled
	h.cfg.Netboot.Boot.CatalogRedirect = config.CatalogRedirectConfig{
		Enabled:    req.CatalogRedirect.Enabled,
		TargetURL:  req.CatalogRedirect.TargetURL,
		DetectArch: req.CatalogRedirect.DetectArch,
		Preamble:   req.CatalogRedirect.Preamble,
	}
	h.cfg.Netboot.Boot.CatalogDisplay = config.CatalogDisplayConfig{
		Title:  req.CatalogDisplay.Title,
	}
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	OK(w, map[string]string{"status": "saved"})
}

type CacheStatsResponse struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"size_bytes"`
	FileCount int    `json:"file_count"`
}

func (h *SettingsHandler) GetCacheStats(w http.ResponseWriter, r *http.Request) {
	dd := h.cfg.Global.DataDir
	if dd == "" {
		dd = ".pxego"
	}
	cacheDir := filepath.Join(dd, "cache", "netboot")

	var size int64
	var count int
	filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		if !info.IsDir() {
			size += info.Size()
			count++
		}
		return nil
	})

	OK(w, CacheStatsResponse{
		Path:      cacheDir,
		SizeBytes: size,
		FileCount: count,
	})
}

func convertMenuEntriesToAPI(entries []config.MenuEntry) []MenuEntrySettings {
	if len(entries) == 0 {
		return nil
	}
	result := make([]MenuEntrySettings, len(entries))
	for i, e := range entries {
		result[i] = MenuEntrySettings{
			Label:   e.Label,
			Type:    e.Type,
			Kernel:  e.Kernel,
			Initrd:  e.Initrd,
			Cmdline: e.Cmdline,
			URL:     e.URL,
			WIM:     e.WIM,
		}
	}
	return result
}
func convertMenuEntriesFromAPI(entries []MenuEntrySettings) []config.MenuEntry {
	if len(entries) == 0 {
		return nil
	}
	result := make([]config.MenuEntry, len(entries))
	for i, e := range entries {
		result[i] = config.MenuEntry{
			Label:   e.Label,
			Type:    e.Type,
			Kernel:  e.Kernel,
			Initrd:  e.Initrd,
			Cmdline: e.Cmdline,
			URL:     e.URL,
			WIM:     e.WIM,
		}
	}
	return result
}
func saveConfig(path string, cfg *config.Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	return enc.Encode(cfg)
}

// copyDir 递归复制目录内容
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
