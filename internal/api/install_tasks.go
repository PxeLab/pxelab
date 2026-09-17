package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/notify"
	"github.com/pxelab/pxelab/internal/store"
)

type InstallTaskHandler struct {
	store      store.Interface
	serverBase string // 可被局域网访问的 HTTP 基址（host:port），用于注入应答钩子
	cfg        *config.Config
	eventBus   *eventbus.Bus          // 任务状态流转到 done/failed 时发布 webhook 通知事件（R3），可为 nil
	bootFS     *boot.BootFileServer   // 驱动包目录快照来源（R6），可为 nil（不注入驱动）
	netbootMgr *netboot.Manager       // catalog answer_param 回落来源，可为 nil
}

func (h *InstallTaskHandler) List(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.store.ListInstallTasks(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, map[string]any{"tasks": tasks})
}

func (h *InstallTaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var task models.InstallTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	task.ID = "task_" + uuid.New().String()
	task.Status = "pending"
	if err := h.store.CreateInstallTask(r.Context(), &task); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditCreate, "install_task", task.ID, remoteIP(r), "新建安装任务: "+task.DistroName+" ("+task.VersionCodename+"/"+task.Arch+")")
	Created(w, task)
}

func (h *InstallTaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.store.GetInstallTask(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "任务未找到")
		return
	}
	OK(w, task)
}

func (h *InstallTaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// 保存旧值
	oldTask, _ := h.store.GetInstallTask(r.Context(), id)

	var task models.InstallTask
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	task.ID = id
	if err := h.store.UpdateInstallTask(r.Context(), &task); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 构建变更详情
	var changes []string
	if oldTask != nil {
		if oldTask.DistroName != task.DistroName {
			changes = append(changes, fmt.Sprintf("发行版: %s→%s", oldTask.DistroName, task.DistroName))
		}
		if oldTask.VersionCodename != task.VersionCodename {
			changes = append(changes, fmt.Sprintf("版本: %s→%s", oldTask.VersionCodename, task.VersionCodename))
		}
		if oldTask.Arch != task.Arch {
			changes = append(changes, fmt.Sprintf("架构: %s→%s", oldTask.Arch, task.Arch))
		}
		if oldTask.Status != task.Status {
			changes = append(changes, fmt.Sprintf("状态: %s→%s", oldTask.Status, task.Status))
		}
		if oldTask.ExtraCmdline != task.ExtraCmdline {
			changes = append(changes, fmt.Sprintf("额外参数: %s→%s", oldTask.ExtraCmdline, task.ExtraCmdline))
		}
		if oldTask.ErrorMsg != task.ErrorMsg {
			changes = append(changes, fmt.Sprintf("错误信息: %s→%s", oldTask.ErrorMsg, task.ErrorMsg))
		}
	}
	detail := "更新安装任务: " + task.DistroName
	if len(changes) > 0 {
		detail = strings.Join(changes, "; ")
	}
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "install_task", id, remoteIP(r), detail)

	// 状态流转到 done/failed → 发布 webhook 通知事件（R3，异步不阻塞）
	if oldTask != nil {
		h.publishStatusEvent(r.Context(), oldTask.Status, &task)
	}
	OK(w, task)
}

// publishStatusEvent 是状态变更事件的统一路径（R3/R7）：
// 任务状态流转到 done/failed 时发布 webhook 通知事件（异步不阻塞）。
// Update / Report / ReportByMAC 都走它；oldStatus 与当前状态相同则不重复发布。
func (h *InstallTaskHandler) publishStatusEvent(ctx context.Context, oldStatus string, task *models.InstallTask) {
	if h.eventBus == nil || oldStatus == task.Status {
		return
	}
	var evtType string
	switch task.Status {
	case "done":
		evtType = notify.EventInstallFinished
	case "failed":
		evtType = notify.EventInstallFailed
	}
	if evtType == "" {
		return
	}
	info := notify.HostInfo{}
	if host, err := h.store.GetHost(ctx, task.HostID); err == nil && host != nil {
		info = notify.HostInfo{MAC: host.MAC, Name: host.Name, IP: host.IP}
	}
	evtDetail := fmt.Sprintf("%s (%s/%s)", task.DistroName, task.VersionCodename, task.Arch)
	if task.Status == "failed" && task.ErrorMsg != "" {
		evtDetail += ": " + task.ErrorMsg
	}
	h.eventBus.PublishAsync(notify.TopicNotify, notify.Event{
		Event:  evtType,
		Time:   time.Now(),
		Host:   info,
		Detail: evtDetail,
	})
}

