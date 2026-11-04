package updatecheck

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func versionServer(t *testing.T, siteVersion, githubTag string, githubStatus int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version.json":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"latest_version":      siteVersion,
				"release_notes_url":   "https://example.com/notes/" + siteVersion,
				"download_url":        "https://example.com/download/" + siteVersion,
				"min_upgrade_version": "0.1.0",
			})
		case "/repos/PxeLab/pxelab/releases/latest":
			if githubStatus != http.StatusOK {
				http.Error(w, "boom", githubStatus)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name":     githubTag,
				"published_at": "2026-09-06T05:05:30Z",
				"html_url":     "https://github.com/PxeLab/pxelab/releases/tag/" + githubTag,
				"assets": []map[string]any{{
					"name":                 "pxelab_" + strings.TrimPrefix(githubTag, "v") + "_checksums.txt",
					"browser_download_url": "https://example.com/checksums/" + githubTag,
				}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestCheckPrefersNewerGitHubRelease(t *testing.T) {
	srv := versionServer(t, "0.1.0", "v0.2.0", http.StatusOK)
	defer srv.Close()

	c := NewChecker("0.1.0", t.TempDir())
	c.endpoint = srv.URL + "/version.json"
	c.apiBase = srv.URL
	c.client = srv.Client()

	result, err := c.Check()
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.LatestVersion != "0.2.0" {
		t.Fatalf("LatestVersion = %q, want 0.2.0", result.LatestVersion)
	}
	if !result.UpdateAvailable {
		t.Fatal("UpdateAvailable = false, want true")
	}
	if result.ReleaseInfo == nil || result.ReleaseInfo.ReleaseNotesURL != "https://github.com/PxeLab/pxelab/releases/tag/v0.2.0" {
		t.Fatalf("ReleaseInfo = %+v", result.ReleaseInfo)
	}
	if result.ReleaseInfo.ChecksumsURL != "https://example.com/checksums/v0.2.0" {
		t.Fatalf("ChecksumsURL = %q", result.ReleaseInfo.ChecksumsURL)
	}
}

func TestCheckKeepsSiteVersionWhenGitHubIsOlder(t *testing.T) {
	srv := versionServer(t, "0.2.0", "v0.1.0", http.StatusOK)
	defer srv.Close()

	c := NewChecker("0.1.0", t.TempDir())
	c.endpoint = srv.URL + "/version.json"
	c.apiBase = srv.URL
	c.client = srv.Client()

	result, err := c.Check()
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.LatestVersion != "0.2.0" {
		t.Fatalf("LatestVersion = %q, want site version 0.2.0", result.LatestVersion)
	}
	if result.ReleaseInfo.ChecksumsURL != "" {
		t.Fatalf("ChecksumsURL = %q, want empty (site value)", result.ReleaseInfo.ChecksumsURL)
	}
}

func TestCheckUsesGitHubWhenSiteFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/version.json":
			http.Error(w, "site down", http.StatusBadGateway)
		case "/repos/PxeLab/pxelab/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name":     "v0.2.0",
				"published_at": "2026-09-06T05:05:30Z",
				"html_url":     "https://github.com/PxeLab/pxelab/releases/tag/v0.2.0",
				"assets":       []map[string]any{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewChecker("0.1.0", t.TempDir())
	c.endpoint = srv.URL + "/version.json"
	c.apiBase = srv.URL
	c.client = srv.Client()

	result, err := c.Check()
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.LatestVersion != "0.2.0" || !result.UpdateAvailable {
		t.Fatalf("result = %+v, want 0.2.0 with update available", result)
	}
	if result.Error != "" {
		t.Fatalf("Error = %q, want empty", result.Error)
	}
}

func TestCheckFallsBackWhenGitHubFails(t *testing.T) {
	srv := versionServer(t, "0.1.0", "v9.9.9", http.StatusInternalServerError)
	defer srv.Close()

	c := NewChecker("0.0.9", t.TempDir())
	c.endpoint = srv.URL + "/version.json"
	c.apiBase = srv.URL
	c.client = srv.Client()

	result, err := c.Check()
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.LatestVersion != "0.1.0" {
		t.Fatalf("LatestVersion = %q, want fallback 0.1.0", result.LatestVersion)
	}
	if !result.UpdateAvailable {
		t.Fatal("UpdateAvailable = false, want true for 0.1.0 > 0.0.9")
	}
}
