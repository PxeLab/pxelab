package drivers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupRoot 构造 boot 根目录：
//
//	drivers/nic-intel/{e1d.inf, x64/e1d64.sys}
//	drivers/raid-lsi/megasas.inf
//	drivers/empty/
func setupRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	nic := filepath.Join(root, "drivers", "nic-intel", "x64")
	if err := os.MkdirAll(nic, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "drivers", "nic-intel", "e1d.inf"), []byte("inf"), 0644)
	os.WriteFile(filepath.Join(nic, "e1d64.sys"), []byte("sys"), 0644)
	os.WriteFile(filepath.Join(nic, "e1d64.cat"), []byte("cat"), 0644)
	if err := os.MkdirAll(filepath.Join(root, "drivers", "raid-lsi"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "drivers", "raid-lsi", "megasas.inf"), []byte("inf"), 0644)
	os.MkdirAll(filepath.Join(root, "drivers", "empty"), 0755)
	return root
}

func TestListPackFilesInfoAndProject(t *testing.T) {
	root := setupRoot(t)

	infos, err := ListPackFilesInfo(root, "nic-intel")
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("infos = %d, want 3", len(infos))
	}
	// 排序稳定（按相对路径）
	if infos[0].Rel != "e1d.inf" || infos[1].Rel != "x64/e1d64.cat" || infos[2].Rel != "x64/e1d64.sys" {
		t.Fatalf("顺序错误: %+v", infos)
	}
	for _, fi := range infos {
		if fi.Size != 3 { // 测试写入的都是 3 字节
			t.Fatalf("size = %d, want 3 (%s)", fi.Size, fi.Rel)
		}
	}

	// arch 为空：不过滤，Files = TotalFiles
	all, err := Project(root, "nic-intel", "")
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if all.TotalFiles != 3 || all.Files != 3 || all.TotalBytes != 9 || all.Bytes != 9 {
		t.Fatalf("all projection = %+v", all)
	}
	if all.InfFiles != 1 || strings.Join(all.Archs, ",") != "x64" {
		t.Fatalf("all projection archs/inf = %+v", all)
	}

	// amd64：x64/ 保留 + 顶层通用 → 3 个（本包 x64 全部保留）
	amd, err := Project(root, "nic-intel", "amd64")
	if err != nil {
		t.Fatalf("project amd64: %v", err)
	}
	if amd.Files != 3 {
		t.Fatalf("amd64 files = %d (%v)", amd.Files, amd.FilesList)
	}

	// arm64：x64/ 被过滤 → 仅顶层通用文件
	arm, err := Project(root, "nic-intel", "arm64")
	if err != nil {
		t.Fatalf("project arm64: %v", err)
	}
	if arm.Files != 1 || len(arm.FilesList) != 1 || arm.FilesList[0] != "e1d.inf" {
		t.Fatalf("arm64 files = %d (%v)", arm.Files, arm.FilesList)
	}
	if arm.Bytes != 3 {
		t.Fatalf("arm64 bytes = %d, want 3", arm.Bytes)
	}

	// 坏包报错
	if _, err := Project(root, "no-such", "amd64"); err == nil {
		t.Fatal("不存在的包应报错")
	}
}

func TestPackBytes(t *testing.T) {
	if got := PackBytes([]FileInfo{{Size: 1}, {Size: 2}, {Size: 3}}); got != 6 {
		t.Fatalf("PackBytes = %d, want 6", got)
	}
	if got := PackBytes(nil); got != 0 {
		t.Fatalf("PackBytes(nil) = %d", got)
	}
}