func (h *InstallTaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// 保存旧值用于审计
	oldTask, _ := h.store.GetInstallTask(r.Context(), id)
	if err := h.store.DeleteInstallTask(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail := "删除安装任务"
	if oldTask != nil && oldTask.DistroName != "" {
		detail = fmt.Sprintf("删除安装任务 %s (%s)", oldTask.DistroName, oldTask.Status)
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "install_task", id, remoteIP(r), detail)
	w.WriteHeader(http.StatusNoContent)
}

// GetTaskByMAC is a PXE runtime endpoint (no auth required).
// It looks up an active install task for the host identified by MAC address.
// R7 失败锁定：以该主机"最新"任务为准——最新任务 failed 时不再下发安装引导
// （返回 404，引导层回落本地磁盘/菜单），须在任务详情页人工重试解锁。
func (h *InstallTaskHandler) GetTaskByMAC(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	task, err := h.store.GetLatestInstallTaskByHostMAC(r.Context(), mac)
	if err != nil {
		Error(w, http.StatusNotFound, "未找到安装任务")
		return
	}
	serve, locked := netboot.TaskBootDecision(task.Status)
	if locked {
		Error(w, http.StatusNotFound, "安装任务已失败锁定，请人工重试解锁")
		return
	}
	if !serve {
		Error(w, http.StatusNotFound, "未找到安装任务")
		return
	}

	// Enrich with overlay answer_param info
	overlay, _ := h.store.GetNetbootOverlay(r.Context(), task.DistroName)
	taskInfo := buildBootTaskInfo(task, overlay, r.Host)

	// answer_param 回落：overlay 未配置时采用 catalog 版本自带的 answer_param
	// （如商店导入的 anaconda 系条目 inst.ks={{.AnswerURL}}），让无人值守
	// 应答注入开箱即用，无需手工配 overlay。
	if taskInfo.AnswerParam == "" && h.netbootMgr != nil {
		if d := h.netbootMgr.GetDistro(task.DistroName); d != nil {
			for _, v := range d.Versions {
				if v.Codename == task.VersionCodename && v.AnswerParam != "" {
					taskInfo.AnswerParam = v.AnswerParam
					break
				}
			}
		}
	}

	// PXE 引导留痕归因：该 MAC 正在走某发行版的无人值守安装
	_, _ = h.store.UpsertPxeBootRecord(r.Context(), mac, "", "install-task:"+task.DistroName, remoteIP(r))

	OK(w, taskInfo)
}

// GetAnswerFile is a PXE runtime endpoint (no auth required).
// It renders and returns the answer file content for a given task.
func (h *InstallTaskHandler) GetAnswerFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "task_id")
	task, err := h.store.GetInstallTask(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "任务未找到")
		return
	}

	if task.AnswerTemplateID == nil {
		Error(w, http.StatusNotFound, "该任务未关联应答模板")
		return
	}

	tmpl, err := h.store.GetAnswerTemplate(r.Context(), *task.AnswerTemplateID)
	if err != nil {
		Error(w, http.StatusNotFound, "应答模板未找到")
		return
	}

	// R7 状态通道：安装器首次拉取应答文件即视为进入安装。
	// 幂等：仅 pending→installing，重复拉取/其他状态不回写。
	if task.Status == "pending" {
		task.Status = "installing"
		if err := h.store.UpdateInstallTask(r.Context(), task); err != nil {
			slog.Warn("装机任务状态置为 installing 失败", "task_id", task.ID, "error", err)
		}
	}

	// Try to get host info for template rendering
	host, err := h.store.GetHost(r.Context(), task.HostID)
	if err != nil || host == nil {
		host = &models.Host{Name: task.HostID}
	}

	data := netboot.AnswerDataFromHost(host, task.Arch)

	rendered, err := netboot.RenderAnswerTemplate(tmpl.Content, data)
	if err != nil {
		Error(w, http.StatusInternalServerError, "模板渲染失败: "+err.Error())
		return
	}

	// winpeshl 模式下，模板内容作为 install.bat 返回；winpeshl.ini 由后端
	// 生成一个引用 X:\install.bat 的包装，通过 ?file=winpeshl.ini 访问，
	// 避免与 install.bat 返回同一内容。
	if r.URL.Query().Get("file") == "winpeshl.ini" {
		rendered = "[LaunchApp]\n" +
			"AppPath = X:\\install.bat\n"
	}

	// 勾选了“自动下发初始化基线”时，注入拉取并执行的钩子（身份按配置取 mac/sn）。
	// R6：主机绑定了驱动包且模板为 autounattend 时，首登编排先装驱动、再拉基线。
	driverPacks, _ := host.GetDriverPacks()
	wantDrivers := tmpl.Type == "autounattend" && len(driverPacks) > 0
	if tmpl.EnableBaselinePull || wantDrivers {
		mac, sn := "", ""
		if host != nil && host.ID != "" && host.MAC != "" {
			mac = host.MAC
			sn = host.SN
		} else if real, e := h.store.GetHost(r.Context(), task.HostID); e == nil && real != nil {
			mac = real.MAC
			sn = real.SN
		}
		base := chooseServerBase(h.serverBase, r.Host)
		if tmpl.Type == "autounattend" {
			var driverCmds []driverCommand
			if wantDrivers {
				driverCmds = h.buildDriverCommands(base, driverPacks)
				if len(driverCmds) == 0 {
					slog.Warn("主机绑定的驱动包均不可用，跳过驱动注入",
						"template_id", tmpl.ID, "host_id", task.HostID)
				}
			}
			// 无 mac/sn 身份时基线无法安全注入（沿用旧行为），驱动注入不依赖身份
			psURL := ""
			if mac != "" || sn != "" {
				parts := []string{}
				if mac != "" {
					parts = append(parts, "mac="+mac)
				}
				if sn != "" {
					parts = append(parts, "sn="+sn)
				}
				psURL = buildPullURL(base, "ps1", strings.Join(parts, "&"))
			}
			wantBaseline := tmpl.EnableBaselinePull && psURL != ""
			if augmented, ok := augmentAutoUnattendFirstLogon(rendered, psURL, driverCmds, wantBaseline, h.cfg.Global.BaselineHooks); ok {
				rendered = augmented
			} else {
				slog.Warn("autounattend 首登编排无法安全注入（请人工添加钩子）",
					"template_type", tmpl.Type, "template_id", tmpl.ID,
					"baseline_pull", tmpl.EnableBaselinePull, "driver_cmds", len(driverCmds))
			}
		} else if tmpl.EnableBaselinePull {
			if augmented, ok := augmentBaselinePull(rendered, tmpl.Type, base, mac, sn, h.cfg.Global.BaselineHooks); ok {
				rendered = augmented
			} else {
				slog.Warn("应答模板启用了基线自动下发，但无法安全注入（请人工添加钩子）",
					"template_type", tmpl.Type, "template_id", tmpl.ID)
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// buildBootTaskInfo constructs the task info needed for boot line injection.
// AnswerURL is always computed so catalog-level answer_param (fallback when
// the overlay has none) can inject the answer file URL as well.
func buildBootTaskInfo(task *models.InstallTask, overlay *models.NetbootOverlay, serverAddr string) *netboot.BootTaskInfo {
	answerURL := fmt.Sprintf("http://%s/api/v1/netboot/answer/%s", serverAddr, task.ID)

	if overlay == nil {
		return &netboot.BootTaskInfo{
			ID:           task.ID,
			AnswerURL:    answerURL,
			ExtraCmdline: task.ExtraCmdline,
		}
	}

	// Find matching version override for answer_param
	ovs, _ := overlay.GetVersionOverrides()
	answerParam := ""
	answerType := ""
	for _, ov := range ovs {
		if ov.Codename == task.VersionCodename && ov.AnswerParam != "" {
			answerParam = ov.AnswerParam
			answerType = ov.AnswerType
		}
	}

	return &netboot.BootTaskInfo{
		ID:           task.ID,
		AnswerURL:    answerURL,
		AnswerParam:  answerParam,
		AnswerType:   answerType,
		ExtraCmdline: task.ExtraCmdline,
	}
}
