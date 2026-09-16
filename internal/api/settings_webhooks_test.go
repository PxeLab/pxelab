package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/notify"
	"github.com/pxelab/pxelab/internal/store"
	"gopkg.in/yaml.v3"
)

func newWebhookSettingsHandler(t *testing.T) *SettingsHandler {
	t.Helper()
	cfg := &config.Config{}
	cfg.ConfigPath = filepath.Join(t.TempDir(), "config.yaml")
	return &SettingsHandler{cfg: cfg, store: store.NewMemory()}
}

func webhookBody(t *testing.T, h *SettingsHandler) []config.WebhookConfig {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/webhooks", nil)
	w := httptest.NewRecorder()
	h.ListWebhooks(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Result().StatusCode)
	}
	var resp struct {
		Data struct {
			Webhooks []config.WebhookConfig `json:"webhooks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Data.Webhooks
}

func TestWebhookCRUD(t *testing.T) {
	h := newWebhookSettingsHandler(t)

	// Create
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/webhooks",
		strings.NewReader(`{"name":"ops","url":"https://example.com/hook","events":["install.finished"],"format":"generic","enabled":true}`))
	w := httptest.NewRecorder()
	h.CreateWebhook(w, req)
	if w.Result().StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	var createResp struct {
		Data config.WebhookConfig `json:"data"`
	}
	json.Unmarshal(w.Body.Bytes(), &createResp)
	id := createResp.Data.ID
	if id == "" || createResp.Data.Format != "generic" {
		t.Fatalf("unexpected created webhook: %+v", createResp.Data)
	}

	// 配置已写回 config.yaml
	data, err := os.ReadFile(h.cfg.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var saved struct {
		Notify config.NotifyConfig `yaml:"notify"`
	}
	if err := yaml.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if len(saved.Notify.Webhooks) != 1 || saved.Notify.Webhooks[0].Name != "ops" {
		t.Fatalf("config.yaml not persisted: %+v", saved.Notify.Webhooks)
	}

	// List
	if got := webhookBody(t, h); len(got) != 1 || got[0].ID != id {
		t.Fatalf("list: unexpected %+v", got)
	}

	// Update（改为飞书 + 关闭）
	req = httptest.NewRequest(http.MethodPut, "/api/v1/settings/webhooks/"+id,
		strings.NewReader(`{"name":"ops2","url":"https://example.com/hook2","events":["*"],"format":"feishu","secret":"s3","enabled":false}`))
	req = withChiParams(req, "id", id)
	w = httptest.NewRecorder()
	h.UpdateWebhook(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	got := webhookBody(t, h)
	if len(got) != 1 || got[0].Name != "ops2" || got[0].Format != "feishu" || got[0].Secret != "s3" || got[0].Enabled {
		t.Fatalf("update not applied: %+v", got)
	}

	// Update 不存在 → 404
	req = httptest.NewRequest(http.MethodPut, "/api/v1/settings/webhooks/wh_none",
		strings.NewReader(`{"name":"x","url":"https://x","events":["*"]}`))
	req = withChiParams(req, "id", "wh_none")
	w = httptest.NewRecorder()
	h.UpdateWebhook(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("update missing: expected 404, got %d", w.Result().StatusCode)
	}

	// 校验：空名称 / 无事件 / 非法 format
	for _, body := range []string{
		`{"name":"","url":"https://x","events":["*"]}`,
		`{"name":"x","url":"https://x","events":[]}`,
		`{"name":"x","url":"https://x","events":["*"],"format":"slack"}`,
	} {
		req = httptest.NewRequest(http.MethodPost, "/api/v1/settings/webhooks", strings.NewReader(body))
		w = httptest.NewRecorder()
		h.CreateWebhook(w, req)
		if w.Result().StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d", body, w.Result().StatusCode)
		}
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/settings/webhooks/"+id, nil)
	req = withChiParams(req, "id", id)
	w = httptest.NewRecorder()
	h.DeleteWebhook(w, req)
	if w.Result().StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", w.Result().StatusCode)
	}
	if got := webhookBody(t, h); len(got) != 0 {
		t.Fatalf("expected empty after delete: %+v", got)
	}
}

func TestWebhookTestEndpoint(t *testing.T) {
	h := newWebhookSettingsHandler(t)

	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 新建指向测试接收端的 webhook
	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/webhooks",
		strings.NewReader(`{"name":"ops","url":"`+srv.URL+`","events":["install.finished"],"format":"generic","enabled":true}`))
	w := httptest.NewRecorder()
	h.CreateWebhook(w, req)
	id := webhookBody(t, h)[0].ID

	// 发送测试消息 → 投递成功
	req = httptest.NewRequest(http.MethodPost, "/api/v1/settings/webhooks/"+id+"/test", nil)
	req = withChiParams(req, "id", id)
	w = httptest.NewRecorder()
	h.TestWebhook(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("test: expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	if atomic.LoadInt64(&hits) != 1 {
		t.Fatalf("expected receiver hit once, got %d", hits)
	}

	// 指向不可达地址 → 502
	h.mu.Lock()
	h.cfg.Notify.Webhooks[0].URL = "http://127.0.0.1:1/unreachable"
	h.mu.Unlock()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/settings/webhooks/"+id+"/test", nil)
	req = withChiParams(req, "id", id)
	w = httptest.NewRecorder()
	h.TestWebhook(w, req)
	if w.Result().StatusCode != http.StatusBadGateway {
		t.Fatalf("test unreachable: expected 502, got %d", w.Result().StatusCode)
	}

	// 不存在 → 404
	req = httptest.NewRequest(http.MethodPost, "/api/v1/settings/webhooks/wh_none/test", nil)
	req = withChiParams(req, "id", "wh_none")
	w = httptest.NewRecorder()
	h.TestWebhook(w, req)
	if w.Result().StatusCode != http.StatusNotFound {
		t.Fatalf("test missing: expected 404, got %d", w.Result().StatusCode)
	}
}

// 事件→投递链路：基线脚本失败回执 → notify 事件发布。
func TestBaselineReportFailurePublishesNotifyEvent(t *testing.T) {
	h, st := newBaselineHandler(t)
	hostID := seedReportHost(t, st)
	_ = hostID

	received := make(chan notify.Event, 1)
	h.eventBus.Subscribe(notify.TopicNotify, func(e eventbus.Event) {
		if evt, ok := e.Payload.(notify.Event); ok {
			received <- evt
		}
	})

	// exit_code=0 不发布
	postReport(t, h, "mac=00:11:22:33:44:55", `{"script_name":"ok","seq":1,"exit_code":0}`)
	select {
	case evt := <-received:
		t.Fatalf("unexpected event for success report: %+v", evt)
	case <-time.After(100 * time.Millisecond):
	}

	// exit_code!=0 发布 baseline.script_failed
	postReport(t, h, "mac=00:11:22:33:44:55", `{"script_name":"bad","seq":2,"exit_code":1,"output_tail":"boom"}`)
	select {
	case evt := <-received:
		if evt.Event != notify.EventBaselineScriptFailed {
			t.Fatalf("unexpected event type: %s", evt.Event)
		}
		if evt.Host.MAC != "00:11:22:33:44:55" || !strings.Contains(evt.Detail, "bad") || !strings.Contains(evt.Detail, "boom") {
			t.Fatalf("unexpected event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("baseline.script_failed event not published")
	}
}

// 事件→投递链路：安装任务状态流转 → notify 事件发布。
func TestInstallTaskStatusTransitionPublishesNotifyEvent(t *testing.T) {
	st := store.NewMemory()
	bus := eventbus.New()
	h := &InstallTaskHandler{store: st, serverBase: "10.0.0.10:8080", eventBus: bus}
	host := &models.Host{Name: "node-1", MAC: "00:11:22:33:44:55", IP: "192.168.1.100"}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}
	task := &models.InstallTask{ID: "task_1", HostID: host.ID, DistroName: "Rocky", VersionCodename: "9", Arch: "x86_64", Status: "installing"}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatal(err)
	}

	received := make(chan notify.Event, 2)
	bus.Subscribe(notify.TopicNotify, func(e eventbus.Event) {
		if evt, ok := e.Payload.(notify.Event); ok {
			received <- evt
		}
	})

	put := func(status, errMsg string) {
		body := `{"host_id":"` + host.ID + `","distro_name":"Rocky","version_codename":"9","arch":"x86_64","status":"` + status + `","error_msg":"` + errMsg + `"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/netboot/tasks/task_1", strings.NewReader(body))
		req = withChiParams(req, "id", "task_1")
		w := httptest.NewRecorder()
		h.Update(w, req)
		if w.Result().StatusCode != http.StatusOK {
			t.Fatalf("update to %s: expected 200, got %d: %s", status, w.Result().StatusCode, w.Body.String())
		}
	}

	put("done", "")
	select {
	case evt := <-received:
		if evt.Event != notify.EventInstallFinished || evt.Host.MAC != "00:11:22:33:44:55" || evt.Host.Name != "node-1" {
			t.Fatalf("unexpected finished event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("install.finished not published")
	}

	put("failed", "disk error")
	select {
	case evt := <-received:
		if evt.Event != notify.EventInstallFailed || !strings.Contains(evt.Detail, "disk error") {
			t.Fatalf("unexpected failed event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("install.failed not published")
	}

	// 状态未变化 → 不重复发布
	put("failed", "disk error")
	select {
	case evt := <-received:
		t.Fatalf("unexpected duplicate event: %+v", evt)
	case <-time.After(100 * time.Millisecond):
	}
}

// 事件→投递链路：未注册主机首次 PXE 引导 → notify 事件发布（经由 UpsertPxeBootRecord created 标记）。
func TestFirstPXEBootDetection(t *testing.T) {
	st := store.NewMemory()
	created, err := st.UpsertPxeBootRecord(t.Context(), "AA:BB:CC:DD:EE:FF", "ipxe", "default-menu", "192.168.1.50")
	if err != nil || !created {
		t.Fatalf("first upsert: created=%v err=%v", created, err)
	}
	created, err = st.UpsertPxeBootRecord(t.Context(), "aa:bb:cc:dd:ee:ff", "ipxe", "default-menu", "192.168.1.50")
	if err != nil || created {
		t.Fatalf("second upsert: created=%v err=%v", created, err)
	}
}
