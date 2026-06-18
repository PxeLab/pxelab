package netboot

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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

// ExtractSeed extracts embedded catalog YAML files to targetDir.
// Only writes files that do not already exist — preserves user modifications.
func ExtractSeed(targetDir string) error {
	entries, err := fs.ReadDir(embeddedFS, "embedded")
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		targetPath := filepath.Join(targetDir, e.Name())
		if _, err := os.Stat(targetPath); err == nil {
			continue // don't overwrite existing files
		}
		data, err := fs.ReadFile(embeddedFS, path.Join("embedded", e.Name()))
		if err != nil {
			continue
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}
