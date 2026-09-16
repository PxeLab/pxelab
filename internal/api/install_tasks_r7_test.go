package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/notify"
	"github.com/pxelab/pxelab/internal/store"
)

func seedHost(t *testing.T, st store.Interface, id, mac string) *models.Host {
	t.Helper()
	host := &models.Host{ID: id, Name: "node-" + id, MAC: mac, IP: "192.168.1.10"}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatalf("seed host: %v", err)
	}
	return host
}

func seedTask(t *testing.T, st store.Interface, id, hostID, status, batchID string) *models.InstallTask {
	t.Helper()
	task := &models.InstallTask{
		ID:              id,
		HostID:          hostID,
		DistroName:      "Rocky",
		VersionCodename: "9",
		Arch:            "x86_64",
		Status:          status,
		BatchID:         batchID,
	}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	// 保证按 created_at 排序时可区分先后
	time.Sleep(time.Millisecond)
	return task
}

// ── 批量创建 ──

func TestBatchCreateSkipsActiveAndInvalid(t *testing.T) {
	h, st := newInstallTaskHandler(t)
	busy := seedHost(t, st, "host-busy", "00:11:22:33:44:01")
	seedTask(t, st, "task_active", busy.ID, "installing", "")
	seedHost(t, st, "host-free", "00:11:22:33:44:02")

	body := `{
		"hosts": [
			{"mac":"00:11:22:33:44:01"},
			{"mac":"00:11:22:33:44:02","name":"node-free"},
			{"mac":"AA:BB:CC:DD:EE:FF","ip":"192.168.1.99"},
			{"mac":"not-a-mac"},
			{"mac":"00:11:22:33:44:02"}
		],
		"distro_name":"Rocky","version_codename":"9","arch":"x86_64","extra_cmdline":"quiet"
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/install-tasks/batch", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.BatchCreate(w, req)
	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	var resp struct {
		Data struct {
			BatchID string `json:"batch_id"`
			Tasks   []struct {
				ID      string `json:"id"`
				HostID  string `json:"host_id"`
				Status  string `json:"status"`
				BatchID string `json:"batch_id"`
			} `json:"tasks"`
			Skipped []struct {
				MAC    string `json:"mac"`
				Reason string `json:"reason"`
			} `json:"skipped"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Result().Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.BatchID == "" {
		t.Fatal("batch_id 不能为空")
	}
	if len(resp.Data.Tasks) != 2 {
		t.Fatalf("expected 2 created tasks, got %d", len(resp.Data.Tasks))
	}
	for _, task := range resp.Data.Tasks {
		if task.BatchID != resp.Data.BatchID || task.Status != "pending" {
			t.Fatalf("task missing batch_id/pending: %+v", task)
		}
	}
	if len(resp.Data.Skipped) != 3 {
		t.Fatalf("expected 3 skipped, got %d: %+v", len(resp.Data.Skipped), resp.Data.Skipped)
	}
	// 未建档 MAC 自动建档 + 默认主机名
	autoHost, err := st.GetHostByMAC(t.Context(), "aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatal("未建档 MAC 应自动建档")
	}
	if autoHost.Name != "node-ddeeff" || autoHost.IP != "192.168.1.99" {
		t.Fatalf("unexpected auto-registered host: %+v", autoHost)
	}
	// 有活跃任务的机器仍是原来的一个任务
	tasks, _ := st.ListInstallTasks(t.Context())
	activeForBusy := 0
	for _, tk := range tasks {
		if tk.HostID == busy.ID && (tk.Status == "pending" || tk.Status == "installing") {
			activeForBusy++
		}
	}
	if activeForBusy != 1 {
		t.Fatalf("已有活跃任务的机器不应新建任务, active=%d", activeForBusy)
	}
}

// ── 批次取消 / 失败重试 ──

