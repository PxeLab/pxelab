package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
)

func TestSmbServerHost(t *testing.T) {
	cases := map[string]string{
		"10.0.0.10:8080": "10.0.0.10",
		"10.0.0.10":      "10.0.0.10",
		"pxelab":         "pxelab",
		"[::1]:8080":     "::1",
		"":               "",
	}
	for in, want := range cases {
		if got := smbServerHost(in); got != want {
			t.Errorf("smbServerHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSmbShareName(t *testing.T) {
	if got := smbShareName(nil); got != "isos" {
		t.Fatalf("nil cfg → %q, want isos", got)
	}
	if got := smbShareName(&config.Config{}); got != "isos" {
		t.Fatalf("无共享 → %q, want isos", got)
	}
	cfg := &config.Config{SMB: config.SMBConfig{Shares: []config.SMBShareConfig{
		{Name: "  "}, // 空名跳过
		{Name: "media"},
	}}}
	if got := smbShareName(cfg); got != "media" {
		t.Fatalf("首个非空共享 → %q, want media", got)
	}
}

func TestWimIndexString(t *testing.T) {
	cases := map[int]string{0: "1", -1: "1", 1: "1", 5: "5"}
	for in, want := range cases {
		if got := wimIndexString(in); got != want {
			t.Errorf("wimIndexString(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestWinpePresetParameterizedRender(t *testing.T) {
	preset := builtinPresets["winpeshl"][0]
	cfg := &config.Config{SMB: config.SMBConfig{Shares: []config.SMBShareConfig{{Name: "media"}}}}

	data := netboot.AnswerData{}
	fillWinpeInstallParams(&data, "10.0.0.10:8080", cfg, 3)

	out, err := netboot.RenderAnswerTemplate(preset.Content, data)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	mustContain := []string{
		`set "PXELAB_HOST=10.0.0.10"`,
		`set "PXELAB_SHARE=media"`,
		`set "WIMINDEX=3"`,
		`set "WINDRV=W:"`,
		`set "ESPART=S:"`,
		`net use Z: \\%PXELAB_HOST%\%PXELAB_SHARE% /persistent:no`,
		`/ImageFile:"%SRC%\install.wim" /Index:%WIMINDEX% /ApplyDir:%WINDRV%\`,
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Fatalf("渲染结果缺少 %q:\n%s", s, out)
		}
	}
	if strings.Contains(out, "{{") {
		t.Fatalf("存在未渲染的模板变量:\n%s", out)
	}
	// 旧的硬编码不应残留
	if strings.Contains(out, `set "PXELAB_HOST=pxelab"`) || strings.Contains(out, `\\%PXELAB_HOST%\isos`) {
		t.Fatalf("预设仍含硬编码主机/共享:\n%s", out)
	}
}

// TestGetAnswerFileWinpePresetFillsInstallParams 端到端：使用内置 winpeshl 预设
// 作为应答内容 + 任务 WIMIndex，install.bat 应填入服务器/共享/索引（借鉴项 6）。
func TestGetAnswerFileWinpePresetFillsInstallParams(t *testing.T) {
	st := store.NewMemory()
	h := &InstallTaskHandler{
		store:      st,
		serverBase: "10.0.0.10:8080",
		cfg:        &config.Config{SMB: config.SMBConfig{Shares: []config.SMBShareConfig{{Name: "isos"}}}},
		bootFS:     boot.NewBootFileServer(t.TempDir()),
	}
	host := &models.Host{Name: "h1", MAC: "aa:bb:cc:dd:ee:02"}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}
	tmpl := &models.AnswerTemplate{
		Name:    "winpe-preset",
		Type:    "winpeshl",
		Content: builtinPresets["winpeshl"][0].Content,
	}
	if err := st.CreateAnswerTemplate(t.Context(), tmpl); err != nil {
		t.Fatal(err)
	}
	task := &models.InstallTask{
		ID: "task-preset", HostID: host.ID, DistroName: "Windows PE",
		VersionCodename: "w11", Arch: "amd64", AnswerTemplateID: &tmpl.ID, WIMIndex: 2,
	}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task-preset", nil)
	req.Host = "10.0.0.10:8080" // 非回环 → chooseServerBase 采用请求 Host（真实 PXE 客户端视角）
	req = withChiParams(req, "task_id", "task-preset")
	w := httptest.NewRecorder()
	h.GetAnswerFile(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Result().StatusCode, w.Body.String())
	}
	body := w.Body.String()
	for _, s := range []string{
		`set "PXELAB_HOST=10.0.0.10"`,
		`set "PXELAB_SHARE=isos"`,
		`set "WIMINDEX=2"`,
	} {
		if !strings.Contains(body, s) {
			t.Fatalf("install.bat 缺少 %q:\n%s", s, body)
		}
	}
	// winpeshl.ini 请求只回 INI 包装，不含模板内容
	req2 := withChiParams(httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task-preset?file=winpeshl.ini", nil), "task_id", "task-preset")
	w2 := httptest.NewRecorder()
	h.GetAnswerFile(w2, req2)
	if strings.Contains(w2.Body.String(), "PXELAB_HOST") {
		t.Fatalf("winpeshl.ini 不应包含 install.bat 内容:\n%s", w2.Body.String())
	}
}

// TestAnswerTemplatePreviewFillsWinpeInstallParams Preview 路径同样填充
// winpeshl 预设参数，并支持可选的 smb_server/smb_share/wim_index 覆盖。
func TestAnswerTemplatePreviewFillsWinpeInstallParams(t *testing.T) {
	st := store.NewMemory()
	h := &AnswerTemplateHandler{
		store:      st,
		serverBase: "10.0.0.10:8080",
		cfg:        &config.Config{SMB: config.SMBConfig{Shares: []config.SMBShareConfig{{Name: "isos"}}}},
	}
	tmpl := &models.AnswerTemplate{
		Name:    "pe",
		Type:    "winpeshl",
		Content: builtinPresets["winpeshl"][0].Content,
	}
	if err := st.CreateAnswerTemplate(t.Context(), tmpl); err != nil {
		t.Fatal(err)
	}

	body := `{"wim_index":4,"smb_share":"override","smb_server":"192.168.1.5"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/netboot/answer-templates/1/preview", strings.NewReader(body))
	req = withChiParams(req, "id", "1")
	req.Host = "localhost:8080" // 回环 → 回落 serverBase（但此处 smb_server 已显式覆盖）
	w := httptest.NewRecorder()
	h.Preview(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Result().StatusCode, w.Body.String())
	}
	// 响应是 JSON 包裹（引号转义），只断言不含引号的关键片段
	rendered := w.Body.String()
	for _, s := range []string{"PXELAB_HOST=192.168.1.5", "PXELAB_SHARE=override", "WIMINDEX=4"} {
		if !strings.Contains(rendered, s) {
			t.Fatalf("预览渲染缺少 %q:\n%s", s, rendered)
		}
	}
}