// TestBatchLiteralEscaping：文件名/子目录名含 `%` 时必须转义为 `%%`
// （cmd 在引号内也做百分号展开），否则落地/下载路径写坏且静默失败。
func TestBatchLiteralEscaping(t *testing.T) {
	// http 块：文件名与子目录名含 %
	http := PeInstallBlockHTTP("10.0.0.10:8080", "nic-intel", []string{"net%20card.inf", "x%64/e1d.sys"})
	if !strings.Contains(http, `%_pxelab_drv%\net%%20card.inf`) {
		t.Fatalf("HTTP 块文件名未转义:\n%s", http)
	}
	if !strings.Contains(http, `md "%_pxelab_drv%\x%%64"`) {
		t.Fatalf("HTTP 块子目录未转义:\n%s", http)
	}
	// 变量引用本身不能被转义
	if !strings.Contains(http, `set "_pxelab_drv=X:\pxelab-drivers\nic-intel"`) {
		t.Fatalf("变量引用被误转义:\n%s", http)
	}
	// URL 侧应走 URL 转义（% → %25），而不是 %%
	if !strings.Contains(http, "http://10.0.0.10:8080/boot/drivers/nic-intel/net%2520card.inf") {
		t.Fatalf("URL 转义应使用 %%25:\n%s", http)
	}

	// initrd 块：目标路径含真实文件名（issue #4 里“initrd 不受影响”的结论不准确）
	inj := []Injection{{Pack: "nic-intel", Rel: "x64/ne%t.sys", Inject: "pxdrv_abcd1234.sys"}}
	initrd := PeInstallBlockInitrd(inj)
	if !strings.Contains(initrd, `"%_pxelab_drv%\x64\ne%%t.sys"`) {
		t.Fatalf("initrd 块目标路径未转义:\n%s", initrd)
	}
	if !strings.Contains(initrd, `"%SYSTEMROOT%\System32\pxdrv_abcd1234.sys"`) {
		t.Fatalf("initrd 注入名不应被改写:\n%s", initrd)
	}
}

func TestNormalizeDelivery(t *testing.T) {
	cases := map[string]string{
		"":         DeliveryHTTP,
		"http":     DeliveryHTTP,
		"HTTP":     DeliveryHTTP,
		" initrd ": DeliveryInitrd,
		"INITRD":   DeliveryInitrd,
		"weird":    DeliveryHTTP,
	}
	for in, want := range cases {
		if got := NormalizeDelivery(in); got != want {
			t.Errorf("NormalizeDelivery(%q) = %q, want %q", in, got, want)
		}
	}
	if !ValidDelivery("") || !ValidDelivery("http") || !ValidDelivery("initrd") {
		t.Fatal("合法值应通过")
	}
	if ValidDelivery("smb") {
		t.Fatal("非法值应被拒")
	}
}

func TestListPackFiles(t *testing.T) {
	root := setupRoot(t)
	files, err := ListPackFiles(root, "nic-intel")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"e1d.inf", "x64/e1d64.cat", "x64/e1d64.sys"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Fatalf("files = %v, want %v", files, want)
	}
	if _, err := ListPackFiles(root, "../etc"); err == nil {
		t.Fatal("非法包名应报错")
	}
	if _, err := ListPackFiles(root, "empty"); err == nil {
		t.Fatal("空包应报错")
	}
	if _, err := ListPackFiles(root, "missing"); err == nil {
		t.Fatal("不存在的包应报错")
	}
}

func TestFilterByArch(t *testing.T) {
	files := []string{"readme.txt", "x64/a.inf", "x64/a.sys", "x86/a.inf", "arm64/a.inf", "common/b.inf"}
	got := FilterByArch(files, "amd64")
	want := []string{"readme.txt", "x64/a.inf", "x64/a.sys", "common/b.inf"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("amd64 = %v, want %v", got, want)
	}
	// 扁平布局原样返回
	flat := []string{"a.inf", "sub/b.sys"}
	if got := FilterByArch(flat, "amd64"); strings.Join(got, ",") != strings.Join(flat, ",") {
		t.Fatalf("flat 应原样返回, got %v", got)
	}
	// 无法识别架构原样返回
	if got := FilterByArch(files, "mips"); strings.Join(got, ",") != strings.Join(files, ",") {
		t.Fatalf("unknown arch 应原样返回, got %v", got)
	}
}

