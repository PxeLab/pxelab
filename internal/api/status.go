package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/pxelab/pxelab/internal/servicemanager"
)

var startTime = time.Now()

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{
		"Netboot": "enabled",
	}

	if h.svcController != nil {
		// Aggregate real service states
		dhcpRunning := false
		proxyRunning := false
		tftpRunning := false
		httpRunning := false
		dnsRunning := false

		for _, svc := range h.svcController.List() {
			status := "stopped"
			switch svc.Status {
			case servicemanager.StatusRunning:
				status = "running"
			case servicemanager.StatusError:
				status = "error"
			}
			services[svc.Name] = status

			// Aggregate by type for backward-compat dashboard
			name := svc.Name
			if strings.HasPrefix(name, "dhcp/") {
				if svc.Status == servicemanager.StatusRunning {
					dhcpRunning = true
				}
			} else if strings.HasPrefix(name, "proxy/") {
				if svc.Status == servicemanager.StatusRunning {
					proxyRunning = true
				}
			} else {
				switch name {
				case "tftp":
					tftpRunning = svc.Status == servicemanager.StatusRunning
				case "http":
					httpRunning = svc.Status == servicemanager.StatusRunning
				case "dns":
					dnsRunning = svc.Status == servicemanager.StatusRunning
				}
			}
		}

		if dhcpRunning {
			services["DHCP"] = "running"
		}
		if proxyRunning {
			services["ProxyDHCP"] = "running"
		}
		if tftpRunning {
			services["TFTP"] = "running"
		}
		if httpRunning {
			services["HTTP"] = "running"
		}
		if dnsRunning {
			services["DNS"] = "running"
		}
	}

	OK(w, map[string]any{
		"status":   "ok",
		"version":  "0.1.0",
		"uptime":   int(time.Since(startTime).Seconds()),
		"services": services,
	})
}
