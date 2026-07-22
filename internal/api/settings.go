package api

import (
	"bytes"
	"context"
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
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
	"gopkg.in/yaml.v3"
)

// SubnetReloader 热重载子网配置
type SubnetReloader interface {
	ReloadSubnets()
}

type SettingsHandler struct {
	cfg               *config.Config
	store             store.Interface
	reloader          SubnetReloader
	setNFSMountPoints func(mps []config.NFSMountPoint)
	getNFSConnections func() map[string]NFSConnectionInfo
	isServiceRunning  func(name string) bool
	mu                sync.Mutex
}

type NFSConnectionInfo struct {
	Connections int          `json:"connections"`
	Clients     []NFSClientInfo `json:"clients"`
}

type NFSClientInfo struct {
	IP           string `json:"ip"`
	ConnectedAt string `json:"connected_at"`
	LastActivity string `json:"last_activity"`
}

func NewSettingsHandler(cfg *config.Config, st store.Interface, reloader SubnetReloader, setNFSMountPoints func(mps []config.NFSMountPoint), getNFSConnections func() map[string]NFSConnectionInfo, isServiceRunning func(name string) bool) *SettingsHandler {
	return &SettingsHandler{cfg: cfg, store: st, reloader: reloader, setNFSMountPoints: setNFSMountPoints, getNFSConnections: getNFSConnections, isServiceRunning: isServiceRunning}
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
	Timeout        int    `json:"timeout"`
	Root           string `json:"root"`
	PXEConfigFile  string `json:"pxe_config_file"`
	GRUBConfigFile string `json:"grub_config_file"`
}

// ArchEntryResponse 单架构映射
type ArchEntryResponse struct {
	ArchCode   int    `json:"arch_code"`
	ArchName   string `json:"arch_name"`
	NBP        string `json:"nbp"`         // "ipxe" | "pxelinux" | "grub2"
	ChainLoad  bool   `json:"chain_load"`  // 是否链式加载到 iPXE
	IPXE       string `json:"ipxe"`
	PXELinux   string `json:"pxelinux"`
	GRUB       string `json:"grub"`
	GRUBConfig string `json:"grub_config"`

	// Secure Boot 支持
	SecureBoot bool   `json:"secure_boot"`
	IPXESB     string `json:"ipxe_sb"`
	ShimIPXE   string `json:"shim_ipxe"`   // iPXE 的 UEFI Shim
	GRUBSB     string `json:"grub_sb"`     // Secure Boot 签名的 GRUB2 二进制
	ShimGRUB   string `json:"shim_grub"`   // GRUB2 的 UEFI Shim
}

// ArchMapResponse 完整的架构映射表
type ArchMapResponse struct {
	Entries []ArchEntryResponse `json:"entries"`
}

type DNSSettingsResponse struct {
	Enabled       bool   `json:"enabled"`
	Port          int    `json:"port"`
	Upstream      string `json:"upstream"`
	LocalDomain   string `json:"local_domain"`
	DefaultRecord bool   `json:"default_record"`
}

type NFSMountPointResponse struct {
	Label           string          `json:"label"`
	ExportPath      string          `json:"export_path"`
	LocalDir        string          `json:"local_dir"`
	ReadOnly        bool            `json:"read_only"`
	AllowIPs        []string        `json:"allow_ips"`
	ConnectionCount int             `json:"connection_count"`
	Clients         []NFSClientInfo `json:"clients,omitempty"`
}

type NFSSettingsResponse struct {
	Enabled       bool                    `json:"enabled"`
	Running       bool                    `json:"running"`
	Port          int                     `json:"port"`
	RpcbindPort   int                     `json:"rpcbind_port"`
	Version       string                  `json:"version"`
	MountPoints   []NFSMountPointResponse `json:"mount_points"`
}

type NetbootSettingsResponse struct {
	Enabled         bool                    `json:"enabled"`
	ProxyHTTPS      bool                    `json:"proxy_https"`
	CacheEnabled    bool                    `json:"cache_enabled"`
	CatalogRedirect CatalogRedirectSettings `json:"catalog_redirect"`
	CatalogDisplay  CatalogDisplaySettings  `json:"catalog_display"`
}

