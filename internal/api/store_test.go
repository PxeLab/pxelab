package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
)

// netbootDistroDetailJSON is a full-schema netboot_distro store item: one
// kernel version and one wimboot version, plus distro-level mirror fields.
const netbootDistroDetailJSON = `{
  "id": "netboot-debtest",
  "type": "netboot_distro",
  "name": "DebTest",
  "description": "test distro",
  "content": {
    "name": "DebTest",
    "enabled": true,
    "menu_group": "linux",
    "mirror": "http://mirror.example.com/debian",
    "kernel_params": "quiet",
    "versions": [
      {
        "codename": "trixie",
        "name": "13.0 (trixie)",
        "arch": "amd64",
        "enabled": true,
        "remote": {"kernel": "http://deb.example.com/linux", "initrd": "http://deb.example.com/initrd.gz"},
        "cmdline": "auto=true",
        "install_type": "legacy"
      },
      {
        "codename": "winpe",
        "name": "WinPE",
        "arch": "amd64",
        "enabled": true,
        "type": "wimboot",
        "remote": {"kernel": "http://deb.example.com/wimboot", "initrd": "http://deb.example.com/winpe"}
      },
      {
        "codename": "disabled-one",
        "name": "Disabled",
        "arch": "amd64",
        "enabled": false,
        "remote": {"kernel": "http://deb.example.com/x", "initrd": "http://deb.example.com/y"}
      }
    ]
  }
}`

// legacyNetbootDistroDetailJSON uses the older content shape (distro_name,
// versions without type/cmdline) as served by hub.pxelab.com before the
// schema was completed.
const legacyNetbootDistroDetailJSON = `{
  "id": "netboot-legacy",
  "type": "netboot_distro",
  "name": "LegacyOS",
  "description": "legacy item",
  "content": {
    "distro_name": "LegacyOS",
    "menu_group": "linux",
    "versions": [
      {
        "codename": "one",
        "name": "1.0",
        "arch": "amd64",
        "enabled": true,
        "remote": {"kernel": "http://legacy.example.com/linux", "initrd": "http://legacy.example.com/initrd.gz"},
        "install_type": "legacy"
      }
    ]
  }
}`

func newNetbootStoreHandler(t *testing.T) (*StoreHandler, store.Interface, *netboot.Manager, string) {
	t.Helper()
	st := store.NewMemory()
	catalogDir := t.TempDir()
	mgr := netboot.NewManager(&netboot.Catalog{})
	return NewStoreHandler(st, catalogDir, mgr), st, mgr, catalogDir
}

func decodeCreated(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var out Response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := out.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data shape: %T", out.Data)
	}
	return data
}

func TestImportNetbootDistroWritesCatalogAndProfile(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/items/") {
			w.Write([]byte(netbootDistroDetailJSON))
			return
		}
		w.WriteHeader(http.StatusNotFound) // dlincrement etc.
	}))
	defer hub.Close()

	h, st, mgr, catalogDir := newNetbootStoreHandler(t)
	h.WithEndpoint(hub.URL)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/store/import",
		strings.NewReader(`{"item_id":"netboot-debtest","item_type":"netboot_distro"}`))
	w := httptest.NewRecorder()
	h.ImportItem(w, req)

	data := decodeCreated(t, w)
	if data["distro_name"] != "DebTest" {
		t.Errorf("expected distro_name=DebTest, got %v", data["distro_name"])
	}

	// Catalog YAML written to disk with full fidelity.
	yamlPath := filepath.Join(catalogDir, "debtest.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Fatalf("expected catalog YAML at %s: %v", yamlPath, err)
	}
	d, err := netboot.LoadDistro(yamlPath)
	if err != nil {
		t.Fatalf("load written distro: %v", err)
	}
	if d.Mirror != "http://mirror.example.com/debian" || d.KernelParams != "quiet" {
		t.Errorf("distro-level fields lost: mirror=%q kernel_params=%q", d.Mirror, d.KernelParams)
	}
	if len(d.Versions) != 3 || d.Versions[1].BootType != netboot.BootWimboot {
		t.Fatalf("versions not preserved: %+v", d.Versions)
	}

	// Manager reloaded — distro visible to the catalog APIs.
	if mgr.GetDistro("DebTest") == nil {
		t.Error("manager not reloaded: GetDistro(DebTest) returned nil")
	}

	// Profile created with correct per-boot-type entries.
	// (memory store assigns its own IDs, so look the profile up by listing)
	profiles, err := st.ListProfiles(req.Context())
	if err != nil || len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d (err=%v)", len(profiles), err)
	}
	menu, err := profiles[0].GetMenu()
	if err != nil {
		t.Fatalf("profile menu: %v", err)
	}
	if len(menu.Entries) != 2 {
		t.Fatalf("expected 2 entries (disabled version skipped), got %d", len(menu.Entries))
	}
	if menu.Entries[0].Type != "direct" || menu.Entries[0].Kernel == nil || *menu.Entries[0].Kernel != "http://deb.example.com/linux" {
		t.Errorf("kernel entry wrong: %+v", menu.Entries[0])
	}
	if menu.Entries[0].Cmdline == nil || *menu.Entries[0].Cmdline != "auto=true" {
		t.Errorf("cmdline lost: %+v", menu.Entries[0])
	}
	if menu.Entries[1].Type != "wds" || menu.Entries[1].URL == nil || *menu.Entries[1].URL != "http://deb.example.com/wimboot" {
		t.Errorf("wimboot entry wrong: %+v", menu.Entries[1])
	}
}

