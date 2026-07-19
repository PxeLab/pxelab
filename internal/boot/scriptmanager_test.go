package boot

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "scriptmanager-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestNewScriptManager(t *testing.T) {
	dir := tempDir(t)
	sm := NewScriptManager(dir)
	if sm.rootDir != dir {
		t.Errorf("rootDir = %q, want %q", sm.rootDir, dir)
	}
	// verify .scripts/versions dir created
	if _, err := os.Stat(filepath.Join(dir, ".scripts", "versions")); err != nil {
		t.Errorf(".scripts/versions not created: %v", err)
	}
}

func TestSaveAndLoad(t *testing.T) {
	sm := NewScriptManager(tempDir(t))

	ver, err := sm.Save("test-ubuntu", "prof1", "Ubuntu", "custom", "#!ipxe\necho hello", "initial")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if ver == nil {
		t.Fatal("Save returned nil version")
	}
	if ver.Content != "#!ipxe\necho hello" {
		t.Errorf("version content = %q", ver.Content)
	}
	if ver.Comment != "initial" {
		t.Errorf("version comment = %q", ver.Comment)
	}
	if ver.Checksum == "" {
		t.Error("checksum should not be empty")
	}

	content, err := sm.Load("test-ubuntu")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if content != "#!ipxe\necho hello" {
		t.Errorf("loaded content = %q", content)
	}
}

func TestLoadNonexistent(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	_, err := sm.Load("no-such-script")
	if err == nil {
		t.Fatal("expected error loading nonexistent script")
	}
}

func TestSaveIfChanged_SameContent(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	sm.Save("test-script", "", "Script", "custom", "content", "")

	ver, changed, err := sm.SaveIfChanged("test-script", "", "Script", "custom", "content", "")
	if err != nil {
		t.Fatalf("SaveIfChanged: %v", err)
	}
	if changed {
		t.Error("expected changed=false for same content")
	}
	if ver != nil {
		t.Error("expected nil version when unchanged")
	}
}

func TestSaveIfChanged_DifferentContent(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	sm.Save("test-script", "", "Script", "custom", "v1", "")

	ver, changed, err := sm.SaveIfChanged("test-script", "", "Script", "custom", "v2", "updated")
	if err != nil {
		t.Fatalf("SaveIfChanged: %v", err)
	}
	if !changed {
		t.Error("expected changed=true for different content")
	}
	if ver == nil {
		t.Fatal("expected non-nil version")
	}
	if ver.Content != "v2" {
		t.Errorf("version content = %q, want %q", ver.Content, "v2")
	}

	// verify two versions exist
	versions, _ := sm.ListVersions("test-script")
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
}

func TestSaveIfChanged_FirstSave(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	_, changed, err := sm.SaveIfChanged("new-script", "p1", "New", "chain", "#!ipxe\nchain foo", "first")
	if err != nil {
		t.Fatalf("SaveIfChanged (first): %v", err)
	}
	if !changed {
		t.Error("expected changed=true for first save")
	}
}

func TestGetMeta(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	sm.Save("test-mymeta", "profX", "My Label", "direct", "script body", "save")

	meta, err := sm.GetMeta("test-mymeta")
	if err != nil {
		t.Fatalf("GetMeta: %v", err)
	}
	if meta.ProfileID != "profX" {
		t.Errorf("ProfileID = %q, want %q", meta.ProfileID, "profX")
	}
	if meta.Label != "My Label" {
		t.Errorf("Label = %q, want %q", meta.Label, "My Label")
	}
	if meta.Type != "direct" {
		t.Errorf("Type = %q, want %q", meta.Type, "direct")
	}
	if meta.ID != "test-mymeta" {
		t.Errorf("ID = %q", meta.ID)
	}
}

func TestGetMetaNonexistent(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	_, err := sm.GetMeta("no-such")
	if err == nil {
		t.Fatal("expected error for nonexistent meta")
	}
}

