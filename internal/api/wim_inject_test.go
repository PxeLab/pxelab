package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/osimage"
	"github.com/pxelab/pxelab/internal/store"
)

// wimInjectTestEnv 构造带 fake 提取目录（sources/boot.wim 占位文件）与
// 驱动包 boot 树的测试环境。不执行任何 dism/PowerShell —— 仅覆盖校验与
// 纯函数路径；真机注入路径属硬件验证范围。
func wimInjectTestEnv(t *testing.T) (*OSImageHandler, store.Interface, *models.OSImage) {
	t.Helper()
	root := setupDriverBootRoot(t) // drivers/nic-intel/{e1d.inf, x64/e1d64.sys}
	st := store.NewMemory()
	h := &OSImageHandler{
		store:  st,
		config: nil,
		bootFS: boot.NewBootFileServer(root),
	}
	extractDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(extractDir, "sources"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extractDir, "sources", "boot.wim"), []byte("fake-wim"), 0644); err != nil {
		t.Fatal(err)
	}
	img := &models.OSImage{
		Name: "win11", Filename: "win11.iso", Distro: "windows", Arch: "amd64",
		Status: "ready", ExtractedTo: extractDir,
	}
	if err := st.CreateOSImage(t.Context(), img); err != nil {
		t.Fatal(err)
	}
	return h, st, img
}

func TestWimInjectValidation(t *testing.T) {
	h, _, img := wimInjectTestEnv(t)
	id := strconv.FormatUint(uint64(img.ID), 10)

	cases := []struct {
		name string
		id   string
		body string
		want int
	}{
		{"empty packs", id, `{"driver_packs":[]}`, http.StatusBadRequest},
		{"missing packs field", id, `{}`, http.StatusBadRequest},
		{"bad json", id, `not-json`, http.StatusBadRequest},
		{"unknown pack", id, `{"driver_packs":["no-such-pack"]}`, http.StatusBadRequest},
		{"invalid pack name", id, `{"driver_packs":["../etc"]}`, http.StatusBadRequest},
		{"bad target", id, `{"driver_packs":["nic-intel"],"target":"recovery"}`, http.StatusBadRequest},
		{"missing image id", "9999", `{"driver_packs":["nic-intel"]}`, http.StatusNotFound},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/os-images/"+c.id+"/inject-drivers", strings.NewReader(c.body))
		req = withChiParams(req, "id", c.id)
		w := httptest.NewRecorder()
		h.InjectDrivers(w, req)
		if w.Result().StatusCode != c.want {
			t.Errorf("%s: expected %d, got %d (%s)", c.name, c.want, w.Result().StatusCode, w.Body.String())
		}
	}
}

func TestWimInjectRequiresExtractionAndWim(t *testing.T) {
	root := setupDriverBootRoot(t)
	st := store.NewMemory()
	h := &OSImageHandler{store: st, bootFS: boot.NewBootFileServer(root)}

	// 未提取 → 400
	img1 := &models.OSImage{Name: "not-extracted", Filename: "x.iso", Distro: "windows", Status: "ready"}
	if err := st.CreateOSImage(t.Context(), img1); err != nil {
		t.Fatal(err)
	}
	id1 := strconv.FormatUint(uint64(img1.ID), 10)
	req := withChiParams(httptest.NewRequest(http.MethodPost, "/api/v1/os-images/"+id1+"/inject-drivers",
		strings.NewReader(`{"driver_packs":["nic-intel"]}`)), "id", id1)
	w := httptest.NewRecorder()
	h.InjectDrivers(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("未提取应 400, got %d", w.Result().StatusCode)
	}

	// 已提取但 sources/boot.wim 缺失 → 400
	img2 := &models.OSImage{Name: "no-wim", Filename: "y.iso", Distro: "windows", Status: "ready", ExtractedTo: t.TempDir()}
	if err := st.CreateOSImage(t.Context(), img2); err != nil {
		t.Fatal(err)
	}
	id2 := strconv.FormatUint(uint64(img2.ID), 10)
	req = withChiParams(httptest.NewRequest(http.MethodPost, "/api/v1/os-images/"+id2+"/inject-drivers",
		strings.NewReader(`{"driver_packs":["nic-intel"]}`)), "id", id2)
	w = httptest.NewRecorder()
	h.InjectDrivers(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("boot.wim 缺失应 400, got %d", w.Result().StatusCode)
	}
}

