package netboot

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed embedded/*.yaml
var embeddedFS embed.FS

// DefaultCatalog returns the built-in catalog shipped with the binary.
// Generated from netboot.xyz — covers 27+ distros with versions and remote URLs.
// Users can override any distro by placing a YAML file with the same name
// in <data_dir>/netboot/catalog/; disk files take precedence at startup.
func DefaultCatalog() *Catalog {
	c := &Catalog{}
	entries, err := fs.ReadDir(embeddedFS, "embedded")
	if err != nil {
		return c
	}
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		data, err := fs.ReadFile(embeddedFS, path.Join("embedded", e.Name()))
		if err != nil {
			continue
		}
		var d Distro
		if err := yaml.Unmarshal(data, &d); err != nil {
			continue
		}
		if d.Name == "" {
			continue
		}
		c.Distros = append(c.Distros, &d)
	}
	return c
}
