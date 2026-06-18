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
