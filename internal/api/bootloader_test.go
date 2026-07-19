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
)

func tempBootFS(t *testing.T) *boot.BootFileServer {
	t.Helper()
	dir, err := os.MkdirTemp("", "bootloader-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return boot.NewBootFileServer(dir)
}

func touchFile(t *testing.T, bfs *boot.BootFileServer, name string) {
	t.Helper()
	path := filepath.Join(bfs.Root(), name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte("test content for "+name), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func readBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return body
}

func TestBootloaderCheck_AllMissing(t *testing.T) {
	bfs := tempBootFS(t)
	h := NewBootloaderHandler(bfs)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/bootloader/check", nil)
	h.Check(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := readBody(t, w)
	data, _ := json.Marshal(body["data"])
	var result BootloaderCheckResult
	json.Unmarshal(data, &result)

	if result.AllOK {
		t.Error("expected AllOK=false when all files missing")
	}
	if result.Total != len(knownBootFiles) {
		t.Errorf("Total = %d, want %d", result.Total, len(knownBootFiles))
	}
	if result.Present != 0 {
		t.Errorf("Present = %d, want 0", result.Present)
	}
	if result.Missing != len(knownBootFiles) {
		t.Errorf("Missing = %d, want %d", result.Missing, len(knownBootFiles))
	}
}

func TestBootloaderCheck_SomePresent(t *testing.T) {
	bfs := tempBootFS(t)
	touchFile(t, bfs, "ipxe.efi")
	touchFile(t, bfs, "ipxe.pxe")
	h := NewBootloaderHandler(bfs)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/bootloader/check", nil)
	h.Check(w, r)

	body := readBody(t, w)
	data, _ := json.Marshal(body["data"])
	var result BootloaderCheckResult
	json.Unmarshal(data, &result)

	if result.AllOK {
		t.Error("expected AllOK=false (some missing)")
	}
	if result.Present != 2 {
		t.Errorf("Present = %d, want 2", result.Present)
	}
	if result.Missing != len(knownBootFiles)-2 {
		t.Errorf("Missing = %d, want %d", result.Missing, len(knownBootFiles)-2)
	}

	// Check that present files have checksums and sizes
	var ipxeEFI, ipxePXE bool
	for _, f := range result.Files {
		if f.Name == "ipxe.efi" && f.Present {
			ipxeEFI = true
			if f.Size == 0 {
				t.Error("ipxe.efi size should not be 0")
			}
			if f.Checksum == "" {
				t.Error("ipxe.efi checksum should not be empty")
			}
		}
		if f.Name == "ipxe.pxe" && f.Present {
			ipxePXE = true
		}
	}
	if !ipxeEFI {
		t.Error("ipxe.efi should be marked present")
	}
	if !ipxePXE {
		t.Error("ipxe.pxe should be marked present")
	}
}

func TestBootloaderCheck_AllPresent(t *testing.T) {
	bfs := tempBootFS(t)
	for _, kf := range knownBootFiles {
		touchFile(t, bfs, kf.Name)
	}
	h := NewBootloaderHandler(bfs)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/bootloader/check", nil)
	h.Check(w, r)

	body := readBody(t, w)
	data, _ := json.Marshal(body["data"])
	var result BootloaderCheckResult
	json.Unmarshal(data, &result)

	if !result.AllOK {
		t.Error("expected AllOK=true when all files present")
	}
	if result.Present != len(knownBootFiles) {
		t.Errorf("Present = %d, want %d", result.Present, len(knownBootFiles))
	}
	if result.Missing != 0 {
		t.Errorf("Missing = %d, want 0", result.Missing)
	}
}

func TestBootloaderCheck_SortOrder(t *testing.T) {
	bfs := tempBootFS(t)
	touchFile(t, bfs, "ipxe.efi")
	h := NewBootloaderHandler(bfs)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/bootloader/check", nil)
	h.Check(w, r)

	body := readBody(t, w)
	data, _ := json.Marshal(body["data"])
	var result BootloaderCheckResult
	json.Unmarshal(data, &result)

	// Present files should come first
	foundMissing := false
	for _, f := range result.Files {
		if f.Present && foundMissing {
			t.Error("present files should come before missing files")
		}
		if !f.Present {
			foundMissing = true
		}
	}
}

func TestBootloaderFiles_Empty(t *testing.T) {
	bfs := tempBootFS(t)
	h := NewBootloaderHandler(bfs)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/bootloader/files", nil)
	h.List(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body := readBody(t, w)
	data, _ := json.Marshal(body["data"])
	var entries []map[string]any
	json.Unmarshal(data, &entries)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestBootloaderFiles_WithFiles(t *testing.T) {
	bfs := tempBootFS(t)
	touchFile(t, bfs, "ipxe.efi")
	touchFile(t, bfs, "ipxe.pxe")
	h := NewBootloaderHandler(bfs)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api/v1/bootloader/files", nil)
	h.List(w, r)

	body := readBody(t, w)
	data, _ := json.Marshal(body["data"])
	var entries []map[string]any
	json.Unmarshal(data, &entries)

	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Check alphabetical sort
	if entries[0]["name"] != "ipxe.efi" || entries[1]["name"] != "ipxe.pxe" {
		t.Errorf("expected sorted order: ipxe.efi, ipxe.pxe; got %v, %v",
			entries[0]["name"], entries[1]["name"])
	}
}

func TestCheckFile_Existing(t *testing.T) {
	bfs := tempBootFS(t)
	touchFile(t, bfs, "ipxe.efi")
	h := NewBootloaderHandler(bfs)

	body := strings.NewReader(`{"name":"ipxe.efi"}`)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/bootloader/check-file", body)
	r.Header.Set("Content-Type", "application/json")
	h.CheckFile(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Data BootFileInfo `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if !result.Data.Present {
		t.Error("expected Present=true")
	}
	if result.Data.Size == 0 {
		t.Error("expected non-zero size")
	}
	if result.Data.Checksum == "" {
		t.Error("expected checksum")
	}
}

func TestCheckFile_NotFound(t *testing.T) {
	bfs := tempBootFS(t)
	h := NewBootloaderHandler(bfs)

	body := strings.NewReader(`{"name":"nonexistent.efi"}`)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/bootloader/check-file", body)
	r.Header.Set("Content-Type", "application/json")
	h.CheckFile(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCheckFile_PathTraversal_Rejected(t *testing.T) {
	bfs := tempBootFS(t)
	h := NewBootloaderHandler(bfs)

	traversalAttempts := []string{
		`{"name":"../etc/passwd"}`,
		`{"name":"..\\windows\\system32"}`,
		`{"name":"/etc/passwd"}`,
		`{"name":"\\windows\\system32"}`,
		`{"name":".."}`,
		`{"name":""}`,
	}
	for _, reqBody := range traversalAttempts {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/v1/bootloader/check-file",
			strings.NewReader(reqBody))
		r.Header.Set("Content-Type", "application/json")
		h.CheckFile(w, r)

		resp := w.Result()
		if resp.StatusCode == http.StatusOK {
			t.Errorf("expected non-200 for traversal attempt %s, got %d", reqBody, resp.StatusCode)
		}
	}
}

func TestCheckFile_InvalidBody(t *testing.T) {
	bfs := tempBootFS(t)
	h := NewBootloaderHandler(bfs)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/bootloader/check-file",
		strings.NewReader(`not json`))
	r.Header.Set("Content-Type", "application/json")
	h.CheckFile(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCheckFile_EmptyName(t *testing.T) {
	bfs := tempBootFS(t)
	h := NewBootloaderHandler(bfs)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/api/v1/bootloader/check-file",
		strings.NewReader(`{"name":""}`))
	r.Header.Set("Content-Type", "application/json")
	h.CheckFile(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}
