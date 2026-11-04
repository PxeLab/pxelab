package updatecheck

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newTestChecker(t *testing.T, srv *httptest.Server) *Checker {
	t.Helper()
	c := NewChecker("0.1.0", t.TempDir())
	c.apiBase = srv.URL
	c.downloadBase = srv.URL + "/dl"
	c.repo = "PxeLab/pxelab"
	c.client = srv.Client()
	c.downloadClient = srv.Client()
	c.lastResult = &CheckResult{
		CurrentVersion:  "0.1.0",
		LatestVersion:   "0.2.0",
		UpdateAvailable: true,
		ReleaseInfo:     &ReleaseInfo{LatestVersion: "0.2.0"},
	}
	return c
}

func assetForTest() string {
	return AssetName("0.2.0", runtime.GOOS, runtime.GOARCH)
}

func TestDownloadViaGitHubAPI(t *testing.T) {
	asset := assetForTest()
	var sawAssetRequest bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/PxeLab/pxelab/releases/tags/v0.2.0":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"assets": []map[string]any{{"id": 42, "name": asset}},
			})
		case "/repos/PxeLab/pxelab/releases/assets/42":
			sawAssetRequest = true
			if got := r.Header.Get("Accept"); got != "application/octet-stream" {
				t.Errorf("asset Accept = %q, want application/octet-stream", got)
			}
			_, _ = io.WriteString(w, "update-bytes")
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestChecker(t, srv)
	result, err := c.Download()
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !sawAssetRequest {
		t.Fatal("expected download through the GitHub asset API")
	}
	if result.FileName != asset {
		t.Fatalf("FileName = %q, want %q", result.FileName, asset)
	}
	data, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != "update-bytes" {
		t.Fatalf("downloaded content = %q", data)
	}
	if result.FileSize != int64(len("update-bytes")) {
		t.Fatalf("FileSize = %d, want %d", result.FileSize, len("update-bytes"))
	}
}

func TestDownloadFallsBackToDirectURL(t *testing.T) {
	asset := assetForTest()
	var directRequested bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/PxeLab/pxelab/releases/tags/v0.2.0":
			http.Error(w, "api unavailable", http.StatusInternalServerError)
		case "/dl/PxeLab/pxelab/releases/download/v0.2.0/" + asset:
			directRequested = true
			_, _ = io.WriteString(w, "direct-bytes")
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestChecker(t, srv)
	result, err := c.Download()
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if !directRequested {
		t.Fatal("expected fallback to the direct release URL")
	}
	data, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != "direct-bytes" {
		t.Fatalf("downloaded content = %q", data)
	}
}

func TestDownloadReportsMissingAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/PxeLab/pxelab/releases/tags/v0.2.0":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"assets":[{"id":1,"name":"some-other-asset.zip"}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestChecker(t, srv)
	_, err := c.Download()
	if err == nil {
		t.Fatal("Download() error = nil, want failure")
	}
	for _, want := range []string{"缺少资源", "HTTP 404"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want it to contain %q", err, want)
		}
	}
}

func TestDownloadSkipsExistingFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newTestChecker(t, srv)
	asset := assetForTest()
	dir := filepath.Join(c.dataDir, "updates")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, asset)
	if err := os.WriteFile(path, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := c.Download()
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if result.FilePath != path || result.FileSize != int64(len("existing")) {
		t.Fatalf("result = %+v, want existing file", result)
	}
}