func TestWimInjectBootFSRequired(t *testing.T) {
	st := store.NewMemory()
	h := &OSImageHandler{store: st} // 无 bootFS
	extractDir := t.TempDir()
	os.MkdirAll(filepath.Join(extractDir, "sources"), 0755)
	os.WriteFile(filepath.Join(extractDir, "sources", "boot.wim"), []byte("w"), 0644)
	img := &models.OSImage{
		Name: "win", Filename: "w.iso", Distro: "windows", Status: "ready", ExtractedTo: extractDir,
	}
	if err := st.CreateOSImage(t.Context(), img); err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatUint(uint64(img.ID), 10)
	req := withChiParams(httptest.NewRequest(http.MethodPost, "/api/v1/os-images/"+id+"/inject-drivers",
		strings.NewReader(`{"driver_packs":["nic-intel"]}`)), "id", id)
	w := httptest.NewRecorder()
	h.InjectDrivers(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("无 bootFS 应 400, got %d", w.Result().StatusCode)
	}
}

func TestInjectablePackFilesArchFilter(t *testing.T) {
	root := setupDriverBootRoot(t)
	h := &OSImageHandler{bootFS: boot.NewBootFileServer(root)}

	files, err := h.injectablePackFiles("nic-intel", "amd64")
	if err != nil {
		t.Fatalf("injectablePackFiles: %v", err)
	}
	// amd64 → 保留 x64/ 子目录与顶层通用文件
	want := []string{"e1d.inf", "x64/e1d64.sys"}
	if strings.Join(files, ",") != strings.Join(want, ",") {
		t.Fatalf("files = %v, want %v", files, want)
	}

	// arm64 → x64/ 子目录被过滤，顶层通用 .inf 保留（有意设计：通用驱动跨架构）
	got, err := h.injectablePackFiles("nic-intel", "arm64")
	if err != nil {
		t.Fatalf("arm64: %v", err)
	}
	if strings.Join(got, ",") != "e1d.inf" {
		t.Fatalf("arm64 files = %v, want [e1d.inf]", got)
	}
}

func TestWimInfoValidation(t *testing.T) {
	st := store.NewMemory()
	h := &OSImageHandler{store: st}

	// 未提取 → 400
	img1 := &models.OSImage{Name: "nx", Filename: "nx.iso", Status: "ready"}
	if err := st.CreateOSImage(t.Context(), img1); err != nil {
		t.Fatal(err)
	}
	id1 := strconv.FormatUint(uint64(img1.ID), 10)
	req := withChiParams(httptest.NewRequest(http.MethodGet, "/api/v1/os-images/"+id1+"/wim-info", nil), "id", id1)
	w := httptest.NewRecorder()
	h.WIMInfo(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("未提取应 400, got %d", w.Result().StatusCode)
	}

	// 已提取但 sources/boot.wim 缺失 → 400
	img2 := &models.OSImage{Name: "nw", Filename: "nw.iso", Status: "ready", ExtractedTo: t.TempDir()}
	if err := st.CreateOSImage(t.Context(), img2); err != nil {
		t.Fatal(err)
	}
	id2 := strconv.FormatUint(uint64(img2.ID), 10)
	req = withChiParams(httptest.NewRequest(http.MethodGet, "/api/v1/os-images/"+id2+"/wim-info", nil), "id", id2)
	w = httptest.NewRecorder()
	h.WIMInfo(w, req)
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("wim 缺失应 400, got %d", w.Result().StatusCode)
	}
}

// TestWIMCapabilitiesEndpoint 能力探测：任何平台都返回 200，platform 反映 GOOS，
// offline_driver_injection 与平台一致（非 Windows 需带原因）。
func TestWIMCapabilitiesEndpoint(t *testing.T) {
	h := &OSImageHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/os-images/capabilities", nil)
	w := httptest.NewRecorder()
	h.Capabilities(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Result().StatusCode)
	}
	var resp struct {
		Data osimage.Capabilities `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Platform != runtime.GOOS {
		t.Fatalf("platform = %q, want %q", resp.Data.Platform, runtime.GOOS)
	}
	if want := runtime.GOOS == "windows"; resp.Data.OfflineDriverInjection != want {
		t.Fatalf("offline_driver_injection = %v, want %v", resp.Data.OfflineDriverInjection, want)
	}
}

// TestCapabilitiesRouteNotShadowedByID 路由级：/os-images/capabilities 必须命中
// capabilities（静态段优先），不能被 /os-images/{id} 捕获（否则 ParseUint 失败 → 400）。
func TestCapabilitiesRouteNotShadowedByID(t *testing.T) {
	st := store.NewMemory()
	h := &OSImageHandler{store: st}
	r := chi.NewRouter()
	// 与 handler.go 注册顺序/形态一致（{id} 在前，静态 capabilities 在后）
	r.Get("/os-images/{id}", h.Get)
	r.Get("/os-images/capabilities", h.Capabilities)

	req := httptest.NewRequest(http.MethodGet, "/os-images/capabilities", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("capabilities 应命中并返回 200, got %d (%s)", w.Result().StatusCode, w.Body.String())
	}
	var resp struct {
		Data osimage.Capabilities `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Platform == "" {
		t.Fatalf("应返回 platform, got %s", w.Body.String())
	}
}