// IPXEScriptSettingsResponse iPXE 脚本配置（DHCP Option 175）
type IPXEScriptSettingsResponse struct {
	Enabled      bool   `json:"enabled"`
	Port         int    `json:"port"`
	Path         string `json:"path"`
	FeatureFlags int    `json:"feature_flags"`
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
		cfg.Global.ServerName = "PxeLab"
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
			ir.DHCPMode = "server"
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
					dhcpMode = "server"
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
				CIDR: ir.Subnet, DHCPMode: "server",
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
					DHCP:       "server",
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

	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), "更新完整配置")
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
		cfg.Global.ServerName = "PxeLab"
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
	oldServerName := h.cfg.Global.ServerName
	oldPageSize := h.cfg.Global.PageSize
	oldAppMode := h.cfg.Global.AppMode
	oldLogLevel := h.cfg.Log.Level

	// 校验 data_dir 路径是否可用
	if req.DataDir != "" {
		cleaned := filepath.Clean(req.DataDir)
		if err := os.MkdirAll(cleaned, 0755); err != nil {
			Error(w, http.StatusBadRequest, "数据目录无法创建: "+err.Error())
			return
		}
		testFile := filepath.Join(cleaned, ".pxelab_write_test")
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

	// Build change description
	var changes []string
	if oldServerName != req.ServerName {
		changes = append(changes, "服务器名称: "+oldServerName+" → "+req.ServerName)
	}
	if oldPageSize != req.PageSize {
		changes = append(changes, fmt.Sprintf("每页条数: %d → %d", oldPageSize, req.PageSize))
	}
	if oldAppMode != req.AppMode {
		mode := "生产模式"
		if req.AppMode {
			mode = "开发模式"
		}
		changes = append(changes, "应用模式: "+mode)
	}
	if oldLogLevel != req.LogLevel {
		changes = append(changes, "日志级别: "+oldLogLevel+" → "+req.LogLevel)
	}
	if oldDataDir != req.DataDir {
		changes = append(changes, "数据目录: "+oldDataDir+" → "+req.DataDir)
	}
	detail := "通用设置已保存"
	if len(changes) > 0 {
		detail = strings.Join(changes, "; ")
	}

	// 服务器名称变更时同步 DNS 记录
	if h.store != nil && h.cfg.DNS.LocalDomain != "" && oldServerName != req.ServerName {
		if oldServerName != "" && oldServerName != "@" {
			oldRecords, _ := h.store.FindDNSRecords(context.Background(), oldServerName, "A", "")
			for _, rec := range oldRecords {
				_ = h.store.DeleteDNSRecord(context.Background(), rec.ID)
			}
		}
		if req.ServerName != "" && req.ServerName != "@" {
			existing, _ := h.store.FindDNSRecords(context.Background(), req.ServerName, "A", "")
			if len(existing) == 0 {
				serverIP := "127.0.0.1"
				for _, iface := range h.cfg.Interfaces {
					ip := net.ParseIP(iface.IP)
					if ip != nil && !ip.IsLoopback() {
						serverIP = iface.IP
						break
					}
				}
				_ = h.store.CreateDNSRecord(context.Background(), &models.DNSRecord{
					Name:    req.ServerName,
					Type:    "A",
					Value:   serverIP,
					TTL:     300,
					Enabled: true,
				})
			}
		}
	}

	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), detail)
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
			ir.DHCPMode = "server"
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
					dhcpMode = "server"
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
				CIDR: ir.Subnet, DHCPMode: "server",
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
				DHCP:       "server",
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

	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), "更新接口配置")
	OK(w, map[string]string{"status": "saved"})
}

// ── TFTP ──