func TestImportLocalNetbootDistroLegacyFormat(t *testing.T) {
	h, _, mgr, catalogDir := newNetbootStoreHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/store/import-local",
		strings.NewReader(legacyNetbootDistroDetailJSON))
	w := httptest.NewRecorder()
	h.ImportLocalItem(w, req)

	data := decodeCreated(t, w)
	if data["distro_name"] != "LegacyOS" {
		t.Errorf("expected distro_name=LegacyOS, got %v", data["distro_name"])
	}
	if _, err := os.Stat(filepath.Join(catalogDir, "legacyos.yaml")); err != nil {
		t.Errorf("legacy import did not write catalog YAML: %v", err)
	}
	if mgr.GetDistro("LegacyOS") == nil {
		t.Error("manager not reloaded for legacy import")
	}
}

func TestImportNetbootDistroIdempotentOverwrite(t *testing.T) {
	h, st, mgr, catalogDir := newNetbootStoreHandler(t)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/store/import-local",
			strings.NewReader(netbootDistroDetailJSON))
		w := httptest.NewRecorder()
		h.ImportLocalItem(w, req)
		if w.Result().StatusCode != http.StatusCreated {
			t.Fatalf("import %d failed: %d", i, w.Result().StatusCode)
		}
	}

	// Same-name YAML overwritten, not duplicated; still exactly one distro.
	cat, err := netboot.LoadCatalog(catalogDir)
	if err != nil || len(cat.Distros) != 1 {
		t.Fatalf("expected exactly 1 distro on disk, got %v (err=%v)", cat.Distros, err)
	}
	if mgr.GetDistro("DebTest") == nil {
		t.Error("manager lost distro after re-import")
	}

	// Second profile gets a suffixed ID and a deduped name; both exist.
	profiles, err := st.ListProfiles(t.Context())
	if err != nil || len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d (err=%v)", len(profiles), err)
	}
	if profiles[0].Name == profiles[1].Name {
		t.Errorf("profile names not deduped: both %q", profiles[0].Name)
	}
}

func TestImportNetbootDistroOverwritesSameNameSeedFile(t *testing.T) {
	h, _, mgr, catalogDir := newNetbootStoreHandler(t)

	// Seed a catalog file whose filename differs from the derived one — mimics
	// the embedded seed (tinycore.yaml defining "Tiny Core Linux").
	seed := &netboot.Distro{
		Name:      "DebTest",
		Enabled:   true,
		MenuGroup: "linux",
		Versions: []*netboot.Version{
			{Codename: "old", Name: "old", Arch: "amd64", Enabled: true,
				Remote: &netboot.FileRef{Kernel: "http://old.example.com/linux"}},
		},
	}
	if err := netboot.SaveDistro(filepath.Join(catalogDir, "debtest-seed.yaml"), seed); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/store/import-local",
		strings.NewReader(netbootDistroDetailJSON))
	w := httptest.NewRecorder()
	h.ImportLocalItem(w, req)
	decodeCreated(t, w)

	// The seed file is overwritten in place; no second file is created.
	if _, err := os.Stat(filepath.Join(catalogDir, "debtest.yaml")); !os.IsNotExist(err) {
		t.Error("import created a duplicate file instead of overwriting the seed file")
	}
	d, err := netboot.LoadDistro(filepath.Join(catalogDir, "debtest-seed.yaml"))
	if err != nil {
		t.Fatalf("seed file missing after import: %v", err)
	}
	if len(d.Versions) != 3 {
		t.Errorf("expected overwritten content (3 versions), got %d", len(d.Versions))
	}
	cat, err := netboot.LoadCatalog(catalogDir)
	if err != nil || len(cat.Distros) != 1 {
		t.Fatalf("expected exactly 1 distro in catalog, got %d (err=%v)", len(cat.Distros), err)
	}
	if mgr.GetDistro("DebTest") == nil {
		t.Error("manager not reloaded")
	}
}

