package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
}

func NewSettingsHandler(cfg *config.Config, reloader SubnetReloader) *SettingsHandler {
	return &SettingsHandler{cfg: cfg, reloader: reloader}
}

type SettingsResponse struct {
	Server     ServerSettings      `json:"server"`
	DHCP       DHCPSettings        `json:"dhcp"`
	TFTP       TFTPSettings        `json:"tftp"`
	DNS        DNSSettings         `json:"dns"`
	HTTP       HTTPSettings        `json:"http"`
	IPMI       IPMISettings        `json:"ipmi"`
	LogLevel   string              `json:"log_level"`
	DataDir    string              `json:"data_dir"`
	Interfaces []InterfaceResponse `json:"interfaces"`
}

type ServerSettings struct {
	Name    string `json:"name"`
	AppMode bool   `json:"app_mode"`
	Token   string `json:"token"`
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
	Enabled  bool   `json:"enabled"`
	Port     int    `json:"port"`
	Upstream string `json:"upstream"`
}

type HTTPSettings struct {
	Port    int    `json:"port"`
	BootDir string `json:"boot_dir"`
}

type IPMISettings struct {
	Enabled bool `json:"enabled"`
	Timeout int  `json:"timeout"`
}

type InterfaceResponse struct {
	Name       string   `json:"name"`
	IP         string   `json:"ip"`
	DHCPMode   string   `json:"dhcp_mode"`
	Subnet     string   `json:"subnet"`
	Pools      []string `json:"pools"`
	Gateway    string   `json:"gateway"`
	DNSServers string   `json:"dns_servers"`
	LeaseTime  int      `json:"lease_time"`
	NextServer string   `json:"next_server"`
	TFTP       bool     `json:"tftp"`
	HTTP       bool     `json:"http"`
	DNS        bool     `json:"dns"`
}

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg

	if cfg.Auth.Token == "" {
		cfg.Auth.Token = generateToken()
		cfgPath := filepath.Join(cfg.Global.DataDir, "config.yaml")
		saveConfig(cfgPath, cfg)
	}
	if cfg.Global.ServerName == "" {
		cfg.Global.ServerName = "pxego"
	}

	resp := SettingsResponse{
		LogLevel: cfg.Log.Level,
		DataDir:  cfg.Global.DataDir,
		Server: ServerSettings{
			Name:    cfg.Global.ServerName,
			AppMode: cfg.Global.AppMode,
			Token:   cfg.Auth.Token,
		},
		DHCP: DHCPSettings{
			Enabled:    true,
			Range:      "",
			Gateway:    "",
			Subnet:     "",
			LeaseTime:  config.DefaultLeaseTime,
			DNSServers: "8.8.8.8",
		},
		TFTP: TFTPSettings{
			Enabled: true,
			Port:    config.DefaultPortTFTP,
			Root:    cfg.Boot.RootDir,
		},
		DNS: DNSSettings{
			Enabled:  false,
			Port:     config.DefaultPortDNS,
			Upstream: "8.8.8.8:53",
		},
		HTTP: HTTPSettings{
			Port:    config.DefaultPortHTTP,
			BootDir: cfg.Boot.RootDir,
		},
		IPMI: IPMISettings{
			Enabled: false,
			Timeout: 5,
		},
	}

	for _, iface := range cfg.Interfaces {
		ir := InterfaceResponse{
			Name:       iface.Name,
			IP:         iface.IP,
			DHCPMode:   iface.DHCP,
			TFTP:       iface.TFTP,
			HTTP:       iface.HTTP,
			DNS:        iface.DNS,
			LeaseTime:  config.DefaultLeaseTime,
			DNSServers: "8.8.8.8",
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
		}

		if iface.DHCP == "" || iface.DHCP == "off" {
			resp.DHCP.Enabled = false
		}

		resp.Interfaces = append(resp.Interfaces, ir)
	}

	if len(cfg.Interfaces) > 0 && len(cfg.Interfaces[0].Subnets) > 0 {
		sn := cfg.Interfaces[0].Subnets[0]
		resp.DHCP.Enabled = cfg.Interfaces[0].DHCP != "" && cfg.Interfaces[0].DHCP != "off"
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
		if ir.DHCPMode == "" || ir.DHCPMode == "off" || ir.DHCPMode == "proxy" {
			continue
		}
		for pi, p := range ir.Pools {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			parts := strings.SplitN(p, "-", 2)
			if len(parts) != 2 {
				Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d: 地址池 #%d 格式无效", i+1, pi+1))
				return
			}
			startIP := strings.TrimSpace(parts[0])
			endIP := strings.TrimSpace(parts[1])
			if ir.Subnet != "" {
				if !ipInCIDR(startIP, ir.Subnet) {
					Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d: 地址池 #%d 起始地址 %s 不属于子网 %s", i+1, pi+1, startIP, ir.Subnet))
					return
				}
				if !ipInCIDR(endIP, ir.Subnet) {
					Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d: 地址池 #%d 结束地址 %s 不属于子网 %s", i+1, pi+1, endIP, ir.Subnet))
					return
				}
			}
		}
		// 检测地址池冲突
		validPools := make([]string, 0, len(ir.Pools))
		for _, p := range ir.Pools {
			if strings.TrimSpace(p) != "" && strings.Contains(p, "-") {
				validPools = append(validPools, p)
			}
		}
		for pi := 0; pi < len(validPools); pi++ {
			for pj := pi + 1; pj < len(validPools); pj++ {
				if poolsOverlap(validPools[pi], validPools[pj]) {
					Error(w, http.StatusBadRequest, fmt.Sprintf("接口 #%d: 地址池 #%d 和 #%d 范围冲突", i+1, pi+1, pj+1))
					return
				}
			}
		}
	}

	h.cfg.Log.Level = req.LogLevel
	h.cfg.Global.ServerName = req.Server.Name
	h.cfg.Global.AppMode = req.Server.AppMode
	if req.Server.Token != "" {
		h.cfg.Auth.Token = req.Server.Token
	}

	if req.Interfaces != nil {
		h.cfg.Interfaces = make([]config.InterfaceConfig, 0, len(req.Interfaces))
		for _, ir := range req.Interfaces {
			iface := config.InterfaceConfig{
				Name: ir.Name,
				IP:   ir.IP,
				DHCP: ir.DHCPMode,
				TFTP: ir.TFTP,
				HTTP: ir.HTTP,
				DNS:  ir.DNS,
			}
			if ir.DHCPMode == "" {
				iface.DHCP = "full"
			}

			if ir.Subnet != "" || len(ir.Pools) > 0 || ir.Gateway != "" {
				iface.Subnets = []config.SubnetConfig{{
					CIDR:       ir.Subnet,
					Pools:      ir.Pools,
					Gateway:    ir.Gateway,
					DNSServers: ir.DNSServers,
					NextServer: ir.NextServer,
					LeaseTime:  ir.LeaseTime,
				}}
			}
			h.cfg.Interfaces = append(h.cfg.Interfaces, iface)
		}
	}

	cfgPath := filepath.Join(h.cfg.Global.DataDir, "config.yaml")
	if err := saveConfig(cfgPath, h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
	}

	if h.reloader != nil {
		h.reloader.ReloadSubnets()
	}

	OK(w, map[string]string{"status": "saved"})
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