func TestBatchCancelAndRetryFailed(t *testing.T) {
	h, st := newInstallTaskHandler(t)
	host := seedHost(t, st, "host-1", "00:11:22:33:44:10")
	seedTask(t, st, "task_p", host.ID, "pending", "batch_1")
	seedTask(t, st, "task_i", host.ID, "installing", "batch_1")
	seedTask(t, st, "task_d", host.ID, "done", "batch_1")
	seedTask(t, st, "task_other", host.ID, "pending", "batch_2")

	// cancel batch_1：pending 删除、installing→failed、done 不动、其他批次不动
	req := httptest.NewRequest(http.MethodPost, "/api/v1/install-tasks/batch/batch_1/cancel", nil)
	req = withChiParams(req, "batch_id", "batch_1")
	w := httptest.NewRecorder()
	h.BatchCancel(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("cancel: expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	if _, err := st.GetInstallTask(t.Context(), "task_p"); err == nil {
		t.Fatal("pending 任务应被删除")
	}
	ti, _ := st.GetInstallTask(t.Context(), "task_i")
	if ti.Status != "failed" || ti.ErrorMsg == "" {
		t.Fatalf("installing 任务应置为 failed: %+v", ti)
	}
	td, _ := st.GetInstallTask(t.Context(), "task_d")
	if td.Status != "done" {
		t.Fatal("done 任务不应受影响")
	}
	if _, err := st.GetInstallTask(t.Context(), "task_other"); err != nil {
		t.Fatal("其他批次任务不应受影响")
	}

	// retry-failed batch_1：task_i 回到 pending
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/install-tasks/batch/batch_1/retry-failed", nil)
	req2 = withChiParams(req2, "batch_id", "batch_1")
	w2 := httptest.NewRecorder()
	h.BatchRetryFailed(w2, req2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatalf("retry-failed: expected 200, got %d: %s", w2.Result().StatusCode, w2.Body.String())
	}
	ti, _ = st.GetInstallTask(t.Context(), "task_i")
	if ti.Status != "pending" || ti.ErrorMsg != "" {
		t.Fatalf("失败任务应重置为 pending: %+v", ti)
	}

	// 没有失败任务可重试 → 404
	w3 := httptest.NewRecorder()
	h.BatchRetryFailed(w3, httptest.NewRequest(http.MethodPost, "/", nil))
	// 无 batch_id 参数不匹配任何任务
	if w3.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("empty retry: expected 404, got %d", w3.Result().StatusCode)
	}
}

// ── 单机重试（失败解锁）──

func TestRetrySingleTask(t *testing.T) {
	h, st := newInstallTaskHandler(t)
	host := seedHost(t, st, "host-1", "00:11:22:33:44:20")
	seedTask(t, st, "task_f", host.ID, "failed", "")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/install-tasks/task_f/retry", nil)
	req = withChiParams(req, "id", "task_f")
	w := httptest.NewRecorder()
	h.Retry(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("retry: expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	task, _ := st.GetInstallTask(t.Context(), "task_f")
	if task.Status != "pending" {
		t.Fatalf("failed 任务应重置为 pending, got %s", task.Status)
	}

	// 非 failed 任务 → 400
	w2 := httptest.NewRecorder()
	h.Retry(w2, withChiParams(httptest.NewRequest(http.MethodPost, "/", nil), "id", "task_f"))
	if w2.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("retry non-failed: expected 400, got %d", w2.Result().StatusCode)
	}
}

// ── 状态回报（按 id / 按 mac）+ 事件发布 ──

func TestReportByIDTransitionsAndPublishesEvent(t *testing.T) {
	st := store.NewMemory()
	bus := eventbus.New()
	h := &InstallTaskHandler{store: st, serverBase: "10.0.0.10:8080", eventBus: bus}
	host := seedHost(t, st, "host-1", "00:11:22:33:44:30")
	seedTask(t, st, "task_1", host.ID, "installing", "")

	received := make(chan notify.Event, 2)
	bus.Subscribe(notify.TopicNotify, func(e eventbus.Event) {
		if evt, ok := e.Payload.(notify.Event); ok {
			received <- evt
		}
	})

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/install-tasks/task_1/report", strings.NewReader(body))
		req = withChiParams(req, "id", "task_1")
		w := httptest.NewRecorder()
		h.Report(w, req)
		return w
	}

	w := post(`{"status":"done"}`)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("report: expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	task, _ := st.GetInstallTask(t.Context(), "task_1")
	if task.Status != "done" {
		t.Fatalf("expected done, got %s", task.Status)
	}
	select {
	case evt := <-received:
		if evt.Event != notify.EventInstallFinished || evt.Host.MAC != host.MAC {
			t.Fatalf("unexpected event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("install.finished not published")
	}

	// 已终态 → 幂等，不重复发事件
	w2 := post(`{"status":"done"}`)
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatalf("idempotent report: expected 200, got %d", w2.Result().StatusCode)
	}
	select {
	case evt := <-received:
		t.Fatalf("unexpected duplicate event: %+v", evt)
	case <-time.After(100 * time.Millisecond):
	}

	// 非法 status → 400
	w3 := post(`{"status":"installing"}`)
	if w3.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid status: expected 400, got %d", w3.Result().StatusCode)
	}
}

func TestReportByMAC(t *testing.T) {
	st := store.NewMemory()
	bus := eventbus.New()
	h := &InstallTaskHandler{store: st, serverBase: "10.0.0.10:8080", eventBus: bus}
	host := seedHost(t, st, "host-1", "00:11:22:33:44:40")
	seedTask(t, st, "task_1", host.ID, "installing", "")

	received := make(chan notify.Event, 2)
	bus.Subscribe(notify.TopicNotify, func(e eventbus.Event) {
		if evt, ok := e.Payload.(notify.Event); ok {
			received <- evt
		}
	})

	// 按 mac 回报 failed（带 message）
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/install-tasks/report-by-mac?mac=00:11:22:33:44:40", strings.NewReader(`{"status":"failed","message":"disk error"}`))
	w := httptest.NewRecorder()
	h.ReportByMAC(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("report-by-mac: expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	task, _ := st.GetInstallTask(t.Context(), "task_1")
	if task.Status != "failed" || task.ErrorMsg != "disk error" {
		t.Fatalf("unexpected task: %+v", task)
	}
	select {
	case evt := <-received:
		if evt.Event != notify.EventInstallFailed || !strings.Contains(evt.Detail, "disk error") {
			t.Fatalf("unexpected event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("install.failed not published")
	}

	// 未知 mac → 204 静默
	w2 := httptest.NewRecorder()
	h.ReportByMAC(w2, httptest.NewRequest(http.MethodPost,
		"/api/v1/install-tasks/report-by-mac?mac=ff:ff:ff:ff:ff:ff", strings.NewReader(`{"status":"done"}`)))
	if w2.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("unknown mac: expected 204, got %d", w2.Result().StatusCode)
	}

	// 主机存在但无活跃任务 → 404
	w3 := httptest.NewRecorder()
	h.ReportByMAC(w3, httptest.NewRequest(http.MethodPost,
		"/api/v1/install-tasks/report-by-mac?mac=00:11:22:33:44:40", strings.NewReader(`{"status":"done"}`)))
	if w3.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("no active task: expected 404, got %d", w3.Result().StatusCode)
	}
}

// ── GetAnswerFile pending→installing 幂等 ──

func TestGetAnswerFileFlipsPendingToInstalling(t *testing.T) {
	h, st := newInstallTaskHandler(t)
	host := seedHost(t, st, "host-1", "00:11:22:33:44:50")
	tmpl := &models.AnswerTemplate{ID: 1, Name: "ks", Type: "kickstart", Content: "echo hi"}
	if err := st.CreateAnswerTemplate(t.Context(), tmpl); err != nil {
		t.Fatal(err)
	}
	tid := tmpl.ID
	task := &models.InstallTask{
		ID: "task_1", HostID: host.ID, DistroName: "Rocky", VersionCodename: "9",
		Arch: "x86_64", Status: "pending", AnswerTemplateID: &tid,
	}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatal(err)
	}

	get := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task_1", nil)
		req = withChiParams(req, "task_id", "task_1")
		w := httptest.NewRecorder()
		h.GetAnswerFile(w, req)
		return w
	}

	w := get()
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	got, _ := st.GetInstallTask(t.Context(), "task_1")
	if got.Status != "installing" {
		t.Fatalf("首次拉取应答后应为 installing, got %s", got.Status)
	}

	// 幂等：再次拉取状态不变、内容不变
	w2 := get()
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatalf("second fetch: expected 200, got %d", w2.Result().StatusCode)
	}
	got, _ = st.GetInstallTask(t.Context(), "task_1")
	if got.Status != "installing" {
		t.Fatalf("重复拉取不应改变状态, got %s", got.Status)
	}
	b1, _ := io.ReadAll(w.Result().Body)
	b2, _ := io.ReadAll(w2.Result().Body)
	if string(b1) != string(b2) {
		t.Fatal("两次拉取内容应一致")
	}
}

