package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
)

func newOSImageHandlerWithCatalog(t *testing.T) (*OSImageHandler, store.Interface, config.GlobalConfig) {
	t.Helper()
	tmp := t.TempDir()
	catDir := netboot.CatalogDir(tmp)
	if err := os.MkdirAll(catDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := netboot.ExtractSeed(catDir); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	if _, err := os.Stat(filepath.Join(catDir, "windows.yaml")); err != nil {
		t.Fatalf("windows.yaml not seeded: %v", err)
	}
	g := config.GlobalConfig{DataDir: tmp}
	cfg := &config.Config{Global: g}
	mgr := netboot.NewManager(netboot.DefaultCatalog())
	st := store.NewMemory()
	return NewOSImageHandler(st, cfg, nil, mgr), st, g
}

func seedWindowsImage(t *testing.T, st store.Interface) {
	t.Helper()
	if err := st.CreateOSImage(t.Context(), &models.OSImage{
		ID:          1,
		Name:        "win11-iso",
		Filename:    "win11.iso",
		Distro:      "windows",
		Status:      "ready",
		ExtractedTo: "/var/pxelab/boot/isos/abc-win11-iso",
	}); err != nil {
		t.Fatalf("seed os image: %v", err)
	}
}

func TestSetCatalogLocal(t *testing.T) {
	h, st, _ := newOSImageHandlerWithCatalog(t)
	seedWindowsImage(t, st)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/os-images/1/set-catalog-local", nil)
	req = withChiParams(req, "id", "1")
	req.Host = "10.0.0.10:8080"
	w := httptest.NewRecorder()
	h.SetCatalogLocal(w, req)
	if w.Result().StatusCode != http.StatusOK {
		resp := w.Result()
		var out Response
		_ = jsonDecode(resp, &out)
		t.Fatalf("expected 200, got %d (%v)", resp.StatusCode, out.Error)
	}

	win := findWindowsDistro(h.netbootMgr.Catalog())
	if win == nil {
		t.Fatal("catalog missing Windows PE distro after reload")
	}
	ok := false
	for _, v := range win.Versions {
		if v.Enabled && v.Local != nil {
			if v.Local.Kernel != "http://10.0.0.10:8080/netboot/menu/wimboot" {
				t.Errorf("kernel = %s", v.Local.Kernel)
			}
			if v.Local.Initrd != "http://10.0.0.10:8080/boot/isos/abc-win11-iso" {
				t.Errorf("initrd = %s", v.Local.Initrd)
			}
			ok = true
		}
	}
	if !ok {
		t.Fatal("no enabled version got a local ref")
	}
}

func TestSetCatalogLocalRejectsNotWindows(t *testing.T) {
	h, st, _ := newOSImageHandlerWithCatalog(t)
	if err := st.CreateOSImage(t.Context(), &models.OSImage{
		ID:          1,
		Name:        "ubuntu-iso",
		Filename:    "ubuntu.iso",
		Distro:      "ubuntu",
		Status:      "ready",
		ExtractedTo: "/var/pxelab/boot/isos/abc-ubuntu",
	}); err != nil {
		t.Fatalf("seed os image: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/os-images/1/set-catalog-local", nil)
	req = withChiParams(req, "id", "1")
	req.Host = "10.0.0.10:8080"
	w := httptest.NewRecorder()
	h.SetCatalogLocal(w, req)
	if code := w.Result().StatusCode; code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-windows image, got %d", code)
	}
}

func TestSetCatalogLocalRejectsUnextracted(t *testing.T) {
	h, st, _ := newOSImageHandlerWithCatalog(t)
	if err := st.CreateOSImage(t.Context(), &models.OSImage{
		ID:       7,
		Name:     "win11-iso",
		Filename: "win11.iso",
		Distro:   "windows",
		Status:   "ready",
	}); err != nil {
		t.Fatalf("seed os image: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/os-images/1/set-catalog-local", nil)
	req = withChiParams(req, "id", "1")
	req.Host = "10.0.0.10:8080"
	w := httptest.NewRecorder()
	h.SetCatalogLocal(w, req)
	if code := w.Result().StatusCode; code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unextracted image, got %d", code)
	}
}

func findWindowsDistro(c *netboot.Catalog) *netboot.Distro {
	for _, d := range c.Distros {
		if strings.Contains(strings.ToLower(d.Name), "windows") {
			return d
		}
	}
	return nil
}

func jsonDecode(resp *http.Response, out any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(out)
}
