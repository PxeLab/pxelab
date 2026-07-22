package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

func validateDNSRecord(rec *models.DNSRecord) string {
	if rec.Name == "" || rec.Type == "" || rec.Value == "" {
		return "名称、类型、值不能为空"
	}
	switch rec.Type {
	case "A":
		if _, err := netip.ParseAddr(rec.Value); err != nil || strings.Contains(rec.Value, ":") {
			return "A 记录的值必须为合法的 IPv4 地址"
		}
	case "AAAA":
		if addr, err := netip.ParseAddr(rec.Value); err != nil || !addr.Is6() {
			return "AAAA 记录的值必须为合法的 IPv6 地址"
		}
	case "CNAME":
		if strings.Contains(rec.Value, " ") {
			return "CNAME 记录的值不能包含空格"
		}
		labels := strings.Split(rec.Value, ".")
		if len(labels) < 2 {
			return "CNAME 记录的值必须为合法的域名（如 server.example.com）"
		}
		for _, label := range labels {
			if label == "" {
				return "CNAME 记录的值包含空的域名标签（如连续的点）"
			}
		}
	case "MX":
		parts := strings.SplitN(rec.Value, " ", 2)
		if len(parts) != 2 {
			return "MX 记录的值格式为「优先级 域名」，例如「10 mail.example.com」"
		}
		if prio, err := strconv.Atoi(parts[0]); err != nil || prio < 0 || prio > 65535 {
			return "MX 优先级必须在 0-65535 之间"
		}
		if !strings.Contains(parts[1], ".") {
			return "MX 记录的目标必须为合法的域名"
		}
	case "TXT":
		// any non-empty value is fine
	default:
		return "不支持的记录类型: " + rec.Type
	}
	return ""
}

type DNSRecordHandler struct {
	store       store.Interface
	localDomain string
}

func (h *DNSRecordHandler) List(w http.ResponseWriter, r *http.Request) {
	records, err := h.store.ListDNSRecords(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := map[string]any{"records": records}
	if h.localDomain != "" {
		resp["local_domain"] = h.localDomain
	}
	OK(w, resp)
}

func (h *DNSRecordHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	record, err := h.store.GetDNSRecord(r.Context(), uint(id))
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	OK(w, record)
}

func (h *DNSRecordHandler) Create(w http.ResponseWriter, r *http.Request) {
	var rec models.DNSRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := validateDNSRecord(&rec); msg != "" {
		Error(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.store.CreateDNSRecord(r.Context(), &rec); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "dns_record", rec.Name, remoteIP(r), "新建 DNS 记录: "+rec.Type)
	Created(w, rec)
}

func (h *DNSRecordHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	
	// 保存旧值
	oldRec, _ := h.store.GetDNSRecord(r.Context(), uint(id))

	var rec models.DNSRecord
	if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
		Error(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if msg := validateDNSRecord(&rec); msg != "" {
		Error(w, http.StatusBadRequest, msg)
		return
	}
	rec.ID = uint(id)
	if err := h.store.UpdateDNSRecord(r.Context(), &rec); err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	// 构建变更详情
	var changes []string
	if oldRec != nil {
		if oldRec.Name != rec.Name {
			changes = append(changes, fmt.Sprintf("名称: %s→%s", oldRec.Name, rec.Name))
		}
		if oldRec.Type != rec.Type {
			changes = append(changes, fmt.Sprintf("类型: %s→%s", oldRec.Type, rec.Type))
		}
		if oldRec.Value != rec.Value {
			changes = append(changes, fmt.Sprintf("值: %s→%s", oldRec.Value, rec.Value))
		}
		if oldRec.TTL != rec.TTL {
			changes = append(changes, fmt.Sprintf("TTL: %d→%d", oldRec.TTL, rec.TTL))
		}
		if oldRec.Enabled != rec.Enabled {
			changes = append(changes, fmt.Sprintf("启用: %t→%t", oldRec.Enabled, rec.Enabled))
		}
		if oldRec.Subnet != rec.Subnet {
			changes = append(changes, fmt.Sprintf("子网: %s→%s", oldRec.Subnet, rec.Subnet))
		}
	}
	detail := "更新 DNS 记录"
	if len(changes) > 0 {
		detail = strings.Join(changes, "; ")
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "dns_record", rec.Name, remoteIP(r), detail)
	OK(w, rec)
}

func (h *DNSRecordHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		Error(w, http.StatusBadRequest, "无效的 ID")
		return
	}
	// 保存旧值用于审计
	oldRec, _ := h.store.GetDNSRecord(r.Context(), uint(id))
	if err := h.store.DeleteDNSRecord(r.Context(), uint(id)); err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}
	detail := "删除 DNS 记录"
	if oldRec != nil && oldRec.Name != "" {
		detail = fmt.Sprintf("删除 DNS 记录 %s (%s: %s)", oldRec.Name, oldRec.Type, oldRec.Value)
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "dns_record", fmt.Sprintf("%d", id), remoteIP(r), detail)
	w.WriteHeader(http.StatusNoContent)
}
