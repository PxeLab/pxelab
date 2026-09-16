package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

func testEvent() Event {
	return Event{
		Event:  EventInstallFinished,
		Time:   time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local),
		Host:   HostInfo{MAC: "00:11:22:33:44:55", Name: "node-1", IP: "192.168.1.100"},
		Detail: "Rocky 9 (9/x86_64)",
	}
}

func TestBuildPayloadGeneric(t *testing.T) {
	payload, err := BuildPayload(config.WebhookConfig{Format: "generic"}, testEvent())
	if err != nil {
		t.Fatal(err)
	}
	var evt Event
	if err := json.Unmarshal(payload, &evt); err != nil {
		t.Fatalf("generic payload must be raw event JSON: %v", err)
	}
	if evt.Event != EventInstallFinished || evt.Host.MAC != "00:11:22:33:44:55" || evt.Detail == "" {
		t.Fatalf("unexpected generic payload: %s", payload)
	}
}

func TestBuildPayloadDingTalk(t *testing.T) {
	payload, err := BuildPayload(config.WebhookConfig{Format: "dingtalk"}, testEvent())
	if err != nil {
		t.Fatal(err)
	}
	var msg struct {
		MsgType string `json:"msgtype"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.MsgType != "text" {
		t.Fatalf("expected msgtype=text: %s", payload)
	}
	for _, want := range []string{"装机完成", "node-1", "00:11:22:33:44:55", "Rocky 9"} {
		if !strings.Contains(msg.Text.Content, want) {
			t.Fatalf("content missing %q: %s", want, msg.Text.Content)
		}
	}
}

func TestBuildPayloadFeishuWithSign(t *testing.T) {
	payload, err := BuildPayload(config.WebhookConfig{Format: "feishu", Secret: "sec-1"}, testEvent())
	if err != nil {
		t.Fatal(err)
	}
	var msg map[string]any
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatal(err)
	}
	if msg["msg_type"] != "text" {
		t.Fatalf("expected msg_type=text: %s", payload)
	}
	tsStr, ok := msg["timestamp"].(string)
	if !ok || tsStr == "" {
		t.Fatalf("timestamp missing: %s", payload)
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if msg["sign"] != feishuSign(ts, "sec-1") {
		t.Fatalf("sign mismatch: %s", payload)
	}

	// 无 secret 时不带 timestamp/sign
	payload2, _ := BuildPayload(config.WebhookConfig{Format: "feishu"}, testEvent())
	var msg2 map[string]any
	json.Unmarshal(payload2, &msg2)
	if _, ok := msg2["sign"]; ok {
		t.Fatalf("sign should be omitted without secret: %s", payload2)
	}
}

func TestDeliverOnceSuccess(t *testing.T) {
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody = make([]byte, r.ContentLength)
		r.Body.Read(gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(nil, nil)
	if err := d.DeliverOnce(config.WebhookConfig{URL: srv.URL, Format: "generic"}, testEvent()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gotBody), "install.finished") {
		t.Fatalf("server did not receive event body: %s", gotBody)
	}
}

func TestDeliverOnceRobotBizError(t *testing.T) {
	// 钉钉风格：HTTP 200 但 errcode != 0 → 视为失败
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errcode":310000,"errmsg":"sign not match"}`))
	}))
	defer srv.Close()

	d := NewDispatcher(nil, nil)
	if err := d.DeliverOnce(config.WebhookConfig{URL: srv.URL, Format: "dingtalk"}, testEvent()); err == nil {
		t.Fatal("expected biz error to be treated as failure")
	}
}

func TestDeliverWithRetryEventuallySucceeds(t *testing.T) {
	var attempts int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt64(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(nil, nil)
	d.sleep = func(time.Duration) {} // 测试不等待退避
	if err := d.deliverWithRetry(config.WebhookConfig{URL: srv.URL, Format: "generic"}, testEvent()); err != nil {
		t.Fatal(err)
	}
	if got := atomic.LoadInt64(&attempts); got != 3 {
		t.Fatalf("expected 3 attempts, got %d", got)
	}
}

func TestDeliverWithRetryFinalFailureSilent(t *testing.T) {
	var attempts int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	st := store.NewMemory()
	d := NewDispatcher(nil, st)
	d.sleep = func(time.Duration) {}
	err := d.deliverWithRetry(config.WebhookConfig{ID: "wh_1", Name: "hook1", URL: srv.URL, Format: "generic"}, testEvent())
	if err == nil {
		t.Fatal("expected final failure")
	}
	// 1 次首发 + 3 次重试
	if got := atomic.LoadInt64(&attempts); got != 4 {
		t.Fatalf("expected 4 attempts (1+3 retries), got %d", got)
	}
	// Handle 路径：最终失败写审计日志，不 panic、不阻塞
	d.Handle(testEvent())
	// Handle 是异步投递，这里直接验证 recordFailure
	d.recordFailure(config.WebhookConfig{ID: "wh_1", Name: "hook1", URL: srv.URL}, testEvent(), err)
	logs, total, lerr := st.ListAuditLogs(context.Background(), store.AuditLogFilter{Resource: "webhook", Page: 1, Size: 10})
	if lerr != nil {
		t.Fatal(lerr)
	}
	if total != 1 || !strings.Contains(logs[0].Detail, "投递失败") {
		t.Fatalf("expected failure audit log, got total=%d %+v", total, logs)
	}
	_ = models.AuditLog{}
}

func TestDispatcherEndToEnd(t *testing.T) {
	received := make(chan Event, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt Event
		json.NewDecoder(r.Body).Decode(&evt)
		received <- evt
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	webhooks := []config.WebhookConfig{
		{ID: "wh_1", Name: "hit", URL: srv.URL, Format: "generic", Events: []string{EventInstallFinished}, Enabled: true},
		{ID: "wh_2", Name: "not-subscribed", URL: srv.URL, Format: "generic", Events: []string{EventInstallFailed}, Enabled: true},
		{ID: "wh_3", Name: "disabled", URL: srv.URL, Format: "generic", Events: []string{EventInstallFinished}, Enabled: false},
	}
	d := NewDispatcher(func() []config.WebhookConfig { return webhooks }, nil)
	d.sleep = func(time.Duration) {}

	bus := eventbus.New()
	d.Start(bus)
	bus.Publish(TopicNotify, testEvent())

	select {
	case evt := <-received:
		if evt.Event != EventInstallFinished {
			t.Fatalf("unexpected event: %+v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook did not receive event")
	}
	// 只应命中 1 条（not-subscribed 与 disabled 不投）：等一小段时间确认没有第二条
	select {
	case evt := <-received:
		t.Fatalf("unexpected second delivery: %+v", evt)
	case <-time.After(200 * time.Millisecond):
	}
}
