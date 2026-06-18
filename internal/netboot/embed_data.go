package netboot

// DefaultCatalog returns a minimal built-in catalog for first-run usage.
// Full catalog is populated via `pxego netboot sync`.
func DefaultCatalog() *Catalog {
	return &Catalog{
		Distros: []*Distro{
			{
				Name:      "Ubuntu",
				Enabled:   true,
				MenuGroup: GroupLinux,
				Mirror:    "http://archive.ubuntu.com",
				Versions:  []*Version{},
			},
			{
				Name:      "Debian",
				Enabled:   true,
				MenuGroup: GroupLinux,
				Mirror:    "http://deb.debian.org",
				Versions:  []*Version{},
			},
			{
				Name:      "Clonezilla",
				Enabled:   true,
				MenuGroup: GroupTools,
				Versions:  []*Version{},
			},
			{
				Name:      "SystemRescue",
				Enabled:   true,
				MenuGroup: GroupTools,
				Versions:  []*Version{},
			},
			{
				Name:      "MemTest86",
				Enabled:   true,
				MenuGroup: GroupTools,
				Versions:  []*Version{},
			},
			{
				Name:      "GParted",
				Enabled:   true,
				MenuGroup: GroupTools,
				Versions:  []*Version{},
			},
		},
	}
}