// ── 失败锁定：GetTaskByMAC 引导判断 ──

func TestGetTaskByMACFailureLock(t *testing.T) {
	h, st := newInstallTaskHandler(t)
	host := seedHost(t, st, "host-1", "00:11:22:33:44:60")

	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/task/by-mac/00:11:22:33:44:60", nil)
		req = withChiParams(req, "mac", "00:11:22:33:44:60")
		w := httptest.NewRecorder()
		h.GetTaskByMAC(w, req)
		return w
	}

	// 无任务 → 404
	if w := call(); w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("no task: expected 404, got %d", w.Result().StatusCode)
	}

	// pending → 200 下发安装
	seedTask(t, st, "task_1", host.ID, "pending", "")
	if w := call(); w.Result().StatusCode != http.StatusOK {
		t.Fatalf("pending: expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}

	// 最新任务 failed（且存在更老的 pending）→ 404 锁定
	seedTask(t, st, "task_2", host.ID, "failed", "")
	w := call()
	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("failed lock: expected 404, got %d", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "锁定") {
		t.Fatalf("lock response should mention 锁定, got: %s", w.Body.String())
	}

	// retry 解锁（failed→pending）→ 恢复 200
	req := httptest.NewRequest(http.MethodPost, "/api/v1/install-tasks/task_2/retry", nil)
	req = withChiParams(req, "id", "task_2")
	wr := httptest.NewRecorder()
	h.Retry(wr, req)
	if wr.Result().StatusCode != http.StatusOK {
		t.Fatalf("retry: expected 200, got %d", wr.Result().StatusCode)
	}
	if w := call(); w.Result().StatusCode != http.StatusOK {
		t.Fatalf("after retry: expected 200, got %d", w.Result().StatusCode)
	}

	// 最新任务 done → 404 走正常菜单
	seedTask(t, st, "task_3", host.ID, "done", "")
	if w := call(); w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("done latest: expected 404, got %d", w.Result().StatusCode)
	}
}
