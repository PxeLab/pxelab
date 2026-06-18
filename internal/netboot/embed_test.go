package netboot

import (
	"io/fs"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultCatalogHasData(t *testing.T) {
	// Debug: try parsing first file
	entries, err := fs.ReadDir(embeddedFS, "embedded")
	if err != nil {
		t.Fatalf("cannot read embedded dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded files")
	}

	// Try parsing the first file
	data, err := fs.ReadFile(embeddedFS, "embedded/"+entries[0].Name())
	if err != nil {
		t.Fatalf("read first file: %v", err)
	}
	var d Distro
	if err := yaml.Unmarshal(data, &d); err != nil {
		t.Fatalf("parse %s: %v\n---yaml---\n%s", entries[0].Name(), err, string(data))
	}
	t.Logf("parsed %s: name=%q versions=%d", entries[0].Name(), d.Name, len(d.Versions))

	c := DefaultCatalog()
	t.Logf("DefaultCatalog: %d distros", len(c.Distros))
	for _, d := range c.Distros {
		t.Logf("  %s: %d versions", d.Name, len(d.Versions))
	}
	if len(c.Distros) == 0 {
		t.Fatal("DefaultCatalog() returned empty catalog")
	}
}