func (h *SettingsHandler) GetTFTP(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg
	port := cfg.TFTP.Port
	if port <= 0 {
		port = config.DefaultPortTFTP
	}
	timeout := cfg.TFTP.Timeout
	if timeout <= 0 {
		timeout = config.DefaultTFTPTimeout
	}
	OK(w, TFTPSettingsResponse{
		Enabled:        true,
		Port:           port,
		Timeout:        timeout,
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
	if req.Port > 0 {
		h.cfg.TFTP.Port = req.Port
	}
	if req.Timeout >= 0 {
		h.cfg.TFTP.Timeout = req.Timeout
	}
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), "更新 TFTP 配置")
	OK(w, map[string]string{"status": "saved"})
}

// ── ArchMap ──

func (h *SettingsHandler) GetArchMap(w http.ResponseWriter, r *http.Request) {
	entries := make([]ArchEntryResponse, 0)
	for code, entry := range boot.GetArchMap() {
		entries = append(entries, ArchEntryResponse{
			ArchCode:   code,
			ArchName:   boot.ArchName(code),
			NBP:        entry.NBP,
			ChainLoad:  entry.ChainLoad,
			IPXE:       entry.IPXE,
			PXELinux:   entry.PXELinux,
			GRUB:       entry.GRUB,
			GRUBConfig: entry.GRUBConfig,
			SecureBoot: entry.SecureBoot,
			IPXESB:     entry.IPXESB,
			ShimIPXE:   entry.ShimIPXE,
			GRUBSB:     entry.GRUBSB,
			ShimGRUB:   entry.ShimGRUB,
		})
	}
	slices.SortFunc(entries, func(a, b ArchEntryResponse) int {
		return a.ArchCode - b.ArchCode
	})
	OK(w, ArchMapResponse{Entries: entries})
}

func (h *SettingsHandler) UpdateArchMap(w http.ResponseWriter, r *http.Request) {
	var req ArchMapResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	// 保存旧值用于审计对比
	h.mu.Lock()
	oldArchMap := make(map[int]config.ArchEntry, len(h.cfg.Boot.ArchMap))
	for k, v := range h.cfg.Boot.ArchMap {
		oldArchMap[k] = v
	}
	h.mu.Unlock()

	archMap := make(map[int]config.ArchEntry, len(req.Entries))
	for _, e := range req.Entries {
		archMap[e.ArchCode] = config.ArchEntry{
			NBP:        e.NBP,
			ChainLoad:  e.ChainLoad,
			IPXE:       e.IPXE,
			PXELinux:   e.PXELinux,
			GRUB:       e.GRUB,
			GRUBConfig: e.GRUBConfig,
			SecureBoot: e.SecureBoot,
			IPXESB:     e.IPXESB,
			ShimIPXE:   e.ShimIPXE,
			GRUBSB:     e.GRUBSB,
			ShimGRUB:   e.ShimGRUB,
		}
	}

	h.mu.Lock()
	h.cfg.Boot.ArchMap = archMap
	boot.InitArchMap(archMap)
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	// 构建变更详情
	var changes []string
	for code, newEntry := range archMap {
		old, existed := oldArchMap[code]
		if !existed {
			changes = append(changes, fmt.Sprintf("%s(%d): 新增", boot.ArchName(code), code))
			continue
		}
		var diffs []string
		if old.NBP != newEntry.NBP {
			diffs = append(diffs, fmt.Sprintf("NBP: %s→%s", old.NBP, newEntry.NBP))
		}
		if old.ChainLoad != newEntry.ChainLoad {
			diffs = append(diffs, fmt.Sprintf("链式加载: %t→%t", old.ChainLoad, newEntry.ChainLoad))
		}
		if old.IPXE != newEntry.IPXE {
			diffs = append(diffs, fmt.Sprintf("iPXE: %s→%s", old.IPXE, newEntry.IPXE))
		}
		if old.PXELinux != newEntry.PXELinux {
			diffs = append(diffs, fmt.Sprintf("PXELinux: %s→%s", old.PXELinux, newEntry.PXELinux))
		}
		if old.GRUB != newEntry.GRUB {
			diffs = append(diffs, fmt.Sprintf("GRUB2: %s→%s", old.GRUB, newEntry.GRUB))
		}
		if old.SecureBoot != newEntry.SecureBoot {
			diffs = append(diffs, fmt.Sprintf("SecureBoot: %t→%t", old.SecureBoot, newEntry.SecureBoot))
		}
		if len(diffs) > 0 {
			changes = append(changes, fmt.Sprintf("%s(%d): %s", boot.ArchName(code), code, strings.Join(diffs, ", ")))
		}
	}
	// 检查被删除的架构
	for code := range oldArchMap {
		if _, ok := archMap[code]; !ok {
			changes = append(changes, fmt.Sprintf("%s(%d): 已移除", boot.ArchName(code), code))
		}
	}

	detail := "更新引导配置"
	if len(changes) > 0 {
		detail = strings.Join(changes, "; ")
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), detail)
	OK(w, map[string]string{"status": "saved"})
}