// TestWIMInjectLock 并发互斥：同镜像同时只允许一个注入/还原；不同镜像互不阻塞。
// 注意：锁表是进程级全局，测试必须释放自己拿到的锁，否则会污染后续用例。
func TestWIMInjectLock(t *testing.T) {
	const a, b = 9001, 9002 // 测试专用 ID，避开内存库的连续自增 ID
	mu := lockWIMInject(a)
	if mu == nil {
		t.Fatal("首个获取应成功")
	}
	if lockWIMInject(a) != nil {
		t.Fatal("同镜像二次获取应失败（→ 409）")
	}
	mu2 := lockWIMInject(b)
	if mu2 == nil {
		t.Fatal("不同镜像应可获取")
	}
	mu2.Unlock()
	mu.Unlock()
	m := lockWIMInject(a)
	if m == nil {
		t.Fatal("释放后应可再次获取")
	}
	m.Unlock()
}

// TestRestoreWIMValidation：未提取 / 无备份 / 非法 target 应 400。
func TestRestoreWIMValidation(t *testing.T) {
	st := store.NewMemory()
	h := &OSImageHandler{store: st}

	img1 := &models.OSImage{Name: "nx", Filename: "nx.iso", Status: "ready"}
	if err := st.CreateOSImage(t.Context(), img1); err != nil {
		t.Fatal(err)
	}
	id1 := strconv.FormatUint(uint64(img1.ID), 10)
	w := httptest.NewRecorder()
	h.RestoreWIM(w, withChiParams(httptest.NewRequest(http.MethodPost, "/api/v1/os-images/"+id1+"/restore-wim", nil), "id", id1))
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("未提取应 400, got %d", w.Result().StatusCode)
	}

	// 已提取但无 .orig 备份
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sources"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sources", "boot.wim"), []byte("w"), 0644); err != nil {
		t.Fatal(err)
	}
	img2 := &models.OSImage{Name: "nw", Filename: "nw.iso", Status: "ready", ExtractedTo: dir}
	if err := st.CreateOSImage(t.Context(), img2); err != nil {
		t.Fatal(err)
	}
	id2 := strconv.FormatUint(uint64(img2.ID), 10)
	w = httptest.NewRecorder()
	h.RestoreWIM(w, withChiParams(httptest.NewRequest(http.MethodPost, "/api/v1/os-images/"+id2+"/restore-wim", nil), "id", id2))
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("无备份应 400, got %d", w.Result().StatusCode)
	}

	// 非法 target
	if err := os.WriteFile(filepath.Join(dir, "sources", "boot.wim.orig"), []byte("o"), 0644); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.RestoreWIM(w, withChiParams(httptest.NewRequest(http.MethodPost, "/api/v1/os-images/"+id2+"/restore-wim",
		strings.NewReader(`{"target":"recovery"}`)), "id", id2))
	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("非法 target 应 400, got %d", w.Result().StatusCode)
	}
}

