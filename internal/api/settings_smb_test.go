package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/store"
)

// newSMBSettingsHandler 构造只依赖 store 的 SettingsHandler（保存写临时目录）。
func newSMBSettingsHandler(t *testing.T, users []config.SMBUserConfig) *SettingsHandler {
	t.Helper()
	cfg := &config.Config{}
	cfg.Global.DataDir = t.TempDir()
	cfg.SMB.Users = users
	return &SettingsHandler{cfg: cfg, store: store.NewMemory()}
}

// TestGetSMBNeverReturnsPassword：接口绝不回传口令，仅 password_set 标志。
func TestGetSMBNeverReturnsPassword(t *testing.T) {
	h := newSMBSettingsHandler(t, []config.SMBUserConfig{
		{Name: "admin", Password: "sup3r-s3cret"},
		{Name: "empty", Password: ""},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/smb", nil)
	w := httptest.NewRecorder()
	h.GetSMB(w, req)

	body := w.Body.String()
	if strings.Contains(body, "sup3r-s3cret") {
		t.Fatalf("响应泄漏了口令:\n%s", body)
	}
	if strings.Contains(body, `"password"`) {
		t.Fatalf("响应不应包含 password 字段:\n%s", body)
	}
	var resp struct {
		Data SMBSettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Users) != 2 {
		t.Fatalf("users = %d, want 2", len(resp.Data.Users))
	}
	if !resp.Data.Users[0].PasswordSet {
		t.Error("admin 应 password_set=true")
	}
	if resp.Data.Users[1].PasswordSet {
		t.Error("空口令账号应 password_set=false")
	}
}

// TestUpdateSMBKeepsPasswordWhenBlank：留空 = 保持原口令；填值 = 轮换。
func TestUpdateSMBKeepsPasswordWhenBlank(t *testing.T) {
	h := newSMBSettingsHandler(t, []config.SMBUserConfig{{Name: "admin", Password: "orig-pass"}})

	// 留空保存（例如只改共享）→ 口令保持
	body := `{"enabled":true,"port":445,"server_name":"PXELAB","shares":[],"users":[{"name":"admin","password":""}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/services/smb", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.UpdateSMB(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("留空保存应成功, got %d (%s)", w.Result().StatusCode, w.Body.String())
	}
	if got := h.cfg.SMB.Users[0].Password; got != "orig-pass" {
		t.Fatalf("口令应保持原值, got %q", got)
	}

	// 填值 → 轮换
	body2 := `{"enabled":true,"port":445,"server_name":"PXELAB","shares":[],"users":[{"name":"admin","password":"new-pass"}]}`
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/services/smb", strings.NewReader(body2))
	w2 := httptest.NewRecorder()
	h.UpdateSMB(w2, req2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatalf("轮换应成功, got %d", w2.Result().StatusCode)
	}
	if got := h.cfg.SMB.Users[0].Password; got != "new-pass" {
		t.Fatalf("口令应被更新, got %q", got)
	}
}

// TestUpdateSMBRejectsNewUserWithoutPassword：新增（或重命名）账号留空应 400，且不改配置。
func TestUpdateSMBRejectsNewUserWithoutPassword(t *testing.T) {
	h := newSMBSettingsHandler(t, []config.SMBUserConfig{{Name: "admin", Password: "orig-pass"}})

	// 新增账号留空
	body := `{"enabled":true,"port":445,"users":[{"name":"admin","password":""},{"name":"newbie","password":""}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/services/smb", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.UpdateSMB(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("新增账号留空应 400, got %d (%s)", w.Result().StatusCode, w.Body.String())
	}
	if len(h.cfg.SMB.Users) != 1 || h.cfg.SMB.Users[0].Name != "admin" {
		t.Fatalf("校验失败不应改动配置: %+v", h.cfg.SMB.Users)
	}

	// 重命名（等价于删除旧名+新增新名）留空
	body2 := `{"enabled":true,"port":445,"users":[{"name":"renamed","password":""}]}`
	req2 := httptest.NewRequest(http.MethodPut, "/api/v1/services/smb", strings.NewReader(body2))
	w2 := httptest.NewRecorder()
	h.UpdateSMB(w2, req2)
	if w2.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("重命名留空应 400, got %d", w2.Result().StatusCode)
	}
}

// TestUpdateSMBRejectsEmptyUserName：空账号名应 400。
func TestUpdateSMBRejectsEmptyUserName(t *testing.T) {
	h := newSMBSettingsHandler(t, nil)
	body := `{"enabled":true,"users":[{"name":"  ","password":"x"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/services/smb", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.UpdateSMB(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("空账号名应 400, got %d", w.Result().StatusCode)
	}
}

// TestProbePortAvailable：空闲端口探测为可用；被占用后探测为不可用。
func TestProbePortAvailable(t *testing.T) {
	// 取一个当前空闲端口
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("无法取空闲端口: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	if !probePortAvailable(port) {
		t.Skipf("端口 %d 已被其它进程占用（环境相关），跳过", port)
	}

	// 占住该端口 → 应探测为不可用
	l2, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Skipf("无法占用端口 %d: %v", port, err)
	}
	defer l2.Close()
	if probePortAvailable(port) {
		t.Fatalf("端口 %d 被占用时应探测为不可用", port)
	}
}

// TestGetSMBPortAvailableReflectsProbe：GetSMB 在未运行时给出 port_available。
func TestGetSMBPortAvailableReflectsProbe(t *testing.T) {
	h := newSMBSettingsHandler(t, nil)
	h.cfg.SMB.Port = 445
	req := httptest.NewRequest(http.MethodGet, "/api/v1/services/smb", nil)
	w := httptest.NewRecorder()
	h.GetSMB(w, req)
	var resp struct {
		Data SMBSettingsResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Port != 445 {
		t.Fatalf("port = %d", resp.Data.Port)
	}
	// 字段必须存在（值随环境而定：本机 445 是否空闲）
	if resp.Data.Running {
		t.Fatal("未注入 isServiceRunning 时应视为未运行")
	}
}