func TestPackArchs(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{"dedup+sort", []string{"a.inf", "x64/a.sys", "x86_64/b.sys", "amd64/c.sys", "x86/d.sys", "arm64/e.sys"}, "x64,x86,arm64"},
		{"x64 only", []string{"x64/a.inf"}, "x64"},
		{"flat", []string{"a.inf", "sub/b.sys"}, ""},
		{"mixed normalize", []string{"AMD64/a.inf", "aarch64/b.inf"}, "x64,arm64"},
	}
	for _, c := range cases {
		if got := strings.Join(PackArchs(c.files), ","); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestEffectivePacks(t *testing.T) {
	if got := EffectivePacks([]string{"h1"}, []string{"p1"}); len(got) != 1 || got[0] != "h1" {
		t.Fatalf("主机优先: %v", got)
	}
	if got := EffectivePacks(nil, []string{"p1"}); len(got) != 1 || got[0] != "p1" {
		t.Fatalf("继承 Profile: %v", got)
	}
	if got := EffectivePacks(nil, nil); got != nil {
		t.Fatalf("皆空 → nil: %v", got)
	}
}

func TestInjectNameDeterministic(t *testing.T) {
	a := InjectName("nic-intel", "x64/e1d64.sys")
	b := InjectName("nic-intel", "x64/e1d64.sys")
	if a != b {
		t.Fatalf("应确定性: %s != %s", a, b)
	}
	if !strings.HasPrefix(a, "pxdrv_") || !strings.HasSuffix(a, ".sys") {
		t.Fatalf("命名格式不符: %s", a)
	}
	// 不同文件不得碰撞
	if InjectName("nic-intel", "x64/e1d64.sys") == InjectName("nic-intel", "x64/e1d64.cat") {
		t.Fatal("不同文件不应同名")
	}
	if InjectName("a", "b/c") == InjectName("a/b", "c") {
		t.Fatal("pack/rel 边界不应歧义")
	}
	// 无扩展名文件也安全
	if InjectName("p", "README") != InjectName("p", "README") {
		t.Fatal("无扩展名应稳定")
	}
}

func TestPlanInjections(t *testing.T) {
	root := setupRoot(t)

	// 有效：amd64 过滤后保留顶层 e1d.inf + x64/*
	inj, errs := PlanInjections(root, []string{"nic-intel"}, "amd64")
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	if len(inj) != 3 {
		t.Fatalf("injections = %d, want 3 (%v)", len(inj), inj)
	}
	for _, in := range inj {
		if in.Pack != "nic-intel" || in.Inject == "" {
			t.Fatalf("bad injection: %+v", in)
		}
	}

	// 混合：坏包跳过并报错，好包仍保留
	inj2, errs2 := PlanInjections(root, []string{"no-such", "raid-lsi"}, "amd64")
	if len(errs2) != 1 {
		t.Fatalf("应跳过 1 个坏包, errs = %v", errs2)
	}
	if len(inj2) != 1 || inj2[0].Pack != "raid-lsi" {
		t.Fatalf("好包应保留: %+v", inj2)
	}

	// 架构无匹配：nic-intel 有 x64 子目录但目标是 arm64 → 仅顶层通用文件保留 → 非空
	inj3, _ := PlanInjections(root, []string{"nic-intel"}, "arm64")
	if len(inj3) != 1 || inj3[0].Rel != "e1d.inf" {
		t.Fatalf("arm64 应仅保留顶层通用文件: %+v", inj3)
	}
}

func TestInitrdLines(t *testing.T) {
	inj := []Injection{
		{Pack: "nic-intel", Rel: "x64/e1d 64.sys", Inject: "pxdrv_abc.sys"},
		{Pack: "nic-intel", Rel: "e1d.inf", Inject: "pxdrv_def.inf"},
	}
	lines := InitrdLines("10.0.0.10:8080", inj)
	if len(lines) != 2 {
		t.Fatalf("lines = %v", lines)
	}
	if lines[0] != "initrd -n pxdrv_abc.sys http://10.0.0.10:8080/boot/drivers/nic-intel/x64/e1d%2064.sys" {
		t.Fatalf("line[0] = %q", lines[0])
	}
	if lines[1] != "initrd -n pxdrv_def.inf http://10.0.0.10:8080/boot/drivers/nic-intel/e1d.inf" {
		t.Fatalf("line[1] = %q", lines[1])
	}
}

func TestPeInstallBlockInitrd(t *testing.T) {
	inj := []Injection{
		{Pack: "nic-intel", Rel: "e1d.inf", Inject: "pxdrv_aaa.inf"},
		{Pack: "nic-intel", Rel: "x64/e1d64.sys", Inject: "pxdrv_bbb.sys"},
	}
	block := PeInstallBlockInitrd(inj)
	mustContain := []string{
		PullMarker,
		`set "_pxelab_drv=X:\pxelab-drivers\nic-intel"`,
		`md "%_pxelab_drv%\x64"`,
		`copy /y "%SYSTEMROOT%\System32\pxdrv_aaa.inf" "%_pxelab_drv%\e1d.inf"`,
		`copy /y "%SYSTEMROOT%\System32\pxdrv_bbb.sys" "%_pxelab_drv%\x64\e1d64.sys"`,
		`pnputil /add-driver "%_pxelab_drv%\*.inf" /subdirs /install`,
		`drvload "%%i"`,
	}
	for _, s := range mustContain {
		if !strings.Contains(block, s) {
			t.Fatalf("块缺少 %q:\n%s", s, block)
		}
	}
	// 空计划 → 空字符串
	if PeInstallBlockInitrd(nil) != "" {
		t.Fatal("空计划应为空字符串")
	}
}

func TestPeInstallBlockHTTP(t *testing.T) {
	block := PeInstallBlockHTTP("192.168.1.10:8080", "nic-intel", []string{"e1d.inf", "x64/e1d64.sys"})
	mustContain := []string{
		PullMarker,
		`set "_pxelab_drv=X:\pxelab-drivers\nic-intel"`,
		`where curl.exe >nul 2>nul && set "_pxelab_dl=curl"`,
		`certutil -urlcache -split -f "http://192.168.1.10:8080/boot/drivers/nic-intel/x64/e1d64.sys"`,
		`pnputil /add-driver "%_pxelab_drv%\*.inf" /subdirs /install`,
	}
	for _, s := range mustContain {
		if !strings.Contains(block, s) {
			t.Fatalf("块缺少 %q:\n%s", s, block)
		}
	}
}

// TestInitrdPlanConsistencyWithInstallBlock 关键不变量：同一 root+packs+arch 下，
// iPXE 菜单（InitrdLines）注入的文件名，与 install.bat 落地块（PeInstallBlockInitrd）
// 引用的源文件名必须完全一致 —— 否则 PE 里 copy 不到文件，驱动静默丢失。
func TestInitrdPlanConsistencyWithInstallBlock(t *testing.T) {
	root := setupRoot(t)
	inj, _ := PlanInjections(root, []string{"nic-intel", "raid-lsi"}, "amd64")
	lines := strings.Join(InitrdLines("10.0.0.10:8080", inj), "\n")
	block := PeInstallBlockInitrd(inj)

	for _, in := range inj {
		// 菜单里注入
		if !strings.Contains(lines, "initrd -n "+in.Inject+" ") {
			t.Fatalf("菜单缺少注入名 %s:\n%s", in.Inject, lines)
		}
		// install.bat 里从 System32 取用同名文件
		if !strings.Contains(block, `"%SYSTEMROOT%\System32\`+in.Inject+`"`) {
			t.Fatalf("install.bat 缺少源文件 %s:\n%s", in.Inject, block)
		}
	}
}
