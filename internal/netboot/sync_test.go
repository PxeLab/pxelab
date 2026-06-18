package netboot

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMapAnsibleMenu(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"linux", "linux"},
		{"bsd", "bsd"},
		{"live", "live"},
		{"unknown", "tools"},
	}
	for _, tt := range tests {
		got := mapAnsibleMenu(tt.input)
		if got != tt.expected {
			t.Errorf("mapAnsibleMenu(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestInferInstallType(t *testing.T) {
	if inferInstallType("bsd") != InstallBSD {
		t.Fatal("expected BSD install type")
	}
	if inferInstallType("live") != InstallLive {
		t.Fatal("expected live install type")
	}
	if inferInstallType("linux") != InstallLegacy {
		t.Fatal("expected legacy install type")
	}
}

func TestConstructRemoteURL(t *testing.T) {
	ref := constructRemoteURL("http://archive.ubuntu.com", "ubuntu", "noble", "amd64")
	if ref == nil {
		t.Fatal("expected file ref")
	}
	if ref.Kernel != "http://archive.ubuntu.com/ubuntu/noble/amd64/linux" {
		t.Fatalf("unexpected kernel URL: %s", ref.Kernel)
	}
	if ref.Initrd != "http://archive.ubuntu.com/ubuntu/noble/amd64/initrd.gz" {
		t.Fatalf("unexpected initrd URL: %s", ref.Initrd)
	}
}

func TestConstructRemoteURL_EmptyMirror(t *testing.T) {
	ref := constructRemoteURL("", "ubuntu", "noble", "amd64")
	if ref != nil {
		t.Fatal("expected nil for empty mirror")
	}
}

func TestLoadEndpoints_FileNotFound(t *testing.T) {
	eps := loadEndpoints("/nonexistent/path.yml")
	if eps != nil {
		t.Fatal("expected nil for missing file")
	}
}

func TestSyncFromUpstream_NoRepo(t *testing.T) {
	err := SyncFromUpstream("", "/tmp/test-catalog")
	if err == nil {
		t.Fatal("expected error for empty repo dir")
	}
}

func TestSyncFromUpstream_NoCatalog(t *testing.T) {
	err := SyncFromUpstream("/tmp/test-repo", "")
	if err == nil {
		t.Fatal("expected error for empty catalog dir")
	}
}

func TestSyncFromUpstream_RepoNotExist(t *testing.T) {
	dir := t.TempDir()
	err := SyncFromUpstream("/nonexistent/repo/path", dir)
	if err == nil {
		t.Fatal("expected error for non-existent repo")
	}
}

func TestBuildURLFromEndpoint(t *testing.T) {
	ep := endpointConfig{
		KernelTemplate: "{mirror}/dists/{version}/main/installer-{arch}/current/images/netboot/ubuntu-installer/{arch}/linux",
		InitrdTemplate: "{mirror}/dists/{version}/main/installer-{arch}/current/images/netboot/ubuntu-installer/{arch}/initrd.gz",
	}
	ref := buildURLFromEndpoint(ep, "noble", "amd64", "http://archive.ubuntu.com/ubuntu")
	if ref == nil {
		t.Fatal("expected file ref")
	}
	if !strings.Contains(ref.Kernel, "noble") {
		t.Fatal("expected version in URL")
	}
	if !strings.Contains(ref.Kernel, "amd64") {
		t.Fatal("expected arch in URL")
	}
}

func TestBuildURLFromEndpoint_EmptyTemplate(t *testing.T) {
	ref := buildURLFromEndpoint(endpointConfig{}, "noble", "amd64", "http://archive.ubuntu.com")
	if ref != nil {
		t.Fatal("expected nil for empty template")
	}
}

func TestBuildURLFromEndpoint_NoInitrdTemplate(t *testing.T) {
	ep := endpointConfig{
		KernelTemplate: "{mirror}/vmlinuz-{version}-{arch}",
	}
	ref := buildURLFromEndpoint(ep, "noble", "amd64", "http://archive.ubuntu.com")
	if ref == nil {
		t.Fatal("expected file ref")
	}
	if !strings.Contains(ref.Initrd, "initrd.img") {
		t.Fatalf("expected derived initrd.img from vmlinuz template, got: %s", ref.Initrd)
	}
}

func TestSyncFromUpstream_DefaultsFileNotFound(t *testing.T) {
	// Create a minimal git repo without the expected defaults file
	repoDir := t.TempDir()
	// Init a git repo so gitPull passes
	_, err := runCmd(repoDir, "git", "init")
	if err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_, err = runCmd(repoDir, "git", "config", "user.email", "test@test.com")
	if err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	_, err = runCmd(repoDir, "git", "config", "user.name", "Test")
	if err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	catalogDir := t.TempDir()
	err = SyncFromUpstream(repoDir, catalogDir)
	if err == nil {
		t.Fatal("expected error for missing defaults file")
	}
	if !strings.Contains(err.Error(), "defaults file not found") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSyncFromUpstream_NoGitRepo(t *testing.T) {
	repoDir := t.TempDir() // No .git directory
	catalogDir := t.TempDir()
	err := SyncFromUpstream(repoDir, catalogDir)
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSyncFromUpstream_Success(t *testing.T) {
	// Create a minimal git repo
	repoDir := t.TempDir()
	_, err := runCmd(repoDir, "git", "init")
	if err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_, err = runCmd(repoDir, "git", "config", "user.email", "test@test.com")
	if err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	_, err = runCmd(repoDir, "git", "config", "user.name", "Test")
	if err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	// Create the Ansible defaults structure
	defaultsDir := filepath.Join(repoDir, "roles", "netbootxyz", "defaults")
	err = os.MkdirAll(defaultsDir, 0755)
	if err != nil {
		t.Fatalf("mkdir defaults: %v", err)
	}

	ansibleDefaults := `releases:
  ubuntu:
    name: Ubuntu
    enabled: true
    menu: linux
    base_dir: ubuntu
    mirror: http://archive.ubuntu.com/ubuntu
    versions:
      - code_name: noble
        name: "24.04"
        arch: amd64
      - code_name: jammy
        name: "22.04"
        arch: amd64
  debian:
    name: Debian
    enabled: true
    menu: linux
    base_dir: debian
    mirror: http://deb.debian.org/debian
    versions:
      - code_name: bookworm
        name: "12"
        arch: amd64
  freebsd:
    name: FreeBSD
    enabled: true
    menu: bsd
    base_dir: freebsd
    versions:
      - code_name: 13.2
        name: "13.2-RELEASE"
        arch: amd64
  disabled_distro:
    name: DisabledOS
    enabled: false
    menu: linux
    versions: []
`

	err = os.WriteFile(filepath.Join(defaultsDir, "main.yml"), []byte(ansibleDefaults), 0644)
	if err != nil {
		t.Fatalf("write defaults: %v", err)
	}

	// Create an initial commit so git pull doesn't fail on uncommitted state
	_, err = runCmd(repoDir, "git", "add", "-A")
	if err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	_, err = runCmd(repoDir, "git", "commit", "-m", "initial")
	if err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	catalogDir := t.TempDir()
	err = SyncFromUpstream(repoDir, catalogDir)
	if err != nil {
		t.Fatalf("SyncFromUpstream failed: %v", err)
	}

	// Verify catalog files were created
	catalog, err := LoadCatalog(catalogDir)
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}
	if len(catalog.Distros) != 3 {
		t.Fatalf("expected 3 distros, got %d", len(catalog.Distros))
	}

	// Check specific distros
	for _, d := range catalog.Distros {
		switch d.Name {
		case "Ubuntu":
			if d.MenuGroup != GroupLinux {
				t.Errorf("Ubuntu menu_group: expected %s, got %s", GroupLinux, d.MenuGroup)
			}
			if len(d.Versions) != 2 {
				t.Errorf("Ubuntu versions: expected 2, got %d", len(d.Versions))
			}
			if d.Mirror != "http://archive.ubuntu.com/ubuntu" {
				t.Errorf("Ubuntu mirror: expected http://archive.ubuntu.com/ubuntu, got %s", d.Mirror)
			}
		case "Debian":
			if d.MenuGroup != GroupLinux {
				t.Errorf("Debian menu_group: expected %s, got %s", GroupLinux, d.MenuGroup)
			}
			if len(d.Versions) != 1 {
				t.Errorf("Debian versions: expected 1, got %d", len(d.Versions))
			}
		case "FreeBSD":
			if d.MenuGroup != GroupBSD {
				t.Errorf("FreeBSD menu_group: expected %s, got %s", GroupBSD, d.MenuGroup)
			}
			if len(d.Versions) != 1 {
				t.Errorf("FreeBSD versions: expected 1, got %d", len(d.Versions))
			}
			if len(d.Versions) > 0 {
				installType := d.Versions[0].InstallType
				if installType != InstallBSD {
					t.Errorf("FreeBSD install_type: expected %s, got %s", InstallBSD, installType)
				}
			}
		default:
			t.Errorf("unexpected distro: %s", d.Name)
		}
	}
}

func TestSyncFromUpstream_WithEndpoints(t *testing.T) {
	repoDir := t.TempDir()
	_, err := runCmd(repoDir, "git", "init")
	if err != nil {
		t.Fatalf("git init failed: %v", err)
	}
	_, err = runCmd(repoDir, "git", "config", "user.email", "test@test.com")
	if err != nil {
		t.Fatalf("git config user.email failed: %v", err)
	}
	_, err = runCmd(repoDir, "git", "config", "user.name", "Test")
	if err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}

	defaultsDir := filepath.Join(repoDir, "roles", "netbootxyz", "defaults")
	err = os.MkdirAll(defaultsDir, 0755)
	if err != nil {
		t.Fatalf("mkdir defaults: %v", err)
	}

	ansibleDefaults := `releases:
  ubuntu:
    name: Ubuntu
    enabled: true
    menu: linux
    base_dir: ubuntu
    mirror: http://archive.ubuntu.com/ubuntu
    versions:
      - code_name: noble
        name: "24.04"
        arch: amd64
`

	err = os.WriteFile(filepath.Join(defaultsDir, "main.yml"), []byte(ansibleDefaults), 0644)
	if err != nil {
		t.Fatalf("write defaults: %v", err)
	}

	// Create endpoints file
	endpointsYAML := `endpoints:
  ubuntu:
    kernel: "{mirror}/dists/{version}/main/installer-{arch}/current/images/netboot/ubuntu-installer/{arch}/linux"
    initrd: "{mirror}/dists/{version}/main/installer-{arch}/current/images/netboot/ubuntu-installer/{arch}/initrd.gz"
`
	err = os.WriteFile(filepath.Join(repoDir, "endpoints.yml"), []byte(endpointsYAML), 0644)
	if err != nil {
		t.Fatalf("write endpoints: %v", err)
	}

	_, err = runCmd(repoDir, "git", "add", "-A")
	if err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	_, err = runCmd(repoDir, "git", "commit", "-m", "initial")
	if err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	catalogDir := t.TempDir()
	err = SyncFromUpstream(repoDir, catalogDir)
	if err != nil {
		t.Fatalf("SyncFromUpstream failed: %v", err)
	}

	catalog, err := LoadCatalog(catalogDir)
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}
	if len(catalog.Distros) != 1 {
		t.Fatalf("expected 1 distro, got %d", len(catalog.Distros))
	}
	if len(catalog.Distros[0].Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(catalog.Distros[0].Versions))
	}
	v := catalog.Distros[0].Versions[0]
	if v.Remote == nil {
		t.Fatal("expected remote endpoint URL")
	}
	if !strings.Contains(v.Remote.Kernel, "installer-amd64") {
		t.Fatalf("unexpected kernel URL: %s", v.Remote.Kernel)
	}
}

// runCmd is a helper to run a command in a specific directory and return output or error
func runCmd(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
