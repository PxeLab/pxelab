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
