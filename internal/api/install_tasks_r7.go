package api

// R7 批量装机 + 装机任务最小状态回报通道。
//
//	POST /api/v1/install-tasks/batch                      — 批量创建（共享 batch_id，活跃任务机器跳过）
//	POST /api/v1/install-tasks/batch/{batch_id}/cancel        — 批次全部取消
//	POST /api/v1/install-tasks/batch/{batch_id}/retry-failed  — 批次失败重试
//	POST /api/v1/install-tasks/{id}/retry                 — 单机失败解锁重试（failed→pending）
//	POST /api/v1/install-tasks/{id}/report                — 匿名状态回报（按 task_id）
//	POST /api/v1/install-tasks/report-by-mac              — 匿名状态回报（按 mac/sn 反查活跃任务）

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxelab/pxelab/internal/models"
)

// installReportReq 装机状态回报请求体（最小状态通道）。
type installReportReq struct {
	Status  string `json:"status"` // done | failed
	Message string `json:"message"`
}

// applyReport 把回报落到任务上：仅活跃任务（pending/installing）可流转；
// 已终态任务幂等返回当前状态，不重复发事件。
func (h *InstallTaskHandler) applyReport(w http.ResponseWriter, r *http.Request, task *models.InstallTask) {
	var req installReportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if req.Status != "done" && req.Status != "failed" {
		Error(w, http.StatusBadRequest, "status 仅支持 done / failed")
		return
	}
	if task.Status != "pending" && task.Status != "installing" {
		OK(w, task)
		return
	}
	oldStatus := task.Status
	task.Status = req.Status
	if req.Status == "failed" {
		task.ErrorMsg = req.Message
	}
	if err := h.store.UpdateInstallTask(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "install_task", task.ID, remoteIP(r),
		fmt.Sprintf("装机回报: %s→%s", oldStatus, req.Status))
	h.publishStatusEvent(r.Context(), oldStatus, task)
	OK(w, task)
}

// Report 接收安装器/脚本按 task_id 的状态回报（无鉴权，PXE 运行时端点）。
func (h *InstallTaskHandler) Report(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.store.GetInstallTask(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "任务未找到")
		return
	}
	h.applyReport(w, r, task)
}

// ReportByMAC 按 mac/sn 反查该主机的当前活跃任务并回报（无鉴权）。
// 供基线聚合脚本（pull.sh/pull.ps1）在装机末尾自动回报"装机完成"：
// 脚本只持有 mac/sn 身份而不知道 task_id。主机不存在 → 204 静默丢弃，
// 与 /baselines/report 一致，不阻断装机流程。
func (h *InstallTaskHandler) ReportByMAC(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var host *models.Host
	if mac := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("mac"))); mac != "" {
		host, _ = h.store.GetHostByMAC(ctx, mac)
	}
	if host == nil {
		if sn := strings.TrimSpace(r.URL.Query().Get("sn")); sn != "" {
			host, _ = h.store.GetHostBySN(ctx, sn)
		}
	}
	if host == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	tasks, err := h.store.ListInstallTasks(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// List 按 created_at 倒序，首个匹配即最新活跃任务（单机单活跃任务约束）
	for i := range tasks {
		if tasks[i].HostID == host.ID && (tasks[i].Status == "pending" || tasks[i].Status == "installing") {
			h.applyReport(w, r, &tasks[i])
			return
		}
	}
	Error(w, http.StatusNotFound, "该主机无活跃装机任务")
}

// Retry 单机失败解锁：failed→pending（人工确认后重试，FR-7.5）。
func (h *InstallTaskHandler) Retry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.store.GetInstallTask(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "任务未找到")
		return
	}
	if task.Status != "failed" {
		Error(w, http.StatusBadRequest, "仅失败任务可重试")
		return
	}
	task.Status = "pending"
	task.ErrorMsg = ""
	if err := h.store.UpdateInstallTask(r.Context(), task); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "install_task", id, remoteIP(r), "重试安装任务（失败解锁）")
	OK(w, task)
}

// ── 批次操作 ──

type batchHostReq struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
	SN   string `json:"sn"`
	IP   string `json:"ip"`
}

type batchCreateReq struct {
	Hosts            []batchHostReq `json:"hosts"`
	DistroName       string         `json:"distro_name"`
	VersionCodename  string         `json:"version_codename"`
	Arch             string         `json:"arch"`
	AnswerTemplateID *uint          `json:"answer_template_id"`
	ExtraCmdline     string         `json:"extra_cmdline"`
	ProfileID        string         `json:"profile_id"` // 可选：逐台绑定引导配置（对齐向导单机编排行为）
}

type batchSkipped struct {
	MAC    string `json:"mac"`
	Reason string `json:"reason"`
}

// defaultTaskHostname 生成默认主机名：node-<mac 后 6 位>。
func defaultTaskHostname(mac string) string {
	compact := strings.NewReplacer(":", "", "-", "").Replace(mac)
	if len(compact) > 6 {
		compact = compact[len(compact)-6:]
	}
	return "node-" + compact
}