func (h *SettingsHandler) GetArchMapDefaults(w http.ResponseWriter, _ *http.Request) {
	defs := boot.DefaultArchMap()
	entries := make([]ArchEntryResponse, 0, len(defs))
	for code := range defs {
		entry := defs[code]
		entries = append(entries, ArchEntryResponse{
			ArchCode:   code,
			ArchName:   boot.ArchName(code),
			NBP:        entry.NBP,
			ChainLoad:  entry.ChainLoad,
			IPXE:       entry.IPXE,
			PXELinux:   entry.PXELinux,
			GRUB:       entry.GRUB,
			GRUBConfig: entry.GRUBConfig,
			SecureBoot: entry.SecureBoot,
			IPXESB:     entry.IPXESB,
			ShimIPXE:   entry.ShimIPXE,
			GRUBSB:     entry.GRUBSB,
			ShimGRUB:   entry.ShimGRUB,
		})
	}
	slices.SortFunc(entries, func(a, b ArchEntryResponse) int {
		return a.ArchCode - b.ArchCode
	})
	OK(w, ArchMapResponse{Entries: entries})
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
			dhcpMode = "server"
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

	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), "更新 DHCP 配置")
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

	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), "更新 DNS 配置")
	OK(w, map[string]string{"status": "saved"})
}

// ── NFS ──

func (h *SettingsHandler) GetNFS(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg

	// Get live connection stats if callback available
	var connStats map[string]NFSConnectionInfo
	if h.getNFSConnections != nil {
		connStats = h.getNFSConnections()
	}

	// Check if NFS service is running
	running := false
	if h.isServiceRunning != nil {
		running = h.isServiceRunning("nfs")
	}

	mps := make([]NFSMountPointResponse, len(cfg.NFS.MountPoints))
	for i, mp := range cfg.NFS.MountPoints {
		resp := NFSMountPointResponse{
			Label:      mp.Label,
			ExportPath: mp.ExportPath,
			LocalDir:   mp.LocalDir,
			ReadOnly:   mp.ReadOnly,
			AllowIPs:   mp.AllowIPs,
		}
		if connStats != nil {
			if stat, ok := connStats[mp.ExportPath]; ok {
				resp.ConnectionCount = stat.Connections
				resp.Clients = stat.Clients
			}
		}
		mps[i] = resp
	}
	OK(w, NFSSettingsResponse{
		Enabled:     cfg.NFS.Enabled,
		Running:     running,
		Port:        cfg.NFS.Port,
		RpcbindPort: 111,
		Version:     "NFSv3",
		MountPoints: mps,
	})
}

func (h *SettingsHandler) UpdateNFS(w http.ResponseWriter, r *http.Request) {
	var req NFSSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	h.mu.Lock()
	h.cfg.NFS.Enabled = req.Enabled
	if req.Port > 0 {
		h.cfg.NFS.Port = req.Port
	}

	// 转换挂载点配置
	mps := make([]config.NFSMountPoint, len(req.MountPoints))
	for i, mp := range req.MountPoints {
		mps[i] = config.NFSMountPoint{
			Label:      mp.Label,
			ExportPath: mp.ExportPath,
			LocalDir:   mp.LocalDir,
			ReadOnly:   mp.ReadOnly,
			AllowIPs:   mp.AllowIPs,
		}
	}
	h.cfg.NFS.MountPoints = mps
	h.mu.Unlock()

	if h.setNFSMountPoints != nil {
		h.setNFSMountPoints(mps)
	}

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), "更新 NFS 配置")
	OK(w, map[string]string{"status": "saved"})
}

