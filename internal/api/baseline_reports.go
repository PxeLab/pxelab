package api

// Baseline 执行回执（R2）：装机后基线脚本逐条上报结果，平台侧存库 + 主机详情页展示。
//
//   POST /api/v1/baselines/report            — 无鉴权，按 mac/sn 定位主机（resolveHost）
//   GET  /api/v1/hosts/{id}/baseline-reports — 鉴权区，返回该主机回执列表（倒序）

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/notify"
)

const (
	// baselineOutputTailLimit output_tail 最大保留字节数（4KB，保留尾部）
	baselineOutputTailLimit = 4096
	// baselineReportKeep 每个主机滚动保留的回执条数
	baselineReportKeep = 100
)

// baselineReportReq 是回执接口的请求体。
type baselineReportReq struct {
	ScriptName string `json:"script_name"`
	Seq        int    `json:"seq"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	OutputTail string `json:"output_tail"`
}

// baselineReportDTO 是回执的 API 响应结构。
type baselineReportDTO struct {
	ID         uint   `json:"id"`
	HostID     string `json:"host_id"`
	ScriptName string `json:"script_name"`
	Seq        int    `json:"seq"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	OutputTail string `json:"output_tail"`
	CreatedAt  string `json:"created_at"`
}

// truncateTail 把 output 截断到 limit 字节以内（保留尾部，按 UTF-8 边界对齐）。
func truncateTail(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	tail := s[len(s)-limit:]
	// 丢弃截断点落在一个多字节字符中间产生的 UTF-8 续字节
	for len(tail) > 0 && (tail[0]&0xC0) == 0x80 {
		tail = tail[1:]
	}
	return tail
}

// Report 接收基线脚本执行回执（无鉴权，按 mac/sn 定位主机）。
// 主机不存在 → 204 静默丢弃，不阻断装机流程。
func (h *BaselineHandler) Report(w http.ResponseWriter, r *http.Request) {
	host, found := h.resolveHost(r)
	if !found {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	var req baselineReportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	rep := &models.BaselineReport{
		HostID:     host.ID,
		ScriptName: req.ScriptName,
		Seq:        req.Seq,
		ExitCode:   req.ExitCode,
		DurationMs: req.DurationMs,
		OutputTail: truncateTail(req.OutputTail, baselineOutputTailLimit),
	}
	if err := h.store.CreateBaselineReport(r.Context(), rep); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 按主机滚动清理，只留最近 N 条（清理失败不影响回执写入）
	_ = h.store.PruneBaselineReports(r.Context(), host.ID, baselineReportKeep)
	// 脚本执行失败 → 发布 webhook 通知事件（R3，异步不阻塞回执）
	if rep.ExitCode != 0 && h.eventBus != nil {
		h.eventBus.PublishAsync(notify.TopicNotify, notify.Event{
			Event: notify.EventBaselineScriptFailed,
			Time:  time.Now(),
			Host:  notify.HostInfo{MAC: host.MAC, Name: host.Name, IP: host.IP},
			Detail: fmt.Sprintf("脚本 %s（第 %d 条）退出码 %d%s",
				rep.ScriptName, rep.Seq, rep.ExitCode, tailSnippet(rep.OutputTail)),
		})
	}
	Created(w, baselineReportToDTO(rep))
}

// tailSnippet 截取输出尾部一行作为事件详情摘要。
func tailSnippet(tail string) string {
	line := tail
	if idx := strings.LastIndex(strings.TrimRight(tail, "\n"), "\n"); idx >= 0 {
		line = strings.TrimRight(tail, "\n")[idx+1:]
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if len([]rune(line)) > 80 {
		line = string([]rune(line)[:80]) + "…"
	}
	return "，输出: " + line
}

// ListReports 返回主机的基线执行回执列表（倒序）。
// GET /hosts/{id}/baseline-reports
func (h *BaselineHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")
	reports, err := h.store.ListBaselineReports(r.Context(), hostID)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	dtos := make([]baselineReportDTO, 0, len(reports))
	for i := range reports {
		dtos = append(dtos, baselineReportToDTO(&reports[i]))
	}
	OK(w, map[string]any{"reports": dtos})
}

func baselineReportToDTO(r *models.BaselineReport) baselineReportDTO {
	return baselineReportDTO{
		ID:         r.ID,
		HostID:     r.HostID,
		ScriptName: r.ScriptName,
		Seq:        r.Seq,
		ExitCode:   r.ExitCode,
		DurationMs: r.DurationMs,
		OutputTail: r.OutputTail,
		CreatedAt:  r.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
