package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/store"
)

type LeaseHandler struct {
	store  store.Interface
	config *config.Config
}

type LeaseStats struct {
	SubnetID   string  `json:"subnet_id"`
	DHCPMode   string  `json:"dhcp_mode"`
	PoolSize   int     `json:"pool_size"`
	Allocated  int     `json:"allocated"`
	Available  int     `json:"available"`
	UsagePct   float64 `json:"usage_pct"`
	ActiveOnly int     `json:"active_only"`
}

func (h *LeaseHandler) List(w http.ResponseWriter, r *http.Request) {
	leases, err := h.store.ListLeases(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, leases)
}

func (h *LeaseHandler) Stats(w http.ResponseWriter, r *http.Request) {
	leases, err := h.store.ListLeases(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	type subnetCount struct {
		total  int
		active int
	}
	leaseCounts := make(map[string]*subnetCount)
	for i := range leases {
		sid := leases[i].SubnetID
		if leaseCounts[sid] == nil {
			leaseCounts[sid] = &subnetCount{}
		}
		leaseCounts[sid].total++
		if time.Now().Before(leases[i].ExpiresAt) {
			leaseCounts[sid].active++
		}
	}

	var stats []LeaseStats

	for _, iface := range h.config.Interfaces {
		for _, sn := range iface.Subnets {
			if sn.DHCP == "off" {
				continue
			}
			poolSize := 0
			pools := sn.Pools
			if len(pools) == 0 && sn.Pool != "" {
				pools = []string{sn.Pool}
			}
			for _, p := range pools {
				ps, err := poolIPCount(p)
				if err != nil {
					log.Printf("address pool stat failed: %v", err)
				}
				poolSize += ps
			}

			cnt := leaseCounts[sn.CIDR]
			allocated := 0
			activeOnly := 0
			if cnt != nil {
				allocated = cnt.total
				activeOnly = cnt.active
			}

			if poolSize == 0 {
				stats = append(stats, LeaseStats{
					SubnetID:   sn.CIDR,
					DHCPMode:   sn.DHCP,
					PoolSize:   0,
					Allocated:  activeOnly,
					Available:  0,
					UsagePct:   0,
					ActiveOnly: activeOnly,
				})
			} else {
				if allocated > poolSize {
					allocated = poolSize
				}
				stats = append(stats, LeaseStats{
					SubnetID:   sn.CIDR,
					DHCPMode:   sn.DHCP,
					PoolSize:   poolSize,
					Allocated:  allocated,
					Available:  poolSize - allocated,
					UsagePct:   float64(allocated) / float64(poolSize) * 100,
					ActiveOnly: activeOnly,
				})
			}
		}
	}

	OK(w, stats)
}

func (h *LeaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	if mac == "" || !macRe.MatchString(mac) {
		Error(w, http.StatusBadRequest, "invalid MAC address")
		return
	}
	if err := h.store.DeleteLease(r.Context(), mac); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]string{"message": "lease deleted"})
}

type BatchDeleteRequest struct {
	MACs []string `json:"macs"`
}

func (h *LeaseHandler) BatchDelete(w http.ResponseWriter, r *http.Request) {
	var req BatchDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.MACs) == 0 {
		Error(w, http.StatusBadRequest, "MAC list is empty")
		return
	}

	var failed []string
	for _, mac := range req.MACs {
		if !macRe.MatchString(mac) {
			failed = append(failed, mac+"(invalid format)")
			continue
		}
		if err := h.store.DeleteLease(r.Context(), mac); err != nil {
			failed = append(failed, mac)
		}
	}


	result := map[string]interface{}{
		"success_count": len(req.MACs) - len(failed),
		"failed_count":  len(failed),
	}
	if len(failed) > 0 {
		result["failed_macs"] = failed
		result["message"] = fmt.Sprintf("%d succeeded, %d failed", len(req.MACs)-len(failed), len(failed))
	} else {
		result["message"] = fmt.Sprintf("%d leases deleted", len(req.MACs))
	}
	OK(w, result)
}

func (h *LeaseHandler) Prune(w http.ResponseWriter, r *http.Request) {
	if err := h.store.PruneLeases(r.Context()); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]string{"message": "expired leases pruned"})
}

func poolIPCount(pool string) (int, error) {
	parts := strings.SplitN(pool, "-", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid pool format: %s", pool)
	}
	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	start := net.ParseIP(startStr)
	if start == nil {
		return 0, fmt.Errorf("invalid start IP: %s", startStr)
	}
	start = start.To4()
	if start == nil {
		return 0, fmt.Errorf("non-IPv4 start: %s", startStr)
	}

	var end net.IP
	if strings.Contains(endStr, ".") {
		end = net.ParseIP(endStr)
	} else {
		end = net.IPv4(start[0], start[1], start[2], 0)
		var last byte
		if _, err := fmt.Sscanf(endStr, "%d", &last); err != nil {
			return 0, fmt.Errorf("invalid shorthand end IP: %s", endStr)
		}
		end[3] = last
	}
	if end == nil {
		return 0, fmt.Errorf("invalid end IP: %s", endStr)
	}
	end = end.To4()
	if end == nil {
		return 0, fmt.Errorf("non-IPv4 end: %s", endStr)
	}

	diff := int(end[3])-int(start[3]) +
		(int(end[2])-int(start[2]))*256 +
		(int(end[1])-int(start[1]))*65536 +
		(int(end[0])-int(start[0]))*16777216

	if diff < 0 {
		return 0, fmt.Errorf("end IP before start IP: %s", pool)
	}

	return diff + 1, nil
}
