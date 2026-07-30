package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// --- Structs matching internal/netboot/types.go (avoid import cycle concerns) ---

type FileRef struct {
	Kernel string `yaml:"kernel" json:"kernel"`
	Initrd string `yaml:"initrd" json:"initrd"`
}

type Version struct {
	Codename    string   `yaml:"codename" json:"codename"`
	Name        string   `yaml:"name" json:"name"`
	Arch        string   `yaml:"arch" json:"arch"`
	Enabled     bool     `yaml:"enabled" json:"enabled"`
	Remote      *FileRef `yaml:"remote,omitempty" json:"remote,omitempty"`
	Cmdline     string   `yaml:"cmdline,omitempty" json:"cmdline,omitempty"`
	BootType    string   `yaml:"type,omitempty" json:"type,omitempty"`
	InstallType string   `yaml:"install_type,omitempty" json:"install_type,omitempty"`
}

type Distro struct {
	Name      string     `yaml:"name" json:"name"`
	Enabled   bool       `yaml:"enabled" json:"enabled"`
	MenuGroup string     `yaml:"menu_group" json:"menu_group"`
	Mirror    string     `yaml:"mirror,omitempty" json:"mirror,omitempty"`
	Versions  []*Version `yaml:"versions" json:"versions"`
}

// --- Store JSON output types ---

type StoreItem struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Author      string   `json:"author"`
	Icon        string   `json:"icon"`
	Downloads   int      `json:"downloads"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type VersionContent struct {
	Codename string `json:"codename"`
	Name     string `json:"name"`
	Arch     string `json:"arch"`
	Type     string `json:"type"`
	Kernel   string `json:"kernel,omitempty"`
	Initrd   string `json:"initrd,omitempty"`
	Cmdline  string `json:"cmdline,omitempty"`
}

type DistroContent struct {
	Mirror   string            `json:"mirror"`
	Versions []*VersionContent `json:"versions"`
}

// --- Catalog wrapper matching existing catalog.json ---

type CatalogWrapper struct {
	Items []StoreItem `json:"items"`
	Total int         `json:"total"`
}

// --- menu_group mapping ---

type groupInfo struct {
	Icon  string
	Label string
}

var groupMap = map[string]groupInfo{
	"linux":       {"linux", "Linux Distribution"},
	"linux-i386":  {"linux", "Linux Distribution (i386)"},
	"linux-arm64": {"linux", "Linux Distribution (ARM64)"},
	"tools":       {"tools", "Utility / Tool"},
	"live":        {"live", "Live Environment"},
	"live-arm":    {"live", "Live Environment (ARM)"},
	"bsd":         {"bsd", "BSD Distribution"},
	"dos":         {"dos", "DOS Environment"},
	"unix":        {"unix", "Unix Distribution"},
	"windows":     {"windows", "Windows Environment"},
}

func getGroupInfo(group string) groupInfo {
	if gi, ok := groupMap[group]; ok {
		return gi
	}
	return groupInfo{"other", "Distribution"}
}

func mapInstallType(it string) string {
	if it == "" {
		return "direct"
	}
	return it
}

func slugFromFilename(name string) string {
	return strings.TrimSuffix(name, ".yaml")
}

func countEnabledVersions(versions []*Version) int {
	n := 0
	for _, v := range versions {
		if v.Enabled {
			n++
		}
	}
	return n
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func main() {
	embeddedDir := "internal/netboot/embedded"
	storeDir := "pxelab-store/api/v1/items/netboot_distro"
	catalogPath := "pxelab-store/api/v1/catalog.json"

	now := time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(storeDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", storeDir, err)
		os.Exit(1)
	}

	entries, err := os.ReadDir(embeddedDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readdir %s: %v\n", embeddedDir, err)
		os.Exit(1)
	}

	var catalogEntries []StoreItem

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		path := filepath.Join(embeddedDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
			continue
		}

		var d Distro
		if err := yaml.Unmarshal(data, &d); err != nil {
			fmt.Fprintf(os.Stderr, "yaml parse %s: %v\n", path, err)
			continue
		}

		if !d.Enabled {
			fmt.Printf("  skip (disabled): %s\n", entry.Name())
			continue
		}

		itemID := "netboot-" + slugFromFilename(entry.Name())
		gi := getGroupInfo(d.MenuGroup)
		enabledCount := countEnabledVersions(d.Versions)

		// Build catalog entry
		plural := "versions"
		if enabledCount == 1 {
			plural = "version"
		}
		catEntry := StoreItem{
			ID:          itemID,
			Type:        "netboot_distro",
			Name:        d.Name,
			Description: fmt.Sprintf("%s · %d %s available for boot.", gi.Label, enabledCount, plural),
			Tags:        []string{d.MenuGroup},
			Author:      "netboot.xyz / PxeLab",
			Icon:        gi.Icon,
			Downloads:   0,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		catalogEntries = append(catalogEntries, catEntry)

		// Build content
		var contentVersions []*VersionContent
		for _, v := range d.Versions {
			if !v.Enabled {
				continue
			}
			vc := &VersionContent{
				Codename: v.Codename,
				Name:     v.Name,
				Arch:     v.Arch,
				Type:     mapInstallType(v.InstallType),
			}
			if v.Remote != nil {
				vc.Kernel = v.Remote.Kernel
				vc.Initrd = v.Remote.Initrd
			}
			if v.Cmdline != "" {
				vc.Cmdline = v.Cmdline
			}
			contentVersions = append(contentVersions, vc)
		}

		content := DistroContent{
			Mirror:   d.Mirror,
			Versions: contentVersions,
		}

		// Build detail JSON
		detail := struct {
			StoreItem
			Content DistroContent `json:"content"`
		}{
			StoreItem: catEntry,
			Content:   content,
		}

		// Write detail JSON file
		detailPath := filepath.Join(storeDir, itemID+".json")
		if err := writeJSON(detailPath, detail); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", detailPath, err)
			continue
		}

		fmt.Printf("  OK: %s -> %s (%d versions)\n", entry.Name(), itemID, enabledCount)
	}

	fmt.Printf("\nTotal: %d enabled distros converted.\n", len(catalogEntries))

	// --- Update catalog.json ---
	// Read existing catalog
	var catalog CatalogWrapper
	if existing, err := os.ReadFile(catalogPath); err == nil {
		if err := json.Unmarshal(existing, &catalog); err != nil {
			fmt.Fprintf(os.Stderr, "warning: parse existing catalog: %v (will overwrite)\n", err)
			catalog = CatalogWrapper{}
		}
	}

	// Filter out any existing netboot_distro entries (to replace with fresh)
	var kept []StoreItem
	for _, item := range catalog.Items {
		if item.Type == "netboot_distro" {
			continue
		}
		kept = append(kept, item)
	}
	catalog.Items = append(kept, catalogEntries...)
	catalog.Total = len(catalog.Items)

	if err := writeJSON(catalogPath, catalog); err != nil {
		fmt.Fprintf(os.Stderr, "write catalog: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Catalog updated: %d total items (baselines + netboot_distros)\n", catalog.Total)
}
