package netboot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CatalogDir returns the on-disk netboot catalog directory for a data dir.
// The catalog lives on disk (seeded from embedded YAML at startup) so users
// and store imports can override or extend the built-in entries.
func CatalogDir(dataDir string) string {
	return filepath.Join(dataDir, "netboot", "catalog")
}

// LoadDistro reads a single distro YAML file
func LoadDistro(path string) (*Distro, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read distro file %s: %w", path, err)
	}
	var d Distro
	if err := yaml.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse distro %s: %w", path, err)
	}
	if d.Name == "" {
		return nil, fmt.Errorf("distro file %s: name is required", path)
	}
	if d.MenuGroup == "" {
		d.MenuGroup = GroupLinux
	}
	return &d, nil
}

// SaveDistro writes a distro to a YAML file
func SaveDistro(path string, d *Distro) error {
	data, err := yaml.Marshal(d)
	if err != nil {
		return fmt.Errorf("marshal distro: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadCatalog reads all .yaml files from a directory
func LoadCatalog(dir string) (*Catalog, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read catalog dir %s: %w", dir, err)
	}
	c := &Catalog{}
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		d, err := LoadDistro(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", e.Name(), err)
		}
		c.Distros = append(c.Distros, d)
	}
	return c, nil
}

// ErrNoWindowsEntry is returned when the on-disk catalog has no Windows PE entry
// that could be localised.
var ErrNoWindowsEntry = fmt.Errorf("catalog 中不存在 Windows PE 条目（windows.yaml），无法设为本地源")

// SetWindowsLocal rewrites the on-disk Windows PE catalog entry so every enabled
// wimboot version boots from the given local wimboot URL and Windows base URL.
// It persists the change and returns the full reloaded catalog so the caller can
// swap it into the running Manager.
func SetWindowsLocal(catalogDir, wimbootURL, winBase string) (*Catalog, error) {
	entries, err := os.ReadDir(catalogDir)
	if err != nil {
		return nil, fmt.Errorf("read catalog dir %s: %w", catalogDir, err)
	}
	target := ""
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".yaml") && !strings.HasSuffix(e.Name(), ".yml")) {
			continue
		}
		p := filepath.Join(catalogDir, e.Name())
		d, err := LoadDistro(p)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(d.Name), "windows") {
			target = p
			break
		}
	}
	if target == "" {
		return nil, ErrNoWindowsEntry
	}

	d, err := LoadDistro(target)
	if err != nil {
		return nil, err
	}
	changed := false
	for _, v := range d.Versions {
		if v.BootType != BootWimboot || !v.Enabled {
			continue
		}
		v.Local = &FileRef{Kernel: wimbootURL, Initrd: winBase}
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("catalog Windows 条目 %s 中没有启用的 wimboot 版本", filepath.Base(target))
	}
	if err := SaveDistro(target, d); err != nil {
		return nil, err
	}
	return LoadCatalog(catalogDir)
}
