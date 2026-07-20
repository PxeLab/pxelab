package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
)

type InstallTaskHandler struct {
	store store.Interface
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
	RecordAudit(r.Context(), h.store, models.AuditCreate, "install_task", task.ID, remoteIP(r), task.DistroName)
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
	RecordAudit(r.Context(), h.store, models.AuditUpdate, "install_task", id, remoteIP(r), task.DistroName)
	OK(w, task)
}

func (h *InstallTaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteInstallTask(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	RecordAudit(r.Context(), h.store, models.AuditDelete, "install_task", id, remoteIP(r), "")
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
