package netboot

// MenuGroup categories matching netboot.xyz layout
const (
	GroupLinux    = "linux"
	GroupLinux32  = "linux-i386"
	GroupLinuxARM = "linux-arm64"
	GroupBSD      = "bsd"
	GroupLive     = "live"
	GroupLiveARM  = "live-arm"
	GroupTools    = "tools"
	GroupUnix     = "unix"
	GroupDOS      = "dos"
	GroupWindows  = "windows"
)

// InstallType for boot method
type InstallType string

const (
	InstallSubiquity InstallType = "subiquity"
	InstallLegacy    InstallType = "legacy"
	InstallLive      InstallType = "live"
	InstallDirect    InstallType = "direct"
	InstallBSD       InstallType = "bsd"
)

// BootType defines the iPXE boot mechanism
type BootType string

const (
	BootKernel  BootType = "kernel"   // standard kernel+initrd (default)
	BootMemdisk BootType = "memdisk"  // kernel memdisk + initrd ISO
	BootSanboot BootType = "sanboot"  // sanboot URL
	BootMemtest BootType = "memtest"  // binary loaded as kernel, no initrd
)

// FileRef holds either local paths or remote URLs for boot files
type FileRef struct {
	Kernel string `yaml:"kernel" json:"kernel"`
	Initrd string `yaml:"initrd" json:"initrd"`
}

// Distro represents an operating system distribution
type Distro struct {
	Name          string     `yaml:"name" json:"name"`
	Enabled       bool       `yaml:"enabled" json:"enabled"`
	Website       string     `yaml:"website,omitempty" json:"website,omitempty"`
	MenuGroup     string     `yaml:"menu_group" json:"menu_group"`
	Logo          string     `yaml:"logo,omitempty" json:"logo,omitempty"`
	Mirror        string     `yaml:"mirror,omitempty" json:"mirror,omitempty"`
	ArchiveMirror string     `yaml:"archive_mirror,omitempty" json:"archive_mirror,omitempty"`
	LocalBase     string     `yaml:"local_base,omitempty" json:"local_base,omitempty"`
	KernelParams  string     `yaml:"kernel_params,omitempty" json:"kernel_params,omitempty"`
	Versions      []*Version `yaml:"versions" json:"versions"`
}

// Version represents a specific release of a distro
type Version struct {
	Codename    string      `yaml:"codename" json:"codename"`
	Name        string      `yaml:"name" json:"name"`
	Arch        string      `yaml:"arch" json:"arch"`
	Enabled     bool        `yaml:"enabled" json:"enabled"`
	Local       *FileRef    `yaml:"local,omitempty" json:"local,omitempty"`
	Remote      *FileRef    `yaml:"remote,omitempty" json:"remote,omitempty"`
	Cmdline     string      `yaml:"cmdline,omitempty" json:"cmdline,omitempty"`
	BootType    BootType    `yaml:"type,omitempty" json:"type,omitempty"`
	InstallType InstallType `yaml:"install_type,omitempty" json:"install_type,omitempty"`
}

// Catalog holds all distro definitions
type Catalog struct {
	Distros []*Distro `json:"distros"`
}

// Group holds distros grouped by category
type Group struct {
	Name    string    `json:"name"`
	Distros []*Distro `json:"distros"`
}

// Groups returns distros organized by MenuGroup
func (c *Catalog) Groups() []Group {
	m := make(map[string][]*Distro)
	var order []string
	for _, d := range c.Distros {
		if !d.Enabled {
			continue
		}
		if _, ok := m[d.MenuGroup]; !ok {
			order = append(order, d.MenuGroup)
		}
		m[d.MenuGroup] = append(m[d.MenuGroup], d)
	}
	groups := make([]Group, 0, len(order))
	for _, name := range order {
		groups = append(groups, Group{Name: name, Distros: m[name]})
	}
	return groups
}