// TestRestoreWIMRestoresAndClears：还原把 .orig 覆盖回 WIM，并清空注入记录。
func TestRestoreWIMRestoresAndClears(t *testing.T) {
	dir := t.TempDir()
	sources := filepath.Join(dir, "sources")
	if err := os.MkdirAll(sources, 0755); err != nil {
		t.Fatal(err)
	}
	original := []byte("pristine-wim")
	wimPath := filepath.Join(sources, "boot.wim")
	if err := os.WriteFile(wimPath, []byte("injected-wim"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wimPath+".orig", original, 0644); err != nil {
		t.Fatal(err)
	}

	st := store.NewMemory()
	img := &models.OSImage{
		Name: "w", Filename: "w.iso", Status: "ready", ExtractedTo: dir,
		InjectedDrivers: `["nic-intel"]`, DriverInjectCount: 3,
	}
	if err := st.CreateOSImage(t.Context(), img); err != nil {
		t.Fatal(err)
	}
	h := &OSImageHandler{store: st}
	id := strconv.FormatUint(uint64(img.ID), 10)
	w := httptest.NewRecorder()
	h.RestoreWIM(w, withChiParams(httptest.NewRequest(http.MethodPost, "/api/v1/os-images/"+id+"/restore-wim", nil), "id", id))
	if w.Result().StatusCode != http.StatusOK {
		t.Fatalf("还原应成功, got %d (%s)", w.Result().StatusCode, w.Body.String())
	}
	got, _ := os.ReadFile(wimPath)
	if string(got) != string(original) {
		t.Fatalf("WIM 应被还原为原始内容, got %q", got)
	}
	// .orig 保留（可重复还原）
	if _, err := os.Stat(wimPath + ".orig"); err != nil {
		t.Fatalf("备份应保留: %v", err)
	}
	// 注入记录清空
	reloaded, err := st.GetOSImage(t.Context(), img.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.DriverInjectCount != 0 || reloaded.InjectedDrivers != "" || reloaded.DriverInjectedAt != nil {
		t.Fatalf("注入记录应清空: %+v", reloaded)
	}
}

func TestNormalizeWimTarget(t *testing.T) {
	cases := map[string]string{
		"":          "boot",
		"boot":      "boot",
		"BOOT":      "boot",
		"install":   "install",
		" Install ": "install",
		"recovery":  "boot", // 未知值回落 boot（WIMInfo 查询参数；注入路径已先严格校验）
	}
	for in, want := range cases {
		if got := normalizeWimTarget(in); got != want {
			t.Errorf("normalizeWimTarget(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRespondWIMError(t *testing.T) {
	w := httptest.NewRecorder()
	respondWIMError(w, osimage.ErrWIMUnsupported)
	if w.Result().StatusCode != http.StatusNotImplemented {
		t.Fatalf("ErrWIMUnsupported 应 501, got %d", w.Result().StatusCode)
	}
	if !strings.Contains(w.Body.String(), "PE 阶段") {
		t.Fatalf("501 信息应提示替代方案: %s", w.Body.String())
	}
	// 包装后的错误也应命中 501（osimage 错误经 fmt 包装后 errors.Is 可达）
	w2 := httptest.NewRecorder()
	respondWIMError(w2, fmt.Errorf("mount wim: %w", osimage.ErrWIMUnsupported))
	if w2.Result().StatusCode != http.StatusNotImplemented {
		t.Fatalf("wrapped ErrWIMUnsupported 应 501, got %d", w2.Result().StatusCode)
	}
	// 其他错误 → 500
	w3 := httptest.NewRecorder()
	respondWIMError(w3, errors.New("boom"))
	if w3.Result().StatusCode != http.StatusInternalServerError {
		t.Fatalf("普通错误应 500, got %d", w3.Result().StatusCode)
	}
}

func TestCopyFileTo(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	os.WriteFile(src, []byte("payload"), 0644)
	dst := filepath.Join(dir, "nested", "dst.bin")
	if err := copyFileTo(src, dst); err != nil {
		t.Fatalf("copyFileTo: %v", err)
	}
	b, err := os.ReadFile(dst)
	if err != nil || string(b) != "payload" {
		t.Fatalf("dst = %q, err = %v", b, err)
	}
}

func TestOSImageInjectedDriversJSONRoundTrip(t *testing.T) {
	var img models.OSImage
	if err := json.Unmarshal([]byte(`{"name":"w","injected_drivers":["nic-intel","raid-lsi"]}`), &img); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	packs, err := img.GetInjectedDrivers()
	if err != nil || len(packs) != 2 || packs[0] != "nic-intel" {
		t.Fatalf("packs = %v, err = %v", packs, err)
	}
	out, err := json.Marshal(&img)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"injected_drivers":["nic-intel","raid-lsi"]`) {
		t.Fatalf("marshal 后 injected_drivers 错误: %s", out)
	}
	// 未设置时序列化为空数组而非 null
	var empty models.OSImage
	out2, _ := json.Marshal(&empty)
	if !strings.Contains(string(out2), `"injected_drivers":[]`) {
		t.Fatalf("空镜像 injected_drivers 应为 []: %s", out2)
	}
}

func TestMustJSONStringArray(t *testing.T) {
	if got := mustJSONStringArray([]string{"a", "b"}); got != `["a","b"]` {
		t.Fatalf("got %s", got)
	}
	if got := mustJSONStringArray(nil); got != "null" && got != "[]" {
		t.Fatalf("nil → %s（允许 null/[]，调用方仅在有注入成功时写入）", got)
	}
}
