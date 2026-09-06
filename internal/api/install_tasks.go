package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
)

type InstallTaskHandler struct {
	store      store.Interface
	serverBase string // 可被局域网访问的 HTTP 基址（host:port），用于注入应答钩子
	cfg        *config.Config
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
	OK(w, task)
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
func (h *InstallTaskHandler) GetTaskByMAC(w http.ResponseWriter, r *http.Request) {
	mac := chi.URLParam(r, "mac")
	task, err := h.store.GetInstallTaskByHostMAC(r.Context(), mac)
	if err != nil {
		Error(w, http.StatusNotFound, "未找到安装任务")
		return
	}

	// Enrich with overlay answer_param info
	overlay, _ := h.store.GetNetbootOverlay(r.Context(), task.DistroName)
	taskInfo := buildBootTaskInfo(task, overlay, r.Host)

	// PXE 引导留痕归因：该 MAC 正在走某发行版的无人值守安装
	_ = h.store.UpsertPxeBootRecord(r.Context(), mac, "", "install-task:"+task.DistroName, remoteIP(r))

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

	// 勾选了“自动下发初始化基线”时，注入拉取并执行的钩子（身份按配置取 mac/sn）
	if tmpl.EnableBaselinePull {
		mac, sn := "", ""
		if host != nil && host.ID != "" && host.MAC != "" {
			mac = host.MAC
			sn = host.SN
		} else if real, e := h.store.GetHost(r.Context(), task.HostID); e == nil && real != nil {
			mac = real.MAC
			sn = real.SN
		}
		base := chooseServerBase(h.serverBase, r.Host)
		if augmented, ok := augmentBaselinePull(rendered, tmpl.Type, base, mac, sn, h.cfg.Global.BaselineHooks); ok {
			rendered = augmented
		} else {
			slog.Warn("应答模板启用了基线自动下发，但无法安全注入（请人工添加钩子）",
				"template_type", tmpl.Type, "template_id", tmpl.ID)
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// buildBootTaskInfo constructs the task info needed for boot line injection.
func buildBootTaskInfo(task *models.InstallTask, overlay *models.NetbootOverlay, serverAddr string) *netboot.BootTaskInfo {
	if overlay == nil {
		return &netboot.BootTaskInfo{
			ID:           task.ID,
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

	answerURL := fmt.Sprintf("http://%s/api/v1/netboot/answer/%s", serverAddr, task.ID)

	return &netboot.BootTaskInfo{
		ID:           task.ID,
		AnswerURL:    answerURL,
		AnswerParam:  answerParam,
		AnswerType:   answerType,
		ExtraCmdline: task.ExtraCmdline,
	}
}