// ValidateNFSPath checks if a local directory exists and is accessible
func (h *SettingsHandler) ValidateNFSPath(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Path == "" {
		Error(w, http.StatusBadRequest, "路径不能为空")
		return
	}

	result := map[string]interface{}{
		"exists":  false,
		"writable": false,
	}

	info, err := os.Stat(req.Path)
	if err != nil {
		result["error"] = err.Error()
		OK(w, result)
		return
	}

	result["exists"] = true
	result["is_dir"] = info.IsDir()

	// Test write access by attempting to create a temp file
	if info.IsDir() {
		tmpFile := filepath.Join(req.Path, ".pxelab_write_test")
		f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
		if err == nil {
			f.Close()
			os.Remove(tmpFile)
			result["writable"] = true
		}
	}

	OK(w, result)
}

// BrowseNFSPath lists directories at a given path for the directory browser UI
func (h *SettingsHandler) BrowseNFSPath(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")

	// On Windows with empty path, return drive letters as root-level entries
	if dirPath == "" {
		if runtime.GOOS == "windows" {
			drives := listWindowsDrives()
			type DirEntry struct {
				Name  string `json:"name"`
				Path  string `json:"path"`
				IsDir bool   `json:"is_dir"`
			}
			var entries []DirEntry
			for _, d := range drives {
				entries = append(entries, DirEntry{
					Name:  d,
					Path:  d,
					IsDir: true,
				})
			}
			OK(w, map[string]interface{}{
				"current": "",
				"entries": entries,
			})
			return
		}
		dirPath = "/"
	}

	info, err := os.Stat(dirPath)
	if err != nil || !info.IsDir() {
		Error(w, http.StatusBadRequest, "路径不存在或不是目录")
		return
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		Error(w, http.StatusInternalServerError, "无法读取目录: "+err.Error())
		return
	}

	type DirEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
	}
	var dirs []DirEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Skip hidden directories
		if len(name) > 0 && name[0] == '.' {
			continue
		}
		dirs = append(dirs, DirEntry{
			Name:  name,
			Path:  filepath.Join(dirPath, name),
			IsDir: true,
		})
	}

	OK(w, map[string]interface{}{
		"current": dirPath,
		"entries": dirs,
	})
}

// listWindowsDrives returns available drive letters on Windows (e.g. ["C:\\", "D:\\"])
func listWindowsDrives() []string {
	var drives []string
	for _, letter := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(letter) + ":\\"
		if _, err := os.Stat(path); err == nil {
			drives = append(drives, path)
		}
	}
	return drives
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

	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), "更新网络引导配置")
	OK(w, map[string]string{"status": "saved"})
}

// ── iPXE Script (DHCP Option 175) ──

func (h *SettingsHandler) GetIPXEScript(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg

	resp := IPXEScriptSettingsResponse{
		Enabled:      cfg.IPXEScript.Enabled,
		Port:         cfg.IPXEScript.Port,
		Path:         cfg.IPXEScript.Path,
		FeatureFlags: cfg.IPXEScript.FeatureFlags,
	}

	// 设置默认值
	if resp.Port <= 0 {
		resp.Port = config.DefaultPortHTTP
	}
	if resp.Path == "" {
		resp.Path = "/boot/ipxe/script"
	}
	if resp.FeatureFlags <= 0 {
		resp.FeatureFlags = 0x01
	}

	OK(w, resp)
}

