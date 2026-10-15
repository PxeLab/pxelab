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
	"github.com/pxelab/pxelab/internal/drivers"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/store"
)

// setupDriverBootRoot 构造 boot 根目录：
//
//	drivers/nic-intel/{e1d.inf, x64/e1d64.sys}  合法包（含架构子目录）
//	drivers/flat-pack/chipset.inf                合法包（扁平布局，无架构子目录）
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
	flat := filepath.Join(root, "drivers", "flat-pack")
	if err := os.MkdirAll(flat, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flat, "chipset.inf"), []byte("inf"), 0644); err != nil {
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
	if len(resp.Data.Packages) != 2 {
		t.Fatalf("packages = %v, want 2 个（空包跳过）", resp.Data.Packages)
	}
	// Glob 字典序：flat-pack 在前，nic-intel 在后
	f := resp.Data.Packages[0]
	if f.Name != "flat-pack" || f.Files != 1 || f.InfFiles != 1 {
		t.Fatalf("package = %+v", f)
	}
	// 扁平布局 → archs = []（全架构通用）
	if len(f.Archs) != 0 {
		t.Fatalf("flat-pack archs = %v, want []", f.Archs)
	}
	p := resp.Data.Packages[1]
	if p.Name != "nic-intel" || p.Files != 2 || p.InfFiles != 1 {
		t.Fatalf("package = %+v", p)
	}
	// nic-intel 含 x64/ 子目录 → archs = ["x64"]（顶层 e1d.inf 是通用文件）
	if strings.Join(p.Archs, ",") != "x64" {
		t.Fatalf("nic-intel archs = %v, want [x64]", p.Archs)
	}
}

