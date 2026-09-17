package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
)

func newInstallTaskHandler(t *testing.T) (*InstallTaskHandler, store.Interface) {
	t.Helper()
	st := store.NewMemory()
	return &InstallTaskHandler{store: st, serverBase: "10.0.0.10:8080"}, st
}

// TestGetTaskByMACAnswerParamFallback: when the overlay carries no
// answer_param, the catalog version's own answer_param (e.g. a store-imported
// anaconda entry with inst.ks={{.AnswerURL}}) is used so answer injection
// works out of the box.
func TestGetTaskByMACAnswerParamFallback(t *testing.T) {
	st := store.NewMemory()
	mgr := netboot.NewManager(&netboot.Catalog{Distros: []*netboot.Distro{
		{
			Name:      "Rocky",
			Enabled:   true,
			MenuGroup: "linux",
			Versions: []*netboot.Version{
				{Codename: "9", Name: "9", Arch: "amd64", Enabled: true, AnswerParam: "inst.ks={{.AnswerURL}}"},
			},
		},
	}})
	h := &InstallTaskHandler{store: st, serverBase: "10.0.0.10:8080", netbootMgr: mgr}
	host := seedHost(t, st, "host-ks", "00:11:22:33:44:77")
	seedTask(t, st, "task_ks", host.ID, "pending", "")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/task/by-mac/00:11:22:33:44:77", nil)
	req = withChiParams(req, "mac", "00:11:22:33:44:77")
	w := httptest.NewRecorder()
	h.GetTaskByMAC(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Result().StatusCode, w.Body.String())
	}
	var resp struct {
		Data struct {
			AnswerParam string `json:"AnswerParam"`
			AnswerURL   string `json:"AnswerURL"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Result().Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.AnswerParam != "inst.ks={{.AnswerURL}}" {
		t.Errorf("expected catalog answer_param fallback, got %q", resp.Data.AnswerParam)
	}
	if resp.Data.AnswerURL == "" {
		t.Error("answer URL not computed")
	}
}

func seedAnswerTask(t *testing.T, st store.Interface, content string) uint {
	t.Helper()
	tmpl := &models.AnswerTemplate{
		ID:      1,
		Name:    "winpeshl",
		Type:    "autounattend",
		Content: content,
	}
	if err := st.CreateAnswerTemplate(t.Context(), tmpl); err != nil {
		t.Fatalf("seed template: %v", err)
	}
	tid := tmpl.ID
	task := &models.InstallTask{
		ID:               "task_winpe",
		HostID:           "host-1",
		DistroName:       "Windows",
		VersionCodename:  "w11",
		Arch:             "amd64",
		AnswerTemplateID: &tid,
	}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatalf("seed task: %v", err)
	}
	return tid
}

func TestGetAnswerFileWinpeShellIni(t *testing.T) {
	h, st := newInstallTaskHandler(t)
	body := "echo installing > C:\\install.log"
	seedAnswerTask(t, st, body)

	// 无参数 -> 返回模板本身（即 X:\install.bat 内容）
	req := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task_winpe", nil)
	req = withChiParams(req, "task_id", "task_winpe")
	w := httptest.NewRecorder()
	h.GetAnswerFile(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	out, _ := io.ReadAll(w.Body)
	if string(out) != body {
		t.Fatalf("expected template body, got: %q", string(out))
	}

	// ?file=winpeshl.ini -> 返回引用 install.bat 的 INI 包装
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task_winpe?file=winpeshl.ini", nil)
	req2 = withChiParams(req2, "task_id", "task_winpe")
	w2 := httptest.NewRecorder()
	h.GetAnswerFile(w2, req2)
	if w2.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w2.Result().StatusCode)
	}
	ini, _ := io.ReadAll(w2.Body)
	s := string(ini)
	if !strings.Contains(s, "[LaunchApp]") || !strings.Contains(s, "AppPath = X:\\install.bat") {
		t.Fatalf("expected winpeshl.ini wrapper, got: %q", s)
	}
	if strings.Contains(s, body) {
		t.Fatalf("winpeshl.ini must not embed the install.bat body, got: %q", s)
	}
}

// withChiParams 在直调 handler 时注入 chi 路由参数。
func withChiParams(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
