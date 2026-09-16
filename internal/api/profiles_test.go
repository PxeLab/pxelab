package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/models"
	"github.com/pxelab/pxelab/internal/netboot"
	"github.com/pxelab/pxelab/internal/store"
)

func newProfileHandler(t *testing.T) (*ProfileHandler, store.Interface) {
	t.Helper()
	st := store.NewMemory()
	mgr := netboot.NewManager(&netboot.Catalog{})
	return &ProfileHandler{store: st, netbootMgr: mgr}, st
}

func mustOSImage(t *testing.T) *models.OSImage {
	t.Helper()
	return &models.OSImage{
		ID:          1,
		Name:        "win11-iso",
		Filename:    "win11.iso",
		Distro:      "windows",
		Status:      "ready",
		ExtractedTo: "/boot/isos/123-win11-iso",
	}
}

func TestCreateFromOSImage(t *testing.T) {
	h, st := newProfileHandler(t)

	if err := st.CreateOSImage(t.Context(), mustOSImage(t)); err != nil {
		t.Fatalf("seed os image: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/from-os-image",
		strings.NewReader(`{"os_image_id":1,"profile_name":"winpe-local","description":"local win"}`))
	req.Host = "10.0.0.10:8080"
	w := httptest.NewRecorder()
	h.CreateFromOSImage(w, req)

	if w.Result().StatusCode != http.StatusCreated {
		resp := w.Result()
		defer resp.Body.Close()
		var out Response
		_ = json.NewDecoder(resp.Body).Decode(&out)
		t.Fatalf("expected 201, got %d (%v)", resp.StatusCode, out.Error)
	}

	profiles, err := st.ListProfiles(t.Context())
	if err != nil || len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d (err=%v)", len(profiles), err)
	}
	p := profiles[0]
	if p.Name != "winpe-local" {
		t.Errorf("profile name = %q", p.Name)
	}
	menu, err := p.GetMenu()
	if err != nil {
		t.Fatalf("get menu: %v", err)
	}
	if len(menu.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(menu.Entries))
	}
	e := menu.Entries[0]
	if e.Type != "wds" {
		t.Errorf("entry type = %q, want wds", e.Type)
	}
	if e.URL == nil || *e.URL != "http://10.0.0.10:8080/netboot/menu/wimboot" {
		t.Errorf("entry url = %v", e.URL)
	}
	if e.WIM == nil || *e.WIM != "http://10.0.0.10:8080/boot/isos/123-win11-iso" {
		t.Errorf("entry wim = %v", e.WIM)
	}
}

func TestCreateFromOSImageRejectsNonWindows(t *testing.T) {
	h, st := newProfileHandler(t)
	img := mustOSImage(t)
	img.Distro = "ubuntu"
	if err := st.CreateOSImage(t.Context(), img); err != nil {
		t.Fatalf("seed os image: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/from-os-image",
		strings.NewReader(`{"os_image_id":1,"profile_name":"winpe-local"}`))
	w := httptest.NewRecorder()
	h.CreateFromOSImage(w, req)
	if code := w.Result().StatusCode; code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-windows image, got %d", code)
	}
}

func TestCreateFromOSImageRejectsNotExtracted(t *testing.T) {
	h, st := newProfileHandler(t)
	img := mustOSImage(t)
	img.ExtractedTo = ""
	if err := st.CreateOSImage(t.Context(), img); err != nil {
		t.Fatalf("seed os image: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/profiles/from-os-image",
		strings.NewReader(`{"os_image_id":1,"profile_name":"winpe-local"}`))
	w := httptest.NewRecorder()
	h.CreateFromOSImage(w, req)
	if code := w.Result().StatusCode; code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unextracted image, got %d", code)
	}
}
