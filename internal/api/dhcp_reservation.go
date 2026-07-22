package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

type DHCPReservationHandler struct {
	store store.Interface
}

func (h *DHCPReservationHandler) List(w http.ResponseWriter, r *http.Request) {
	subnetCIDR := r.URL.Query().Get("subnet_cidr")
	reservations, err := h.store.ListDHCPReservations(r.Context(), subnetCIDR)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"reservations": reservations})
}

func (h *DHCPReservationHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	reservation, err := h.store.GetDHCPReservation(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	OK(w, reservation)
}

func (h *DHCPReservationHandler) Create(w http.ResponseWriter, r *http.Request) {
	var res models.DHCPReservation
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if res.IP == "" {
		Error(w, http.StatusBadRequest, "IP 地址不能为空")
		return
	}
	if err := h.checkIPConflict(r, &res, 0); err != nil {
		Error(w, http.StatusConflict, err.Error())
		return
	}
	if err := h.store.CreateDHCPReservation(r.Context(), &res); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "dhcp_reservation", fmt.Sprintf("%d", res.ID), remoteIP(r), "新建 DHCP 预留: "+res.IP+" → "+res.MAC)
	Created(w, res)
}

func (h *DHCPReservationHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}

	// 保存旧值
	oldRes, _ := h.store.GetDHCPReservation(r.Context(), uint(id))

	var res models.DHCPReservation
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	res.ID = uint(id)
	if err := h.checkIPConflict(r, &res, uint(id)); err != nil {
		Error(w, http.StatusConflict, err.Error())
		return
	}
	if err := h.store.UpdateDHCPReservation(r.Context(), &res); err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	// 构建变更详情
	var changes []string
	if oldRes != nil {
		if oldRes.IP != res.IP {
			changes = append(changes, fmt.Sprintf("IP: %s→%s", oldRes.IP, res.IP))
		}
		if oldRes.MAC != res.MAC {
			changes = append(changes, fmt.Sprintf("MAC: %s→%s", oldRes.MAC, res.MAC))
		}
		if oldRes.Hostname != res.Hostname {
			changes = append(changes, fmt.Sprintf("主机名: %s→%s", oldRes.Hostname, res.Hostname))
		}
		if oldRes.SubnetCIDR != res.SubnetCIDR {
			changes = append(changes, fmt.Sprintf("子网: %s→%s", oldRes.SubnetCIDR, res.SubnetCIDR))
		}
	}
	detail := "更新 DHCP 预留: " + res.IP + " → " + res.MAC
	if len(changes) > 0 {
		detail = strings.Join(changes, "; ")
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "dhcp_reservation", fmt.Sprintf("%d", id), remoteIP(r), detail)
	OK(w, res)
}

// checkIPConflict 检查 IP 是否已被其他预留绑定或有活跃租约
func (h *DHCPReservationHandler) checkIPConflict(r *http.Request, res *models.DHCPReservation, excludeID uint) error {
	if res.SubnetCIDR == "" || res.IP == "" {
		return nil
	}
	// 检查是否已被其他预留占用（遍历列表避免 GORM 列名映射问题）
	all, listErr := h.store.ListDHCPReservations(r.Context(), res.SubnetCIDR)
	if listErr == nil {
		for _, existing := range all {
			if existing.IP == res.IP && existing.ID != excludeID {
				msg := "IP " + res.IP + " 已被预留"
				if existing.MAC != "" {
					msg += "（MAC: " + existing.MAC + "）"
				}
				return &apiError{msg}
			}
		}
	}
	// 检查是否有活跃租约
	leases, err := h.store.ListLeases(r.Context())
	if err == nil {
		for _, l := range leases {
			if l.IP == res.IP && time.Now().Before(l.ExpiresAt) {
				return &apiError{"IP " + res.IP + " 当前有活跃租约"}
			}
		}
	}
	return nil
}

// apiError 用于返回业务错误消息
type apiError struct{ msg string }

func (e *apiError) Error() string { return e.msg }

func (h *DHCPReservationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	if err := h.store.DeleteDHCPReservation(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "dhcp_reservation", fmt.Sprintf("%d", id), remoteIP(r), "删除 DHCP 预留")
	w.WriteHeader(http.StatusNoContent)
}