func TestDriverPackArchs(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{"arch subdirs dedup+sort", []string{
			"a.inf", "x64/a.sys", "x86_64/b.sys", "amd64/c.sys", "x86/d.sys", "arm64/e.sys", "common/f.dat",
		}, "x64,x86,arm64"},
		{"x64 only", []string{"x64/a.inf"}, "x64"},
		{"flat layout", []string{"a.inf", "sub/b.sys"}, ""},
		{"non-arch dirs ignored", []string{"win7/a.inf", "w11/b.inf"}, ""},
		{"mixed normalize", []string{"AMD64/a.inf", "aarch64/b.inf"}, "x64,arm64"},
	}
	for _, c := range cases {
		if got := strings.Join(driverPackArchs(c.files), ","); got != c.want {
			t.Errorf("%s: driverPackArchs = %q, want %q", c.name, got, c.want)
		}
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

func TestProfileDriverPacksJSONRoundTrip(t *testing.T) {
	// B2：Profile 默认驱动包与 Host 同构（JSON 数组）
	var p models.Profile
	if err := json.Unmarshal([]byte(`{"name":"lab","driver_packs":["nic-intel","raid-lsi"]}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	packs, err := p.GetDriverPacks()
	if err != nil || len(packs) != 2 || packs[1] != "raid-lsi" {
		t.Fatalf("packs = %v, err = %v", packs, err)
	}
	out, err := json.Marshal(&p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"driver_packs":["nic-intel","raid-lsi"]`) {
		t.Fatalf("marshal 后 driver_packs 错误: %s", out)
	}
	// 未设置时应序列化为空数组
	var empty models.Profile
	out2, _ := json.Marshal(&empty)
	if !strings.Contains(string(out2), `"driver_packs":[]`) {
		t.Fatalf("空 Profile driver_packs 应为 []: %s", out2)
	}
	// SetDriverPacks(nil) 清空
	if err := p.SetDriverPacks(nil); err != nil || p.DriverPacks != "" {
		t.Fatalf("清空失败: %q %v", p.DriverPacks, err)
	}
}

// TestResolveDriverPacksInheritsProfile B2：主机未绑定驱动包时继承所属 Profile
// 的默认驱动包；主机显式绑定优先；无 Profile 时为空。
func TestResolveDriverPacksInheritsProfile(t *testing.T) {
	st := store.NewMemory()
	h := &InstallTaskHandler{store: st}

	prof := &models.Profile{ID: "prof-1", Name: "lab"}
	if err := prof.SetDriverPacks([]string{"nic-intel", "raid-lsi"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProfile(t.Context(), prof); err != nil {
		t.Fatal(err)
	}
	profID := prof.ID

	// 主机无驱动包 + 关联 Profile → 继承
	hostInherit := &models.Host{ID: "h-inherit", Name: "h1", MAC: "aa:bb:cc:dd:ee:10", ProfileID: &profID}
	if err := st.CreateHost(t.Context(), hostInherit); err != nil {
		t.Fatal(err)
	}
	got := h.resolveDriverPacks(t.Context(), hostInherit)
	if len(got) != 2 || got[0] != "nic-intel" || got[1] != "raid-lsi" {
		t.Fatalf("继承结果 = %v, want [nic-intel raid-lsi]", got)
	}

	// 主机显式绑定 → 主机优先，不继承
	hostOwn := &models.Host{ID: "h-own", Name: "h2", MAC: "aa:bb:cc:dd:ee:11", ProfileID: &profID}
	if err := hostOwn.SetDriverPacks([]string{"chipset"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateHost(t.Context(), hostOwn); err != nil {
		t.Fatal(err)
	}
	got = h.resolveDriverPacks(t.Context(), hostOwn)
	if len(got) != 1 || got[0] != "chipset" {
		t.Fatalf("主机优先结果 = %v, want [chipset]", got)
	}

	// 主机无驱动包 + 无 Profile → nil
	hostBare := &models.Host{ID: "h-bare", Name: "h3", MAC: "aa:bb:cc:dd:ee:12"}
	if err := st.CreateHost(t.Context(), hostBare); err != nil {
		t.Fatal(err)
	}
	if got := h.resolveDriverPacks(t.Context(), hostBare); len(got) != 0 {
		t.Fatalf("无 Profile 应为空, got %v", got)
	}

	// nil host 安全
	if got := h.resolveDriverPacks(t.Context(), nil); len(got) != 0 {
		t.Fatalf("nil host 应为空, got %v", got)
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

// TestGetAnswerFileWinpeShellInitrdDelivery：driver_delivery=initrd 时，winpeshl
// install.bat 顶部应插入「从 %SYSTEMROOT%\System32 落地 + pnputil」块，且不含 HTTP 下载。
func TestGetAnswerFileWinpeShellInitrdDelivery(t *testing.T) {
	root := setupDriverBootRoot(t)
	st := store.NewMemory()
	cfg := &config.Config{}
	cfg.Netboot.DriverDelivery = "initrd"
	h := &InstallTaskHandler{
		store:      st,
		serverBase: "10.0.0.10:8080",
		cfg:        cfg,
		bootFS:     boot.NewBootFileServer(root),
	}
	host := &models.Host{ID: "host-1", Name: "host-1", MAC: "aa:bb:cc:dd:ee:03"}
	if err := host.SetDriverPacks([]string{"nic-intel"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}
	tmpl := &models.AnswerTemplate{
		Name:    "winpe",
		Type:    "winpeshl",
		Content: "@echo off\r\nsetup.exe /unattend:X:\\autounattend.xml\r\n",
	}
	if err := st.CreateAnswerTemplate(t.Context(), tmpl); err != nil {
		t.Fatal(err)
	}
	task := &models.InstallTask{
		ID: "task-pe-initrd", HostID: host.ID, DistroName: "Windows PE",
		VersionCodename: "w11", Arch: "amd64", AnswerTemplateID: &tmpl.ID,
	}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatal(err)
	}

	req := withChiParams(httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task-pe-initrd", nil), "task_id", "task-pe-initrd")
	w := httptest.NewRecorder()
	h.GetAnswerFile(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Result().StatusCode, w.Body.String())
	}
	body := w.Body.String()

	// initrd 落地块：源文件名与 drivers.InjectName 一致（与 iPXE 菜单同源）
	infName := drivers.InjectName("nic-intel", "e1d.inf")
	sysName := drivers.InjectName("nic-intel", "x64/e1d64.sys")
	mustContain := []string{
		drivers.PullMarker,
		`copy /y "%SYSTEMROOT%\System32\` + infName + `" "%_pxelab_drv%\e1d.inf"`,
		`copy /y "%SYSTEMROOT%\System32\` + sysName + `" "%_pxelab_drv%\x64\e1d64.sys"`,
		`pnputil /add-driver "%_pxelab_drv%\*.inf" /subdirs /install`,
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Fatalf("install.bat 缺少 %q:\n%s", s, body)
		}
	}
	// initrd 模式不应走 HTTP 下载
	if strings.Contains(body, "curl.exe") || strings.Contains(body, "certutil") {
		t.Fatalf("initrd 模式不应含 HTTP 下载命令:\n%s", body)
	}
	// 驱动块应位于 setup.exe 之前，且保留首行 @echo off
	if strings.Index(body, drivers.PullMarker) > strings.Index(body, "setup.exe") {
		t.Fatalf("驱动落地块应在 setup.exe 之前:\n%s", body)
	}
	if !strings.HasPrefix(body, "@echo off\r\n") {
		t.Fatalf("应保留首行 @echo off:\n%q", body)
	}
}

// ── 借用项 2：PE 阶段运行时驱动加载 ──

func TestFilterDriverFilesByArch(t *testing.T) {
	files := []string{"readme.txt", "x64/a.inf", "x64/a.sys", "x86/a.inf", "arm64/a.inf", "common/b.inf"}
	got := filterDriverFilesByArch(files, "amd64")
	want := []string{"readme.txt", "x64/a.inf", "x64/a.sys", "common/b.inf"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("amd64 filter = %v, want %v", got, want)
	}
	// 无架构子目录（扁平布局）→ 原样返回，避免误伤老包
	flat := []string{"a.inf", "sub/b.sys"}
	if got := filterDriverFilesByArch(flat, "amd64"); strings.Join(got, ",") != strings.Join(flat, ",") {
		t.Fatalf("flat pack should pass through, got %v", got)
	}
	// 无法识别架构 → 原样返回
	if got := filterDriverFilesByArch(files, "mips"); strings.Join(got, ",") != strings.Join(files, ",") {
		t.Fatalf("unknown arch should pass through, got %v", got)
	}
}

func TestBuildPeDriverInstallBlock(t *testing.T) {
	block := buildPeDriverInstallBlock("192.168.1.10:8080", "nic-intel", []string{"e1d.inf", "x64/e1d64.sys"})
	mustContain := []string{
		"pxelab-driver-pull",
		`set "_pxelab_drv=X:\pxelab-drivers\nic-intel"`,
		`md "%_pxelab_drv%\x64"`,
		`where curl.exe >nul 2>nul && set "_pxelab_dl=curl"`,
		`curl.exe -fsSL -o "%_pxelab_drv%\e1d.inf" "http://192.168.1.10:8080/boot/drivers/nic-intel/e1d.inf"`,
		`certutil -urlcache -split -f "http://192.168.1.10:8080/boot/drivers/nic-intel/x64/e1d64.sys" "%_pxelab_drv%\x64\e1d64.sys"`,
		`pnputil /add-driver "%_pxelab_drv%\*.inf" /subdirs /install`,
		`drvload "%%i"`,
	}
	for _, s := range mustContain {
		if !strings.Contains(block, s) {
			t.Fatalf("PE 驱动块缺少 %q:\n%s", s, block)
		}
	}
}

// TestGetAnswerFileWinpeShellInjectsDrivers 端到端：模板 type=winpeshl + 主机绑定驱动包，
// install.bat 顶部应注入 PE 驱动块（位于 setup.exe 之前）；winpeshl.ini 请求不注入。
func TestGetAnswerFileWinpeShellInjectsDrivers(t *testing.T) {
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
	tmpl := &models.AnswerTemplate{
		Name:    "winpe",
		Type:    "winpeshl",
		Content: "@echo off\r\nsetup.exe /unattend:X:\\autounattend.xml\r\n",
	}
	if err := st.CreateAnswerTemplate(t.Context(), tmpl); err != nil {
		t.Fatal(err)
	}
	task := &models.InstallTask{
		ID: "task-pe", HostID: host.ID, DistroName: "Windows PE",
		VersionCodename: "w11", Arch: "amd64", AnswerTemplateID: &tmpl.ID,
	}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatal(err)
	}

	// install.bat（无 file 参数）→ 顶部注入驱动块，且位于 setup.exe 之前
	req := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task-pe", nil)
	req = withChiParams(req, "task_id", "task-pe")
	w := httptest.NewRecorder()
	h.GetAnswerFile(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	body := w.Body.String()
	if !strings.Contains(body, "pxelab-driver-pull") {
		t.Fatalf("install.bat 缺少驱动块:\n%s", body)
	}
	if !strings.Contains(body, "boot/drivers/nic-intel/e1d.inf") {
		t.Fatalf("install.bat 缺少驱动下载 URL:\n%s", body)
	}
	if strings.Index(body, "pxelab-driver-pull") > strings.Index(body, "setup.exe") {
		t.Fatalf("驱动块应在 setup.exe 之前:\n%s", body)
	}
	if !strings.HasPrefix(body, "@echo off\r\n") {
		t.Fatalf("应保留首行 @echo off:\n%q", body)
	}

	// winpeshl.ini 请求 → 只回 INI 包装，不注入驱动
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task-pe?file=winpeshl.ini", nil)
	req2 = withChiParams(req2, "task_id", "task-pe")
	w2 := httptest.NewRecorder()
	h.GetAnswerFile(w2, req2)
	ini := w2.Body.String()
	if strings.Contains(ini, "pxelab-driver-pull") {
		t.Fatalf("winpeshl.ini 不应包含驱动块:\n%s", ini)
	}
	if !strings.Contains(ini, "%SYSTEMROOT%\\System32\\install.bat") {
		t.Fatalf("winpeshl.ini 包装错误:\n%s", ini)
	}
}

// TestGetAnswerFileWinpeShellViaOverlayAnswerType：模板 type=autounattend 但 overlay
// 版本 answer_type=winpeshl（真实配置路径）时，也应触发 PE 驱动注入。
func TestGetAnswerFileWinpeShellViaOverlayAnswerType(t *testing.T) {
	root := setupDriverBootRoot(t)
	st := store.NewMemory()
	h := &InstallTaskHandler{
		store:      st,
		serverBase: "10.0.0.10:8080",
		cfg:        &config.Config{},
		bootFS:     boot.NewBootFileServer(root),
	}
	host := &models.Host{ID: "host-1", Name: "host-1", MAC: "aa:bb:cc:dd:ee:01"}
	if err := host.SetDriverPacks([]string{"nic-intel"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateHost(t.Context(), host); err != nil {
		t.Fatal(err)
	}
	ov := &models.NetbootOverlay{DistroName: "Windows PE", Enabled: true}
	if err := ov.SetVersionOverrides([]models.VersionOverride{{Codename: "w11", AnswerType: "winpeshl"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertNetbootOverlay(t.Context(), ov); err != nil {
		t.Fatal(err)
	}
	tmpl := &models.AnswerTemplate{Name: "pe", Type: "autounattend", Content: "@echo off\nsetup.exe\n"}
	if err := st.CreateAnswerTemplate(t.Context(), tmpl); err != nil {
		t.Fatal(err)
	}
	task := &models.InstallTask{
		ID: "task-ov", HostID: host.ID, DistroName: "Windows PE",
		VersionCodename: "w11", Arch: "amd64", AnswerTemplateID: &tmpl.ID,
	}
	if err := st.CreateInstallTask(t.Context(), task); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/netboot/answer/task-ov", nil)
	req = withChiParams(req, "task_id", "task-ov")
	w := httptest.NewRecorder()
	h.GetAnswerFile(w, req)
	if !strings.Contains(w.Body.String(), "pxelab-driver-pull") {
		t.Fatalf("overlay answer_type=winpeshl 应触发 PE 驱动注入:\n%s", w.Body.String())
	}
}