func TestListVersions(t *testing.T) {
	sm := NewScriptManager(tempDir(t))

	// empty
	versions, err := sm.ListVersions("test-empty")
	if err != nil {
		t.Fatalf("ListVersions (empty): %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}

	// multiple saves
	sm.Save("test-versions", "", "", "", "v1", "")
	sm.Save("test-versions", "", "", "", "v2", "")
	sm.Save("test-versions", "", "", "", "v3", "")

	versions, err = sm.ListVersions("test-versions")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d", len(versions))
	}
	// should be sorted newest first
	if versions[0].Content != "v3" {
		t.Errorf("newest version should be v3, got %s", versions[0].Content)
	}
	if versions[2].Content != "v1" {
		t.Errorf("oldest version should be v1, got %s", versions[2].Content)
	}
}

func TestGetVersion(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	ver, _ := sm.Save("test-getver", "", "", "", "content v1", "")
	verID := ver.ID

	got, err := sm.GetVersion("test-getver", verID)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if got.Content != "content v1" {
		t.Errorf("content = %q", got.Content)
	}
	if got.ID != verID {
		t.Errorf("ID = %q, want %q", got.ID, verID)
	}
}

func TestGetVersionNonexistent(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	_, err := sm.GetVersion("test-none", "nover")
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}

func TestDiff(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	ver, _ := sm.Save("test-diff", "", "", "", "line1\nline2\nline3", "")
	sm.Save("test-diff", "", "", "", "line1\nmodified\nline3", "")

	diff, err := sm.Diff("test-diff", ver.ID)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(diff, "- line2") {
		t.Errorf("diff should contain '- line2':\n%s", diff)
	}
	if !strings.Contains(diff, "+ modified") {
		t.Errorf("diff should contain '+ modified':\n%s", diff)
	}
	if !strings.Contains(diff, "  line1") {
		t.Errorf("diff should contain '  line1':\n%s", diff)
	}
}

func TestRollbackPreservesMeta(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	sm.Save("test-rollback", "prof1", "Rollback Test", "custom", "v1", "first")
	sm.Save("test-rollback", "prof1", "Rollback Test", "custom", "v2", "second")

	versions, _ := sm.ListVersions("test-rollback")
	var v1ID string
	for _, v := range versions {
		if v.Content == "v1" {
			v1ID = v.ID
			break
		}
	}
	if v1ID == "" {
		t.Fatal("could not find v1 version")
	}

	ver, err := sm.Rollback("test-rollback", v1ID, "going back")
	if err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	if ver.Content != "v1" {
		t.Errorf("rollback content = %q, want %q", ver.Content, "v1")
	}

	// meta should be preserved
	meta, _ := sm.GetMeta("test-rollback")
	if meta.ProfileID != "prof1" {
		t.Errorf("Rollback lost ProfileID: got %q", meta.ProfileID)
	}
	if meta.Label != "Rollback Test" {
		t.Errorf("Rollback lost Label: got %q", meta.Label)
	}

	// current content should be v1
	content, _ := sm.Load("test-rollback")
	if content != "v1" {
		t.Errorf("current = %q, want %q", content, "v1")
	}
}

func TestListScripts(t *testing.T) {
	sm := NewScriptManager(tempDir(t))

	// empty
	scripts, err := sm.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts (empty): %v", err)
	}
	if len(scripts) != 0 {
		t.Errorf("expected 0 scripts, got %d", len(scripts))
	}

	// multiple
	sm.Save("script-a", "p1", "Alpha", "chain", "alpha", "")
	sm.Save("script-b", "p2", "Beta", "direct", "beta", "")

	scripts, err = sm.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts: %v", err)
	}
	if len(scripts) != 2 {
		t.Errorf("expected 2 scripts, got %d", len(scripts))
	}
}

func TestValidateScriptID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"", false},
		{"a", true},
		{"profile-label", true},
		{"ubuntu-22.04-install", true},
		{"test_script", true},
		{"a/b", false},
		{"a\\b", false},
		{"../escape", false},
		{"a..b", true},   // dots alone are fine
		{"..a", true},    // starting with dots is fine
		{"a~b", false},
		{strings.Repeat("a", 257), false},
		{"abc def", false},
	}
	for _, tc := range tests {
		err := validateScriptID(tc.id)
		if tc.valid && err != nil {
			t.Errorf("validateScriptID(%q) = %v, want nil", tc.id, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("validateScriptID(%q) = nil, want error", tc.id)
		}
	}
}

func TestPathTraversalRejected(t *testing.T) {
	sm := NewScriptManager(tempDir(t))

	traversalIDs := []string{
		"../etc/passwd",
		"foo/../../etc",
		"..\\windows",
		"a/../b",
	}
	for _, id := range traversalIDs {
		_, err := sm.Save(id, "", "", "", "content", "")
		if err == nil {
			t.Errorf("expected error for scriptID %q", id)
		}
		_, err = sm.Load(id)
		if err == nil {
			t.Errorf("expected Load error for scriptID %q", id)
		}
	}
}

