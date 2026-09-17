package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

// setupDriverBootRoot 构造 boot 根目录：
//
//	drivers/nic-intel/{e1d.inf, x64/e1d64.sys}  合法包
//	drivers/empty-pack/                          空包（应被跳过）
func setupDriverBootRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	nic := filepath.Join(root, "drivers", "nic-intel")
	if err := os.MkdirAll(filepath.Join(nic, "x64"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nic, "e1d.inf"), []byte("inf"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nic, "x64", "e1d64.sys"), []byte("sys"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "drivers", "empty-pack"), 0755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestListDriverPackFiles(t *testing.T) {
	root := setupDriverBootRoot(t)

	files, err := listDriverPackFiles(root, "nic-intel")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"e1d.inf", "x64/e1d64.sys"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Fatalf("files = %v, want %v", files, want)
	}

	if _, err := listDriverPackFiles(root, "../etc"); err == nil {
		t.Fatal("非法包名应报错")
	}
	if _, err := listDriverPackFiles(root, "empty-pack"); err == nil {
		t.Fatal("空包应报错")
	}
	if _, err := listDriverPackFiles(root, "missing"); err == nil {
		t.Fatal("不存在的包应报错")
	}
}

func TestDriverPackageListEndpoint(t *testing.T) {
	root := setupDriverBootRoot(t)
	h := &DriverPackageHandler{bootFS: boot.NewBootFileServer(root)}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/driver-packages", nil)
	w := httptest.NewRecorder()
	h.List(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	var resp struct {
		Data struct {
			Packages []driverPackageDTO `json:"packages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data.Packages) != 1 {
		t.Fatalf("packages = %v, want 1 个（空包跳过）", resp.Data.Packages)
	}
	p := resp.Data.Packages[0]
	if p.Name != "nic-intel" || p.Files != 2 || p.InfFiles != 1 {
		t.Fatalf("package = %+v", p)
	}
}

func TestHostDriverPacksJSONRoundTrip(t *testing.T) {
	var h models.Host
	if err := json.Unmarshal([]byte(`{"name":"n1","driver_packs":["nic-intel","raid-lsi"]}`), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	packs, err := h.GetDriverPacks()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(packs) != 2 || packs[0] != "nic-intel" || packs[1] != "raid-lsi" {
		t.Fatalf("packs = %v", packs)
	}

	out, err := json.Marshal(&h)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	arr, ok := m["driver_packs"].([]any)
	if !ok || len(arr) != 2 {
		t.Fatalf("marshal 后 driver_packs = %v", m["driver_packs"])
	}

	// 未设置时应序列化为空数组而非 null/缺省
	var empty models.Host
	out2, _ := json.Marshal(&empty)
	if !strings.Contains(string(out2), `"driver_packs":[]`) {
		t.Fatalf("空主机 driver_packs 应为 []: %s", out2)
	}
}

func TestBuildDriverInstallCommand(t *testing.T) {
	cmd := buildDriverInstallCommand("192.168.1.10:8080", "nic-intel", []string{"e1d.inf", "x64/e1d64.sys"})

	mustContain := []string{
		`powershell.exe -NoProfile -ExecutionPolicy Bypass`,
		`Join-Path $env:TEMP 'pxelab-drivers\nic-intel'`,
		`New-Item -ItemType Directory -Force (Join-Path $dest 'x64')`,
		`$wc.DownloadFile('http://192.168.1.10:8080/boot/drivers/nic-intel/e1d.inf', (Join-Path $dest 'e1d.inf'))`,
		`$wc.DownloadFile('http://192.168.1.10:8080/boot/drivers/nic-intel/x64/e1d64.sys', (Join-Path $dest 'x64\e1d64.sys'))`,
		`pnputil /add-driver ($dest + '\*.inf') /subdirs /install`,
		`} catch {}"`,
	}
	for _, s := range mustContain {
		if !strings.Contains(cmd, s) {
			t.Fatalf("命令缺少 %q:\n%s", s, cmd)
		}
	}
}

const testUnattendXML = `<?xml version="1.0" encoding="utf-8"?>
<unattend xmlns="urn:schemas-microsoft-com:unattend">
<settings pass="oobeSystem">
<component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64">
<OOBE><HideEULAPage>true</HideEULAPage></OOBE>
</component>
</settings>
</unattend>`

func TestAugmentAutoUnattendFirstLogonBaselineOnlyMatchesLegacy(t *testing.T) {
	hooks := config.BaselineHooksConfig{}
	psURL := buildPullURL("10.0.0.10:8080", "ps1", "mac=aa")

	legacy, ok1 := augmentAutoUnattend(testUnattendXML, psURL, hooks)
	combined, ok2 := augmentAutoUnattendFirstLogon(testUnattendXML, psURL, nil, true, hooks)
	if !ok1 || !ok2 {
		t.Fatalf("ok1=%v ok2=%v", ok1, ok2)
	}
	if legacy != combined {
		t.Fatalf("基线独占时编排版应与旧路径字节一致\nlegacy:\n%s\ncombined:\n%s", legacy, combined)
	}
}

func TestAugmentAutoUnattendFirstLogonWithDrivers(t *testing.T) {
	hooks := config.BaselineHooksConfig{}
	psURL := buildPullURL("10.0.0.10:8080", "ps1", "mac=aa")
	drivers := []driverCommand{
		{name: "nic-intel", cmd: buildDriverInstallCommand("10.0.0.10:8080", "nic-intel", []string{"e1d.inf"})},
		{name: "raid-lsi", cmd: buildDriverInstallCommand("10.0.0.10:8080", "raid-lsi", []string{"megasas.inf"})},
	}

	out, ok := augmentAutoUnattendFirstLogon(testUnattendXML, psURL, drivers, true, hooks)
	if !ok {
		t.Fatal("应注入成功")
	}
	// 顺序：驱动 Order 1/2，基线 Order 3
	idxNic := strings.Index(out, "pxelab-driver-pull): nic-intel")
	idxRaid := strings.Index(out, "pxelab-driver-pull): raid-lsi")
	idxBaseline := strings.Index(out, "PxeLab baseline pull")
	if idxNic < 0 || idxRaid < 0 || idxBaseline < 0 {
		t.Fatalf("缺少编排条目:\n%s", out)
	}
	if !(idxNic < idxRaid && idxRaid < idxBaseline) {
		t.Fatalf("编排顺序错误（应先驱动后基线）:\n%s", out)
	}
	if !strings.Contains(out, "<Order>3</Order>") {
		t.Fatalf("基线应为 Order 3:\n%s", out)
	}
	// XML 转义：命令中的 " 应转义为 &quot;
	if !strings.Contains(out, "&quot;") {
		t.Fatalf("命令应做 XML 转义:\n%s", out)
	}
	if strings.Contains(out, `Bypass -Command "try`) {
		t.Fatalf("未转义的引号不应出现在 XML 中:\n%s", out)
	}

	// 幂等：再次注入应原样返回
	out2, ok2 := augmentAutoUnattendFirstLogon(out, psURL, drivers, true, hooks)
	if !ok2 || out2 != out {
		t.Fatal("重复注入应幂等")
	}
}

func TestAugmentAutoUnattendFirstLogonDriversOnly(t *testing.T) {
	drivers := []driverCommand{
		{name: "nic-intel", cmd: buildDriverInstallCommand("10.0.0.10:8080", "nic-intel", []string{"e1d.inf"})},
	}
	out, ok := augmentAutoUnattendFirstLogon(testUnattendXML, "", drivers, false, config.BaselineHooksConfig{})
	if !ok {
		t.Fatal("仅驱动（无基线）也应注入")
	}
	if !strings.Contains(out, "pxelab-driver-pull") { // 驱动标记
		t.Fatalf("缺少驱动条目:\n%s", out)
	}
	if strings.Contains(out, "PxeLab baseline pull") {
		t.Fatalf("不应包含基线条目:\n%s", out)
	}
	if !strings.Contains(out, "<Order>1</Order>") {
		t.Fatalf("驱动应为 Order 1:\n%s", out)
	}
}

func TestAugmentAutoUnattendFirstLogonUnsupported(t *testing.T) {
	// 无 oobeSystem/组件 → false
	if _, ok := augmentAutoUnattendFirstLogon("<unattend/>", "u", nil, true, config.BaselineHooksConfig{}); ok {
		t.Fatal("缺少组件时应返回 false")
	}
	// 已有 FirstLogonCommands → false
	withFLC := strings.Replace(testUnattendXML, "<OOBE>", "<FirstLogonCommands></FirstLogonCommands><OOBE>", 1)
	if _, ok := augmentAutoUnattendFirstLogon(withFLC, "u", nil, true, config.BaselineHooksConfig{}); ok {
		t.Fatal("已有 FirstLogonCommands 时应返回 false")
	}
	// 无驱动且无基线 → false
	if _, ok := augmentAutoUnattendFirstLogon(testUnattendXML, "", nil, false, config.BaselineHooksConfig{}); ok {
		t.Fatal("无事可排时应返回 false")
	}
}

// TestGetAnswerFileInjectsDriverCommands 端到端：主机绑定驱动包 + autounattend 模板，
// 拉取应答文件应注入首登编排（驱动 + 基线）。
func TestGetAnswerFileInjectsDriverCommands(t *testing.T) {
	root := setupDriverBootRoot(t)
	st := store.NewMemory()
	h := &InstallTaskHandler{
		store:      st,
		serverBase: "10.0.0.10:8080",
		cfg:        &config.Config{},
		bootFS:     boot.NewBootFileServer(root),
	}

	host := &models.Host{ID: "host-1", Name: "host-1", MAC: "aa:bb:cc:dd:ee:ff"}
	if err := host.SetDriverPacks([]string{"nic-intel"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}
	// 内存存储会重写 host.ID，任务需引用实际 ID
	tmpl := &models.AnswerTemplate{
		Name:               "win",
		Type:               "autounattend",
		Content:            testUnattendXML,
		EnableBaselinePull: true,
	}
	if err := st.CreateAnswerTemplate(t.Context(), tmpl); err != nil {
		t.Fatal(err)
	}
	task := &models.InstallTask{
		ID:               "task-1",
		HostID:           host.ID,
		DistroName:       "Windows",
		VersionCodename:  "w11",
		Arch:             "amd64",
		AnswerTemplateID: &tmpl.ID,
	}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task-1", nil)
	req = withChiParams(req, "task_id", "task-1")
	w := httptest.NewRecorder()
	h.GetAnswerFile(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "pxelab-driver-pull") { // 驱动编排
		t.Fatalf("应答文件缺少驱动编排:\n%s", body)
	}
	if !strings.Contains(body, "boot/drivers/nic-intel/e1d.inf") {
		t.Fatalf("应答文件缺少驱动下载 URL:\n%s", body)
	}
	if !strings.Contains(body, "PxeLab baseline pull") {
		t.Fatalf("应答文件缺少基线钩子:\n%s", body)
	}
	if strings.Index(body, "pxelab-driver-pull") > strings.Index(body, "PxeLab baseline pull") {
		t.Fatal("驱动应先于基线执行")
	}
}
