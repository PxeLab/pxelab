package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pxego/pxego/internal/config"
	"gopkg.in/yaml.v3"
)

type SettingsHandler struct {
	cfg     *config.Config
	cfgFile string
}

func NewSettingsHandler(cfg *config.Config) *SettingsHandler {
	return &SettingsHandler{cfg: cfg}
}

type SettingsResponse struct {
	Server   ServerSettings   `json:"server"`
	DHCP     DHCPSettings     `json:"dhcp"`
	TFTP     TFTPSettings     `json:"tftp"`
	DNS      DNSSettings      `json:"dns"`
	HTTP     HTTPSettings     `json:"http"`
	IPMI     IPMISettings     `json:"ipmi"`
	LogLevel string           `json:"log_level"`
	DataDir  string           `json:"data_dir"`
}

type ServerSettings struct {
	Name    string `json:"name"`
	AppMode bool   `json:"app_mode"`
	Token   string `json:"token"`
}

type DHCPSettings struct {
	Enabled   bool   `json:"enabled"`
	Range     string `json:"range"`
	Gateway   string `json:"gateway"`
	Subnet    string `json:"subnet"`
	LeaseTime int    `json:"lease_time"`
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
	Port     int    `json:"port"`
	BootDir  string `json:"boot_dir"`
}

type IPMISettings struct {
	Enabled bool `json:"enabled"`
	Timeout int  `json:"timeout"`
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := h.cfg
	resp := SettingsResponse{
		LogLevel: cfg.Log.Level,
		DataDir:  cfg.Global.DataDir,
		Server: ServerSettings{
			AppMode: cfg.Global.AppMode,
			Token:   cfg.Auth.Token,
		},
		TFTP: TFTPSettings{
			Root: cfg.Boot.RootDir,
		},
	}

	if len(cfg.Interfaces) > 0 {
		iface := cfg.Interfaces[0]
		resp.TFTP.Enabled = iface.TFTP
		resp.HTTP.BootDir = cfg.Boot.RootDir
		resp.DNS.Enabled = iface.DNS

		if len(iface.Subnets) > 0 {
			sn := iface.Subnets[0]
			resp.DHCP.Enabled = iface.DHCP != "" && iface.DHCP != "off"
			resp.DHCP.Range = sn.Pool
			resp.DHCP.Gateway = sn.Gateway
			resp.DHCP.LeaseTime = sn.LeaseTime
			resp.DHCP.DNSServers = sn.DNSServers
		}
	}

	OK(w, resp)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req SettingsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	h.cfg.Log.Level = req.LogLevel
	h.cfg.Global.AppMode = req.Server.AppMode
	h.cfg.Auth.Token = req.Server.Token

	if len(h.cfg.Interfaces) > 0 {
		if len(h.cfg.Interfaces[0].Subnets) > 0 {
			sn := &h.cfg.Interfaces[0].Subnets[0]
			if req.DHCP.Range != "" {
				sn.Pool = req.DHCP.Range
			}
			if req.DHCP.Gateway != "" {
				sn.Gateway = req.DHCP.Gateway
			}
			if req.DHCP.LeaseTime > 0 {
				sn.LeaseTime = req.DHCP.LeaseTime
			}
		}
	}

	cfgPath := filepath.Join(h.cfg.Global.DataDir, "config.yaml")
	if err := saveConfig(cfgPath, h.cfg); err != nil {
		Error(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
		return
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