func TestImportNetbootDistroRejectsEmpty(t *testing.T) {
	h, _, _, _ := newNetbootStoreHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/store/import-local",
		strings.NewReader(`{"id":"netboot-empty","type":"netboot_distro","name":"Empty","content":{"distro_name":"Empty","versions":[]}}`))
	w := httptest.NewRecorder()
	h.ImportLocalItem(w, req)

	if w.Result().StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Result().StatusCode)
	}
}

// twoStageDistroDetailJSON mimics a 信创 OS content pack (Kylin/UOS style):
// requires_local_image marker, image hint, verification note, local-only
// versions, and an embedded kickstart answer template.
const twoStageDistroDetailJSON = `{
  "id": "netboot-kylin-v10",
  "type": "netboot_distro",
  "name": "Kylin V10 Server",
  "description": "two-stage item",
  "content": {
    "name": "Kylin V10 Server",
    "enabled": true,
    "menu_group": "linux",
    "requires_local_image": true,
    "image_hint": "Upload vmlinuz/initrd.img extracted from the Kylin V10 SP3 ISO.",
    "verification": "社区贡献，未实测",
    "local_base": "netboot/kylin",
    "versions": [
      {
        "codename": "v10sp3-x86_64",
        "name": "V10 SP3 (x86_64)",
        "arch": "amd64",
        "enabled": true,
        "local": {"kernel": "kylin/v10sp3-x86_64/vmlinuz", "initrd": "kylin/v10sp3-x86_64/initrd.img"},
        "cmdline": "inst.text ip=dhcp",
        "answer_param": "inst.ks={{.AnswerURL}}",
        "install_type": "anaconda"
      }
    ],
    "answer_template": {
      "name": "Kylin V10 Kickstart",
      "description": "适用镜像: Kylin V10 SP3",
      "type": "kickstart",
      "content": "#version=DEVEL\nlang zh_CN.UTF-8\n"
    }
  }
}`

func TestImportNetbootDistroTwoStage(t *testing.T) {
	h, st, mgr, catalogDir := newNetbootStoreHandler(t)

	doImport := func() map[string]any {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/store/import-local",
			strings.NewReader(twoStageDistroDetailJSON))
		w := httptest.NewRecorder()
		h.ImportLocalItem(w, req)
		return decodeCreated(t, w)
	}

	data := doImport()
	if data["requires_local_image"] != true {
		t.Errorf("expected requires_local_image=true in response, got %v", data["requires_local_image"])
	}
	if data["image_hint"] == "" || data["verification"] != "社区贡献，未实测" {
		t.Errorf("image_hint/verification not passed through: %+v", data)
	}
	if data["answer_template_name"] != "Kylin V10 Kickstart" {
		t.Errorf("expected answer_template_name, got %+v", data)
	}

	// Markers persisted into the local catalog so the UI can badge the entry.
	d, err := netboot.LoadDistro(filepath.Join(catalogDir, "kylin-v10-server.yaml"))
	if err != nil {
		t.Fatalf("catalog YAML not written: %v", err)
	}
	if !d.RequiresLocalImage || d.ImageHint == "" || d.Verification == "" {
		t.Errorf("requires_local_image markers lost in catalog YAML: %+v", d)
	}
	if d.Versions[0].AnswerParam != "inst.ks={{.AnswerURL}}" {
		t.Errorf("answer_param lost: %q", d.Versions[0].AnswerParam)
	}

	// Answer template created once; second import reuses it.
	tpls, err := st.ListAnswerTemplates(t.Context())
	if err != nil || len(tpls) != 1 {
		t.Fatalf("expected 1 answer template, got %d (err=%v)", len(tpls), err)
	}
	if tpls[0].Type != "kickstart" || !strings.Contains(tpls[0].Content, "#version=DEVEL") {
		t.Errorf("answer template content wrong: %+v", tpls[0])
	}

	data = doImport()
	tpls, _ = st.ListAnswerTemplates(t.Context())
	if len(tpls) != 1 {
		t.Errorf("second import duplicated the answer template: %d", len(tpls))
	}
	if mgr.GetDistro("Kylin V10 Server") == nil {
		t.Error("manager not reloaded after re-import")
	}
}