func (h *SettingsHandler) UpdateIPXEScript(w http.ResponseWriter, r *http.Request) {
	var req IPXEScriptSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	// 参数校验
	if req.Port < 0 {
		req.Port = 0
	}
	if req.FeatureFlags < 0 {
		req.FeatureFlags = 0
	}

	// 保存旧值用于审计对比
	h.mu.Lock()
	oldEnabled := h.cfg.IPXEScript.Enabled
	oldPort := h.cfg.IPXEScript.Port
	oldPath := h.cfg.IPXEScript.Path
	oldFlags := h.cfg.IPXEScript.FeatureFlags

	h.cfg.IPXEScript.Enabled = req.Enabled
	if req.Port > 0 {
		h.cfg.IPXEScript.Port = req.Port
	}
	if req.Path != "" {
		h.cfg.IPXEScript.Path = req.Path
	}
	if req.FeatureFlags > 0 {
		h.cfg.IPXEScript.FeatureFlags = req.FeatureFlags
	}
	h.mu.Unlock()

	if err := saveConfig(configPath(h.cfg), h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	// 构建变更详情
	var changes []string
	if oldEnabled != req.Enabled {
		changes = append(changes, fmt.Sprintf("启用: %t→%t", oldEnabled, req.Enabled))
	}
	if oldPort > 0 && oldPort != req.Port {
		changes = append(changes, fmt.Sprintf("端口: %d→%d", oldPort, req.Port))
	}
	if oldPath != "" && oldPath != req.Path {
		changes = append(changes, fmt.Sprintf("路径: %s→%s", oldPath, req.Path))
	}
	if oldFlags > 0 && oldFlags != req.FeatureFlags {
		changes = append(changes, fmt.Sprintf("特征标志: 0x%x→0x%x", oldFlags, req.FeatureFlags))
	}

	detail := "更新 iPXE 脚本配置"
	if len(changes) > 0 {
		detail = strings.Join(changes, "; ")
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), detail)
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
		dd = ".pxelab"
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

// ── 日志管理设置 ──

type LoggingSettingsResponse struct {
	MaxSizeMB       int  `json:"max_size_mb"`
	MaxBackups      int  `json:"max_backups"`
	MaxAgeDays      int  `json:"max_age_days"`
	Compress        bool `json:"compress"`
	CleanupInterval int  `json:"cleanup_interval"`
}

func (h *SettingsHandler) GetLoggingSettings(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	cfg := h.cfg
	h.mu.Unlock()

	resp := LoggingSettingsResponse{
		MaxSizeMB:       cfg.Log.MaxSizeMB,
		MaxBackups:      cfg.Log.MaxBackups,
		MaxAgeDays:      cfg.Log.MaxAgeDays,
		Compress:        cfg.Log.Compress,
		CleanupInterval: cfg.Log.CleanupInterval,
	}
	OK(w, resp)
}

func (h *SettingsHandler) UpdateLoggingSettings(w http.ResponseWriter, r *http.Request) {
	var req LoggingSettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	// 参数校验
	if req.MaxSizeMB < 0 {
		req.MaxSizeMB = 0
	}
	if req.MaxBackups < 0 {
		req.MaxBackups = 0
	}
	if req.MaxAgeDays < 0 {
		req.MaxAgeDays = 0
	}
	if req.CleanupInterval < 0 {
		req.CleanupInterval = 0
	}

	h.mu.Lock()
	h.cfg.Log.MaxSizeMB = req.MaxSizeMB
	h.cfg.Log.MaxBackups = req.MaxBackups
	h.cfg.Log.MaxAgeDays = req.MaxAgeDays
	h.cfg.Log.Compress = req.Compress
	h.cfg.Log.CleanupInterval = req.CleanupInterval
	cfg := h.cfg
	h.mu.Unlock()

	if err := saveConfig(configPath(cfg), cfg); err != nil {
		slog.Error("保存日志配置失败", "service", "HTTP", "error", err)
		Error(w, http.StatusInternalServerError, "保存配置失败")
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "settings", "", remoteIP(r), "更新日志管理配置")
	OK(w, map[string]any{"message": "日志设置已保存，轮转参数将在服务重启后生效"})
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
