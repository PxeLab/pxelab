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
	"github.com/pxelab/pxelab/internal/drivers"
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
	eventBus   *eventbus.Bus        // 任务状态流转到 done/failed 时发布 webhook 通知事件（R3），可为 nil
	bootFS     *boot.BootFileServer // 驱动包目录快照来源（R6），可为 nil（不注入驱动）
	netbootMgr *netboot.Manager     // catalog answer_param 回落来源，可为 nil
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
	taskInfo := netboot.BuildBootTaskInfo(task, overlay, r.Host)

	// 应答类型兜底：overlay 未配置 answer_type 时用模板 type，使 iPXE 菜单注入的
	// 文件名（install.bat/winpeshl.ini vs autounattend.xml）与应答内容一致。
	if taskInfo.AnswerType == "" && task.AnswerTemplateID != nil {
		if tmpl, err := h.store.GetAnswerTemplate(r.Context(), *task.AnswerTemplateID); err == nil && tmpl != nil {
			taskInfo.AnswerType = tmpl.Type
		}
	}

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

	// 借鉴项 6：winpeshl Recipe B 预设参数化 —— SMB 服务器取渲染基址（与引导
	// 下载同源），共享名取 smb.shares 首个（默认 isos），索引取任务的 WIMIndex。
	fillWinpeInstallParams(&data, chooseServerBase(h.serverBase, r.Host), h.cfg, task.WIMIndex)

	rendered, err := netboot.RenderAnswerTemplate(tmpl.Content, data)
	if err != nil {
		Error(w, http.StatusInternalServerError, "模板渲染失败: "+err.Error())
		return
	}

	// winpeshl 模式下，模板内容作为 install.bat 返回；winpeshl.ini 由后端
	// 生成一个引用 install.bat 的包装，通过 ?file=winpeshl.ini 访问，
	// 避免与 install.bat 返回同一内容。
	//
	// 关键路径：wimboot 把每个具名 initrd 注入到 WinPE 内存盘的
	// %SYSTEMROOT%\System32（= X:\Windows\System32），install.bat 与
	// winpeshl.ini 都落在该目录；winpeshl.exe 也从该目录读取 winpeshl.ini。
	// 因此 AppPath 必须指向 %SYSTEMROOT%\System32\install.bat —— 旧值
	// X:\install.bat 指向的是内存盘根目录，那里并没有这个文件。
	// 用 [LaunchApps]（官方 iPXE wimboot/WinPE 例程同样用法）：它按顺序运行
	// 列出的项，适合由一个 .bat 脚本接管 WinPE 启动流程。
	isWinpeShellIni := r.URL.Query().Get("file") == "winpeshl.ini"
	if isWinpeShellIni {
		rendered = "[LaunchApps]\r\n" +
			"%SYSTEMROOT%\\System32\\install.bat\r\n"
	}

	// 勾选了“自动下发初始化基线”时，注入拉取并执行的钩子（身份按配置取 mac/sn）。
	// R6：主机绑定了驱动包且模板为 autounattend 时，首登编排先装驱动、再拉基线。
	// B2：主机未显式绑定时继承所属 Profile 的默认驱动包。
	driverPacks := h.resolveDriverPacks(r.Context(), host)

	// 借用项 2：winpeshl 路线在 PE 阶段（setup.exe 之前）运行时加载驱动，覆盖
	// boot-critical 场景（存储/网卡）。命中条件：winpeshl 模式（模板 type=winpeshl，
	// 或 overlay 版本 answer_type=winpeshl，或本次即 winpeshl.ini 请求）且主机绑定
	// 了驱动包。驱动块必须写进 install.bat（无 file 参数的那次请求）；winpeshl.ini
	// 请求只返回 INI 包装，不注入。
	// 统一解析“实际应答类型”（overlay 版本 answer_type 优先、模板 type 兜底）：
	// PE 阶段注入与首登注入都必须以它为准，否则两条路径判定不一致会产出畸形
	// 应答文件（issue #3）。本次请求即 winpeshl.ini 包装时强制按 winpeshl 处理。
	overlayType := h.resolveTaskAnswerType(r.Context(), task)
	if overlayType != "" && tmpl.Type != "" && overlayType != tmpl.Type {
		slog.Warn("应答类型配置冲突：overlay answer_type 与模板 type 不一致，以 overlay 为准",
			"task_id", task.ID, "template_id", tmpl.ID,
			"template_type", tmpl.Type, "overlay_answer_type", overlayType)
	}
	effectiveType := netboot.ResolveAnswerType(overlayType, tmpl.Type)
	if isWinpeShellIni {
		effectiveType = "winpeshl"
	}
	winpeShellMode := effectiveType == "winpeshl"
	if winpeShellMode && !isWinpeShellIni && len(driverPacks) > 0 {
		base := chooseServerBase(h.serverBase, r.Host)
		switch drivers.NormalizeDelivery(h.cfg.Netboot.DriverDelivery) {
		case drivers.DeliveryInitrd:
			// initrd 直投：驱动文件已由 iPXE 注入 %SYSTEMROOT%\System32（见
			// internal/httpd 菜单生成），这里只插入「落地 + pnputil」块。两侧共用
			// drivers.PlanInjections，注入文件名必然一致。
			// 幂等：模板内容若已含注入标记则跳过（与 http 分支一致）。
			if strings.Contains(rendered, drivers.PullMarker) {
				break
			}
			if h.bootFS != nil {
				injections, errs := drivers.PlanInjections(h.bootFS.Root(), driverPacks, task.Arch)
				for _, e := range errs {
					slog.Warn("winpeshl initrd 直投跳过驱动包", "error", e)
				}
				if block := drivers.PeInstallBlockInitrd(injections); block != "" {
					rendered = insertAfterEchoOff(rendered, block)
				}
			} else {
				slog.Warn("initrd 直投需要 boot 文件服务，已跳过驱动注入", "task_id", task.ID)
			}
		default:
			if injected, ok := h.injectWinpeShellDrivers(rendered, driverPacks, task.Arch, base); ok {
				rendered = injected
			}
		}
	}

	wantDrivers := effectiveType == "autounattend" && len(driverPacks) > 0
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
		if effectiveType == "autounattend" {
			var driverCmds []driverCommand
			if wantDrivers {
				driverCmds = h.buildDriverCommands(base, driverPacks, task.Arch)
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
			if augmented, ok := augmentBaselinePull(rendered, effectiveType, base, mac, sn, h.cfg.Global.BaselineHooks); ok {
				rendered = augmented
			} else {
				slog.Warn("应答模板启用了基线自动下发，但无法安全注入（请人工添加钩子）",
					"template_type", effectiveType, "template_id", tmpl.ID)
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(rendered))
}

// resolveDriverPacks 返回主机在装机时应生效的驱动包列表（B2）：
//   - 主机显式绑定了驱动包 → 用主机自己的（主机级绑定优先）；
//   - 否则继承所属 Profile 的默认驱动包；
//   - 两者皆无 / 解析或查询失败 → nil（不注入驱动）。
//
// ResolveDriverPacks 返回主机在装机时应生效的驱动包（B2）：主机级绑定优先，
// 否则继承所属 Profile 的默认驱动包；两者皆空 / 查询失败返回 nil。
// 导出供 internal/httpd 生成 iPXE initrd 直投菜单复用，保证两侧同源。
func ResolveDriverPacks(ctx context.Context, st store.Interface, host *models.Host) []string {
	if host == nil {
		return nil
	}
	hostPacks, _ := host.GetDriverPacks()
	var profPacks []string
	if host.ProfileID != nil && *host.ProfileID != "" {
		if prof, err := st.GetProfile(ctx, *host.ProfileID); err == nil && prof != nil {
			profPacks, _ = prof.GetDriverPacks()
		}
	}
	return drivers.EffectivePacks(hostPacks, profPacks)
}

func (h *InstallTaskHandler) resolveDriverPacks(ctx context.Context, host *models.Host) []string {
	return ResolveDriverPacks(ctx, h.store, host)
}

// resolveTaskAnswerType 解析该安装任务在引导行实际使用的应答类型，与
// netboot.BuildBootTaskInfo / GetTaskByMAC 同源：任务发行版 overlay 中匹配该版本的
// 版本覆盖里的 answer_type（如 "winpeshl" / "autounattend"）。无 overlay 或
// 未配置时返回 ""。供 GetAnswerFile 判定是否走 winpeshl 的 PE 阶段驱动注入。
func (h *InstallTaskHandler) resolveTaskAnswerType(ctx context.Context, task *models.InstallTask) string {
	overlay, err := h.store.GetNetbootOverlay(ctx, task.DistroName)
	if err != nil || overlay == nil {
		return ""
	}
	ovs, _ := overlay.GetVersionOverrides()
	for _, ov := range ovs {
		if ov.Codename == task.VersionCodename && ov.AnswerType != "" {
			return ov.AnswerType
		}
	}
	return ""
}
