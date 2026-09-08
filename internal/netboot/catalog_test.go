package netboot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDistro(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
name: Ubuntu
enabled: true
menu_group: linux
mirror: http://archive.ubuntu.com
versions:
  - codename: noble
    name: 24.04 LTS Noble Numbat
    arch: amd64
    enabled: true
    remote:
      kernel: http://archive.ubuntu.com/ubuntu/dists/noble/linux
      initrd: http://archive.ubuntu.com/ubuntu/dists/noble/initrd.gz
`
	path := filepath.Join(dir, "ubuntu.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	d, err := LoadDistro(path)
	if err != nil {
		t.Fatalf("LoadDistro failed: %v", err)
	}
	if d.Name != "Ubuntu" {
		t.Fatalf("expected Ubuntu, got %s", d.Name)
	}
	if len(d.Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(d.Versions))
	}
	if d.Versions[0].Codename != "noble" {
		t.Fatalf("expected codename noble, got %s", d.Versions[0].Codename)
	}
	if d.Versions[0].Remote == nil {
		t.Fatal("expected remote file ref")
	}
}

func TestLoadCatalog(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"ubuntu.yaml": `
name: Ubuntu
enabled: true
menu_group: linux
versions:
  - codename: noble
    name: "24.04"
    arch: amd64
    enabled: true
`,
		"debian.yaml": `
name: Debian
enabled: true
menu_group: linux
versions:
  - codename: bookworm
    name: "12"
    arch: amd64
    enabled: true
`,
		"disabled.yaml": `
name: DisabledOS
enabled: false
menu_group: linux
versions: []
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	c, err := LoadCatalog(dir)
	if err != nil {
		t.Fatalf("LoadCatalog failed: %v", err)
	}
	if len(c.Distros) != 3 {
		t.Fatalf("expected 3 distros, got %d", len(c.Distros))
	}
}

func TestSetWindowsLocal(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
name: Windows PE
enabled: true
menu_group: windows
versions:
  - codename: win11-x64
    name: "Windows 11 / Server 2025 PE (x64)"
    arch: amd64
    enabled: true
    type: wimboot
    remote:
      kernel: http://boot.netboot.xyz/wimboot
      initrd: http://boot.netboot.xyz/windows/x64
  - codename: win10-x64
    name: "Windows 10 / Server 2022 PE (x64)"
    arch: amd64
    type: wimboot
    remote:
      kernel: http://boot.netboot.xyz/wimboot
      initrd: http://boot.netboot.xyz/windows/x64
`
	if err := os.WriteFile(filepath.Join(dir, "windows.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	c, err := SetWindowsLocal(dir, "http://10.0.0.10/netboot/menu/wimboot", "http://10.0.0.10/boot/isos/123-win11")
	if err != nil {
		t.Fatalf("SetWindowsLocal failed: %v", err)
	}
	win := findDistro(c, "Windows PE")
	if win == nil {
		t.Fatal("catalog missing Windows PE distro")
	}
	if win.Versions[0].Local == nil {
		t.Fatal("win11-x64 should have a local ref")
	}
	if win.Versions[0].Local.Kernel != "http://10.0.0.10/netboot/menu/wimboot" ||
		win.Versions[0].Local.Initrd != "http://10.0.0.10/boot/isos/123-win11" {
		t.Errorf("unexpected win11 local: %+v", win.Versions[0].Local)
	}
	// disabled 版本不应被改写
	if win.Versions[1].Local != nil {
		t.Errorf("disabled win10-x64 should keep remote-only, got local %+v", win.Versions[1].Local)
	}

	// 落盘持久化后可重新加载
	c2, err := LoadCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	win2 := findDistro(c2, "Windows PE")
	if win2 == nil || win2.Versions[0].Local == nil {
		t.Fatalf("local ref lost after reload: %+v", win2)
	}
}

func TestSetWindowsLocalNoWindowsEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ubuntu.yaml"),
		[]byte("name: Ubuntu\nenabled: true\nmenu_group: linux\nversions: []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := SetWindowsLocal(dir, "u", "v"); err != ErrNoWindowsEntry {
		t.Fatalf("expected ErrNoWindowsEntry, got %v", err)
	}

	empty := t.TempDir()
	if _, err := SetWindowsLocal(empty, "u", "v"); err != ErrNoWindowsEntry {
		t.Fatalf("expected ErrNoWindowsEntry for empty dir, got %v", err)
	}
}

func findDistro(c *Catalog, name string) *Distro {
	for _, d := range c.Distros {
		if d.Name == name {
			return d
		}
	}
	return nil
}

func TestSaveDistro(t *testing.T) {
	dir := t.TempDir()
	d := &Distro{
		Name:      "TestOS",
		Enabled:   true,
		MenuGroup: "linux",
		Versions: []*Version{{
			Codename: "v1",
			Name:     "Version 1",
			Arch:     "amd64",
			Enabled:  true,
		}},
	}
	path := filepath.Join(dir, "testos.yaml")
	if err := SaveDistro(path, d); err != nil {
		t.Fatalf("SaveDistro failed: %v", err)
	}

	loaded, err := LoadDistro(path)
	if err != nil {
		t.Fatalf("LoadDistro after save failed: %v", err)
	}
	if loaded.Name != "TestOS" {
		t.Fatalf("expected TestOS, got %s", loaded.Name)
	}
}