// BatchCreate 批量创建装机任务：逐台校验（MAC 格式、已有活跃任务的跳过并列明），
// 未建档机器自动建档 + 认领引导记录，成功任务共享一个 batch_id（FR-7.2）。
func (h *InstallTaskHandler) BatchCreate(w http.ResponseWriter, r *http.Request) {
	var req batchCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	if len(req.Hosts) == 0 {
		Error(w, http.StatusBadRequest, "机器列表不能为空")
		return
	}
	if req.DistroName == "" || req.VersionCodename == "" {
		Error(w, http.StatusBadRequest, "distro_name 和 version_codename 为必填项")
		return
	}
	ctx := r.Context()

	if req.ProfileID != "" {
		if _, err := h.store.GetProfile(ctx, req.ProfileID); err != nil {
			Error(w, http.StatusBadRequest, "指定的引导配置不存在")
			return
		}
	}

	allTasks, err := h.store.ListInstallTasks(ctx)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	activeByHost := map[string]bool{}
	for _, t := range allTasks {
		if t.Status == "pending" || t.Status == "installing" {
			activeByHost[t.HostID] = true
		}
	}

	batchID := "batch_" + uuid.New().String()
	var created []models.InstallTask
	var skipped []batchSkipped
	seen := map[string]bool{}

	for _, hreq := range req.Hosts {
		mac := strings.ToLower(strings.TrimSpace(hreq.MAC))
		if !macAddressRe.MatchString(mac) {
			skipped = append(skipped, batchSkipped{MAC: hreq.MAC, Reason: "MAC 地址格式不正确"})
			continue
		}
		if seen[mac] {
			skipped = append(skipped, batchSkipped{MAC: mac, Reason: "列表内重复"})
			continue
		}
		seen[mac] = true

		host, err := h.store.GetHostByMAC(ctx, mac)
		if err != nil || host == nil {
			// 未建档自动建档
			name := strings.TrimSpace(hreq.Name)
			if name == "" {
				name = defaultTaskHostname(mac)
			}
			host = &models.Host{
				ID:   uuid.New().String(),
				Name: name,
				MAC:  mac,
				SN:   strings.TrimSpace(hreq.SN),
				IP:   strings.TrimSpace(hreq.IP),
			}
			if err := h.store.CreateHost(ctx, host); err != nil {
				skipped = append(skipped, batchSkipped{MAC: mac, Reason: "建档失败: " + err.Error()})
				continue
			}
		}
		// 认领引导记录（无记录时忽略）
		_ = h.store.ClaimPxeBootRecord(ctx, mac, host.ID)

		if activeByHost[host.ID] {
			skipped = append(skipped, batchSkipped{MAC: mac, Reason: "已有活跃装机任务"})
			continue
		}

		// 可选：绑定引导配置（与向导单机编排一致）
		if req.ProfileID != "" && (host.ProfileID == nil || *host.ProfileID != req.ProfileID) {
			host.ProfileID = &req.ProfileID
			if err := h.store.UpdateHost(ctx, host); err != nil {
				skipped = append(skipped, batchSkipped{MAC: mac, Reason: "绑定引导配置失败: " + err.Error()})
				continue
			}
		}

		task := models.InstallTask{
			ID:               "task_" + uuid.New().String(),
			HostID:           host.ID,
			DistroName:       req.DistroName,
			VersionCodename:  req.VersionCodename,
			Arch:             req.Arch,
			AnswerTemplateID: req.AnswerTemplateID,
			ExtraCmdline:     req.ExtraCmdline,
			Status:           "pending",
			BatchID:          batchID,
		}
		if err := h.store.CreateInstallTask(ctx, &task); err != nil {
			skipped = append(skipped, batchSkipped{MAC: mac, Reason: "创建任务失败: " + err.Error()})
			continue
		}
		activeByHost[host.ID] = true
		created = append(created, task)
	}

	RecordAudit(ctx, h.store, models.AuditCreate, "install_task", batchID, remoteIP(r),
		fmt.Sprintf("批量创建安装任务: %s (%s/%s)，成功 %d 台，跳过 %d 台",
			req.DistroName, req.VersionCodename, req.Arch, len(created), len(skipped)))
	Created(w, map[string]any{
		"batch_id": batchID,
		"tasks":    created,
		"skipped":  skipped,
	})
}

// BatchCancel 批次全部取消：pending 任务直接删除（机器未进入安装，不产生失败锁定）；
// installing 任务置为 failed（机器可能装了一半，按失败锁定语义阻止自动重装）。
func (h *InstallTaskHandler) BatchCancel(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batch_id")
	tasks, err := h.store.ListInstallTasks(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	deleted, cancelled := 0, 0
	for i := range tasks {
		t := &tasks[i]
		if t.BatchID != batchID {
			continue
		}
		switch t.Status {
		case "pending":
			if err := h.store.DeleteInstallTask(r.Context(), t.ID); err == nil {
				deleted++
			}
		case "installing":
			oldStatus := t.Status
			t.Status = "failed"
			t.ErrorMsg = "批次已取消"
			if err := h.store.UpdateInstallTask(r.Context(), t); err == nil {
				cancelled++
				h.publishStatusEvent(r.Context(), oldStatus, t)
			}
		}
	}
	if deleted == 0 && cancelled == 0 {
		Error(w, http.StatusNotFound, "该批次没有可取消的任务")
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "install_task", batchID, remoteIP(r),
		fmt.Sprintf("批次取消: 删除待开始 %d，中止安装中 %d", deleted, cancelled))
	OK(w, map[string]any{"deleted": deleted, "cancelled": cancelled})
}

// BatchRetryFailed 批次失败重试：该批次内所有 failed 任务重置回 pending（逐台走失败解锁语义）。
func (h *InstallTaskHandler) BatchRetryFailed(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batch_id")
	tasks, err := h.store.ListInstallTasks(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	retried := 0
	for i := range tasks {
		t := &tasks[i]
		if t.BatchID != batchID || t.Status != "failed" {
			continue
		}
		t.Status = "pending"
		t.ErrorMsg = ""
		if err := h.store.UpdateInstallTask(r.Context(), t); err == nil {
			retried++
		}
	}
	if retried == 0 {
		Error(w, http.StatusNotFound, "该批次没有失败任务")
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "install_task", batchID, remoteIP(r),
		fmt.Sprintf("批次失败重试: %d 台重置为待开始", retried))
	OK(w, map[string]any{"retried": retried})
}