func TestConcurrentSaves(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	var wg sync.WaitGroup
	n := 10

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			content := "content from goroutine " + string(rune('0'+i))
			_, err := sm.Save("test-concurrent", "", "", "", content, "")
			if err != nil {
				t.Errorf("concurrent Save: %v", err)
			}
		}(i)
	}
	wg.Wait()

	versions, err := sm.ListVersions("test-concurrent")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) == 0 {
		t.Error("expected at least 1 version from concurrent saves")
	}
}

func TestConcurrentSaveIfChanged(t *testing.T) {
	sm := NewScriptManager(tempDir(t))

	// First save to create the script
	sm.Save("test-concurrent-sic", "", "", "", "base", "")

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := sm.SaveIfChanged("test-concurrent-sic", "", "", "", "base", "")
			if err != nil {
				t.Errorf("concurrent SaveIfChanged: %v", err)
			}
		}()
	}
	wg.Wait()

	// All goroutines should see no change — still 1 version
	versions, _ := sm.ListVersions("test-concurrent-sic")
	if len(versions) != 1 {
		t.Errorf("expected 1 version (all goroutines saw same content), got %d", len(versions))
	}
}

func TestSimpleDiff_Identical(t *testing.T) {
	diff := simpleDiff("line1\nline2", "line1\nline2")
	if diff != "" {
		t.Errorf("identical strings should produce empty diff, got:\n%s", diff)
	}
}

func TestSimpleDiff_EmptyStrings(t *testing.T) {
	diff := simpleDiff("", "")
	if diff != "" {
		t.Errorf("empty strings should produce empty diff, got:\n%s", diff)
	}

	diff = simpleDiff("", "new content")
	if !strings.Contains(diff, "+ new content") {
		t.Errorf("should show added lines:\n%s", diff)
	}

	diff = simpleDiff("old content", "")
	if !strings.Contains(diff, "- old content") {
		t.Errorf("should show removed lines:\n%s", diff)
	}
}

func TestSimpleDiff_TrailingNewline(t *testing.T) {
	// with and without trailing newline should diff cleanly
	diff := simpleDiff("a\nb\nc\n", "a\nb\nc")
	if diff != "" {
		t.Errorf("trailing newline difference should be ignored, got:\n%s", diff)
	}
}

func TestSimpleDiff_CompletelyDifferent(t *testing.T) {
	old := "a\nb\nc"
	new := "x\ny\nz"
	diff := simpleDiff(old, new)
	if !strings.Contains(diff, "- a") || !strings.Contains(diff, "- b") || !strings.Contains(diff, "- c") {
		t.Errorf("should show all old lines removed:\n%s", diff)
	}
	if !strings.Contains(diff, "+ x") || !strings.Contains(diff, "+ y") || !strings.Contains(diff, "+ z") {
		t.Errorf("should show all new lines added:\n%s", diff)
	}
}

func TestSimpleDiff_InsertInMiddle(t *testing.T) {
	old := "a\nc"
	new := "a\nb\nc"
	diff := simpleDiff(old, new)
	if !strings.Contains(diff, "+ b") {
		t.Errorf("should show '+ b':\n%s", diff)
	}
}

func TestSave_EmptyContent(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	ver, err := sm.Save("test-empty", "", "", "", "", "")
	if err != nil {
		t.Fatalf("Save empty content: %v", err)
	}
	if ver.Content != "" {
		t.Errorf("content = %q", ver.Content)
	}
	content, _ := sm.Load("test-empty")
	if content != "" {
		t.Errorf("loaded content = %q", content)
	}
}

func TestListScripts_IgnoresCorrupt(t *testing.T) {
	sm := NewScriptManager(tempDir(t))

	// create a valid script
	sm.Save("valid-script", "p1", "Valid", "custom", "ok", "")

	// corrupt the meta of another "script" by creating a dir with no meta.json
	os.MkdirAll(filepath.Join(sm.rootDir, ".scripts", "corrupt-dir"), 0755)

	scripts, err := sm.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts: %v", err)
	}
	if len(scripts) != 1 {
		t.Errorf("expected 1 script (corrupt dir ignored), got %d", len(scripts))
	}
}

func TestGetVersion_InvalidVerID(t *testing.T) {
	sm := NewScriptManager(tempDir(t))
	sm.Save("test-secure", "", "", "", "content", "")

	invalidIDs := []string{"", "../etc", "foo/bar", "a\\b"}
	for _, id := range invalidIDs {
		_, err := sm.GetVersion("test-secure", id)
		if err == nil {
			t.Errorf("expected error for verID %q", id)
		}
	}
}
