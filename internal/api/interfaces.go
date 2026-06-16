package api

import (
	"net"
	"net/http"
)

type InterfaceInfo struct {
	Name string   `json:"name"`
	MAC  string   `json:"mac"`
	IPv4 []string `json:"ipv4"`
	IPv6 []string `json:"ipv6"`
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
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		var v4s, v6s []string
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.String()
			if ipNet.IP.To4() != nil {
				v4s = append(v4s, ip)
			} else {
				v6s = append(v6s, ip)
			}
		}

		result = append(result, InterfaceInfo{
			Name: iface.Name,
			MAC:  iface.HardwareAddr.String(),
			IPv4: v4s,
			IPv6: v6s,
			Up:   iface.Flags&net.FlagUp != 0,
		})
	}

	OK(w, result)
}
