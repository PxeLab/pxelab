package api

import (
	"net"
	"net/http"
	"strings"
)

type InterfaceInfo struct {
	Name string   `json:"name"`
	MAC  string   `json:"mac"`
	IPs  []string `json:"ips"`
	Up   bool     `json:"up"`
}

func (h *Handler) ListInterfaces(w http.ResponseWriter, r *http.Request) {
	ifaces, err := net.Interfaces()
	if err != nil {
		Error(w, http.StatusInternalServerError, "获取网卡列表失败")
		return
	}

	var result []InterfaceInfo
	for _, iface := range ifaces {
		// 跳过 loopback 和没有运行中的接口（保留 up 状态供前端判断）
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		var ips []string
		for _, addr := range addrs {
			ip := addr.String()
			if idx := strings.Index(ip, "/"); idx != -1 {
				ip = ip[:idx]
			}
			ips = append(ips, ip)
		}

		result = append(result, InterfaceInfo{
			Name: iface.Name,
			MAC:  iface.HardwareAddr.String(),
			IPs:  ips,
			Up:   iface.Flags&net.FlagUp != 0,
		})
	}

	OK(w, result)
}
