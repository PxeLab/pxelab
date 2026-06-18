package netboot

import (
	"testing"
)

func TestDistroDefaults(t *testing.T) {
	d := Distro{
		Name:      "Ubuntu",
		Enabled:   true,
		MenuGroup: "linux",
		Mirror:    "http://archive.ubuntu.com",
	}
	if d.Name != "Ubuntu" {
		t.Fatalf("expected Ubuntu, got %s", d.Name)
	}
	if !d.Enabled {
		t.Fatal("expected enabled")
	}
}

func TestVersionLocalPriority(t *testing.T) {
	v := Version{
		Codename: "noble",
		Name:     "24.04",
		Arch:     "amd64",
		Local: &FileRef{
			Kernel: "ubuntu/24.04/vmlinuz",
			Initrd: "ubuntu/24.04/initrd",
		},
		Remote: &FileRef{
			Kernel: "http://archive.ubuntu.com/.../linux",
			Initrd: "http://archive.ubuntu.com/.../initrd.gz",
		},
	}
	if v.Local == nil {
		t.Fatal("expected local file ref")
	}
	if v.Remote == nil {
		t.Fatal("expected remote file ref")
	}
}

func TestCatalogGroups(t *testing.T) {
	c := Catalog{
		Distros: []*Distro{
			{Name: "Ubuntu", Enabled: true, MenuGroup: "linux"},
			{Name: "FreeBSD", Enabled: true, MenuGroup: "bsd"},
			{Name: "Clonezilla", Enabled: true, MenuGroup: "tools"},
		},
	}
	groups := c.Groups()
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if groups[0].Name != "linux" {
		t.Fatalf("expected first group 'linux', got '%s'", groups[0].Name)
	}
}
