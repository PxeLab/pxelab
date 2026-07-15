package menus

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed *
var embeddedFS embed.FS

// ExtractMenus extracts embedded menu files to targetDir.
// Only writes files that do not already exist — preserves user modifications.
func ExtractMenus(targetDir string) error {
	return fs.WalkDir(embeddedFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(targetDir, path), 0755)
		}
		targetPath := filepath.Join(targetDir, path)
		if _, err := os.Stat(targetPath); err == nil {
			return nil // don't overwrite existing files
		}
		data, err := fs.ReadFile(embeddedFS, path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0644)
	})
}

// Open returns the embedded filesystem for serving via HTTP.
func Open() fs.FS {
	return embeddedFS
}
