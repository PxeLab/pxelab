// Package notify 全局 webhook 通知（R3）：订阅事件总线，把事件投递到用户配置的
// 多个 webhook（generic / 钉钉 / 飞书），异步 worker、5s 超时、失败重试 3 次指数退避，
// 最终失败写审计日志；绝不阻塞事件产生方。
package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

// TopicNotify 事件总线上通知事件的 topic；payload 为 notify.Event。
const TopicNotify = "notify"

// 支持的事件类型（首版四类）
const (
	EventInstallFinished      = "install.finished"
	EventInstallFailed        = "install.failed"
	EventBaselineScriptFailed = "baseline.script_failed"
	EventHostFirstPXEBoot     = "host.first_pxe_boot"
)

// 投递参数（FR-3.4）
const (
	deliverTimeout = 5 * time.Second
	maxRetries     = 3 // 首次失败后再重试 3 次
)

// 事件类型的中文标签（钉钉/飞书文本消息用）
var eventLabels = map[string]string{
	EventInstallFinished:      "装机完成",
	EventInstallFailed:        "装机失败",
	EventBaselineScriptFailed: "基线脚本执行失败",
	EventHostFirstPXEBoot:     "新机器首次 PXE 引导",
}

// HostInfo 事件关联的主机信息。
type HostInfo struct {
	MAC  string `json:"mac"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// Event 统一事件体：{event, time, host:{mac,name,ip}, detail}。
type Event struct {
	Event  string    `json:"event"`
	Time   time.Time `json:"time"`
	Host   HostInfo  `json:"host"`
	Detail string    `json:"detail"`
}

// Label 返回事件类型的中文标签（未知类型原样返回）。
func (e Event) Label() string {
	if l, ok := eventLabels[e.Event]; ok {
		return l
	}
	return e.Event
}

// text 渲染钉钉/飞书 text 消息的正文。
func (e Event) text() string {
	hostDesc := e.Host.MAC
	if e.Host.Name != "" {
		hostDesc = fmt.Sprintf("%s (%s)", e.Host.Name, e.Host.MAC)
	}
	text := fmt.Sprintf("[PxeLab] %s\n主机: %s", e.Label(), hostDesc)
	if e.Host.IP != "" {
		text += "\nIP: " + e.Host.IP
	}
	if e.Detail != "" {
		text += "\n详情: " + e.Detail
	}
	text += "\n时间: " + e.Time.Format("2006-01-02 15:04:05")
	return text
}

// feishuSign 飞书自定义机器人签名校验：HMAC-SHA256(key=timestamp+"\n"+secret, msg=空) 后 Base64。
func feishuSign(timestamp int64, secret string) string {
	stringToSign := strconv.FormatInt(timestamp, 10) + "\n" + secret
	h := hmac.New(sha256.New, []byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// BuildPayload 按 webhook format 封装事件消息体。
func BuildPayload(wh config.WebhookConfig, evt Event) ([]byte, error) {
	switch wh.Format {
	case "dingtalk":
		return json.Marshal(map[string]any{
			"msgtype": "text",
			"text":    map[string]string{"content": evt.text()},
		})
	case "feishu":
		msg := map[string]any{
			"msg_type": "text",
			"content":  map[string]string{"text": evt.text()},
		}
		if wh.Secret != "" {
			ts := time.Now().Unix()
			msg["timestamp"] = strconv.FormatInt(ts, 10)
			msg["sign"] = feishuSign(ts, wh.Secret)
		}
		return json.Marshal(msg)
	default: // generic：原始事件 JSON
		return json.Marshal(evt)
	}
}

// Dispatcher webhook 投递器：订阅事件总线并异步投递。
type Dispatcher struct {
	getWebhooks func() []config.WebhookConfig // 读取当前生效的 webhook 列表（配置运行时可变）
	store       store.Interface               // 最终失败写审计日志
	client      *http.Client
	sleep       func(time.Duration) // 指数退避等待（测试可替换）
}

func NewDispatcher(getWebhooks func() []config.WebhookConfig, st store.Interface) *Dispatcher {
	return &Dispatcher{
		getWebhooks: getWebhooks,
		store:       st,
		client:      &http.Client{Timeout: deliverTimeout},
		sleep:       time.Sleep,
	}
}

// Start 订阅事件总线；PublishAsync 触达时 Handle 已不阻塞，但 Start 本身幂等订阅一次。
func (d *Dispatcher) Start(bus *eventbus.Bus) {
	bus.Subscribe(TopicNotify, func(e eventbus.Event) {
		evt, ok := e.Payload.(Event)
		if !ok {
			return
		}
		d.Handle(evt)
	})
}

// Handle 匹配订阅了该事件类型且启用的 webhook，逐个异步投递（不阻塞调用方）。
func (d *Dispatcher) Handle(evt Event) {
	if d.getWebhooks == nil {
		return
	}
	for _, wh := range d.getWebhooks() {
		if !wh.Enabled || !subscribed(wh.Events, evt.Event) {
			continue
		}
		go func(wh config.WebhookConfig) {
			if err := d.deliverWithRetry(wh, evt); err != nil {
				d.recordFailure(wh, evt, err)
			}
		}(wh)
	}
}

// subscribed 判断事件类型是否在订阅列表中（"*" 表示全部）。
func subscribed(events []string, eventType string) bool {
	for _, e := range events {
		if e == "*" || e == eventType {
			return true
		}
	}
	return false
}

// deliverWithRetry 失败重试 3 次，指数退避 1s/2s/4s。
func (d *Dispatcher) deliverWithRetry(wh config.WebhookConfig, evt Event) error {
	var err error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			d.sleep(time.Duration(1<<(attempt-1)) * time.Second)
		}
		if err = d.DeliverOnce(wh, evt); err == nil {
			return nil
		}
		slog.Warn("webhook 投递失败，将重试", "webhook", wh.Name, "url", wh.URL, "attempt", attempt+1, "error", err)
	}
	return err
}

// DeliverOnce 单次投递：5s 超时；HTTP 非 2xx 或机器人返回业务错误码均视为失败。
func (d *Dispatcher) DeliverOnce(wh config.WebhookConfig, evt Event) error {
	payload, err := BuildPayload(wh, evt)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, wh.URL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	// 钉钉 {"errcode":0} / 飞书 {"code":0} 或 {"StatusCode":0}：2xx 但业务失败也算失败
	var biz map[string]any
	if json.Unmarshal(body, &biz) == nil {
		for _, key := range []string{"errcode", "code", "StatusCode"} {
			if v, ok := biz[key]; ok {
				if code, ok := v.(float64); ok && code != 0 {
					return fmt.Errorf("机器人返回错误 %s=%v: %s", key, code, string(body))
				}
			}
		}
	}
	return nil
}

// recordFailure 最终失败：写审计日志（不阻塞、失败仅告警）。
func (d *Dispatcher) recordFailure(wh config.WebhookConfig, evt Event, err error) {
	slog.Error("webhook 投递最终失败", "webhook", wh.Name, "url", wh.URL, "event", evt.Event, "error", err)
	if d.store == nil {
		return
	}
	log := &models.AuditLog{
		ID:        fmt.Sprintf("%d-webhook", time.Now().UnixNano()),
		Action:    models.AuditUpdate,
		Resource:  "webhook",
		ResourceID: wh.ID,
		Detail:    fmt.Sprintf("webhook 投递失败（已重试 %d 次）: %s → %s，事件 %s: %v", maxRetries, wh.Name, wh.URL, evt.Event, err),
		Timestamp: time.Now(),
	}
	if err := d.store.CreateAuditLog(context.Background(), log); err != nil {
		slog.Warn("webhook 失败审计写入失败", "error", err)
	}
}
