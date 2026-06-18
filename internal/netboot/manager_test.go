package netboot

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager(&Catalog{})
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestManagerGetDistro(t *testing.T) {
	cat := &Catalog{
		Distros: []*Distro{
			{Name: "Ubuntu", MenuGroup: "linux"},
			{Name: "Debian", MenuGroup: "linux"},
		},
	}
	m := NewManager(cat)
	d := m.GetDistro("Ubuntu")
	if d == nil {
		t.Fatal("expected Ubuntu distro")
	}
	if m.GetDistro("NonExistent") != nil {
		t.Fatal("expected nil for non-existent distro")
	}
}

func TestManagerGroups(t *testing.T) {
	cat := &Catalog{
		Distros: []*Distro{
			{Name: "Ubuntu", Enabled: true, MenuGroup: "linux"},
			{Name: "Debian", Enabled: true, MenuGroup: "linux"},
			{Name: "FreeBSD", Enabled: true, MenuGroup: "bsd"},
			{Name: "Disabled", Enabled: false, MenuGroup: "linux"},
		},
	}
	m := NewManager(cat)
	groups := m.Groups()
	foundLinux := false
	foundBSD := false
	for _, g := range groups {
		if g.Name == "linux" {
			foundLinux = true
			if len(g.Distros) != 2 {
				t.Fatalf("expected 2 linux distros, got %d", len(g.Distros))
			}
		}
		if g.Name == "bsd" {
			foundBSD = true
		}
	}
	if !foundLinux {
		t.Fatal("expected linux group")
	}
	if !foundBSD {
		t.Fatal("expected bsd group")
	}
}
