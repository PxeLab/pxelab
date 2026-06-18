package netboot

import (
	"fmt"
	"regexp"
	"strings"
)

var reLabelChars = regexp.MustCompile(`[^a-z0-9_]+`)

// GenerateNetbootScript generates the full multi-level iPXE menu
func GenerateNetbootScript(c *Catalog, serverAddr string) string {
	var b strings.Builder
	b.WriteString("#!ipxe\n\n")
	b.WriteString(":netboot_menu\n")
	b.WriteString("menu [OS] Netboot OS Install Catalog\n\n")
	b.WriteString("item local    Boot from local disk\n")

	groups := c.Groups()
	for _, g := range groups {
		b.WriteString(fmt.Sprintf("item --gap %s\n", groupTitle(g.Name)))
		for _, d := range g.Distros {
			label := distroLabel(d)
			b.WriteString(fmt.Sprintf("item %s %s %s\n", label, "   ", d.Name))
		}
	}

	b.WriteString("item --gap\n")
	b.WriteString("item exit    Reboot\n")
	b.WriteString("choose selected || goto exit\n")
	b.WriteString("goto ${selected}\n\n")

	// Generate per-distro submenus
	for _, g := range groups {
		for _, d := range g.Distros {
			label := distroLabel(d)
			b.WriteString(fmt.Sprintf(":%s\n", label))
			b.WriteString(fmt.Sprintf("menu %s - Select Version\n", d.Name))
			for _, v := range d.Versions {
				if !v.Enabled {
					continue
				}
				versionLabel := versionLabel(d, v)
				b.WriteString(fmt.Sprintf("item %s %s %s\n", versionLabel, "   ", v.Name))
			}
			b.WriteString(fmt.Sprintf("item back_%s    <- Back\n", label))
			b.WriteString("choose selected || goto netboot_menu\n")
			b.WriteString("goto ${selected}\n\n")

			// Generate per-version boot entries
			for _, v := range d.Versions {
				if !v.Enabled {
					continue
				}
				versionLabel := versionLabel(d, v)
				b.WriteString(fmt.Sprintf(":%s\n", versionLabel))
				b.WriteString(GenerateBootLine(v, serverAddr, "/boot/netboot", d.KernelParams))
				b.WriteString("\n")
			}

			// Back label for this distro
			b.WriteString(fmt.Sprintf(":back_%s\n", label))
			b.WriteString("goto netboot_menu\n\n")
		}
	}

	b.WriteString(":local\n")
	b.WriteString("exit\n")

	b.WriteString(":exit\n")
	b.WriteString("exit 0\n")
	return b.String()
}

// GenerateDistroScript generates a menu for a single distro
func GenerateDistroScript(d *Distro, serverAddr, bootPrefix string) string {
	var b strings.Builder
	b.WriteString("#!ipxe\n\n")
	b.WriteString(fmt.Sprintf("menu %s\n\n", d.Name))
	for _, v := range d.Versions {
		if !v.Enabled {
			continue
		}
		versionLabel := versionLabel(d, v)
		b.WriteString(fmt.Sprintf("item %s %s %s\n", versionLabel, "   ", v.Name))
	}
	b.WriteString("choose selected || goto exit\n\n")

	for _, v := range d.Versions {
		if !v.Enabled {
			continue
		}
		versionLabel := versionLabel(d, v)
		b.WriteString(fmt.Sprintf(":%s\n", versionLabel))
		b.WriteString(GenerateBootLine(v, serverAddr, bootPrefix, d.KernelParams))
		b.WriteString("\n")
	}

	b.WriteString(":exit\n")
	b.WriteString("exit 0\n")
	return b.String()
}

// GenerateBootLine generates the kernel+initrd boot line for a version.
// Supports multiple boot types: kernel (default), memdisk, sanboot, memtest.
func GenerateBootLine(v *Version, serverAddr, bootPrefix, kernelParams string) string {
	var kernelURL, initrdURL string

	if v.Local != nil {
		kernelURL = fmt.Sprintf("http://%s%s/%s", serverAddr, bootPrefix, v.Local.Kernel)
		initrdURL = fmt.Sprintf("http://%s%s/%s", serverAddr, bootPrefix, v.Local.Initrd)
	} else if v.Remote != nil {
		kernelURL = v.Remote.Kernel
		initrdURL = v.Remote.Initrd
	}

	switch v.BootType {
	case BootMemtest:
		if kernelURL == "" {
			return "# No boot file configured\n"
		}
		return fmt.Sprintf("kernel %s\nboot\n", kernelURL)

	case BootWimboot:
		if kernelURL == "" || initrdURL == "" {
			return "# Windows PE requires wimboot URL (kernel) and Windows base URL (initrd)\n"
		}
		return fmt.Sprintf("kernel %s\ninitrd -n bootmgr %s/bootmgr bootmgr\ninitrd -n bootmgr.efi %s/bootmgr.efi bootmgr.efi\ninitrd -n bcd %s/boot/bcd bcd\ninitrd -n boot.sdi %s/boot/boot.sdi boot.sdi\ninitrd -n boot.wim %s/sources/boot.wim boot.wim\nboot\n",
			kernelURL, initrdURL, initrdURL, initrdURL, initrdURL, initrdURL)

	case BootMemdisk:
		if initrdURL == "" {
			return "# No boot file configured\n"
		}
		return fmt.Sprintf("kernel memdisk\ninitrd %s\nboot\n", initrdURL)

	case BootSanboot:
		url := initrdURL
		if url == "" {
			url = kernelURL
		}
		if url == "" {
			return "# No boot file configured\n"
		}
		return fmt.Sprintf("sanboot %s\n", url)

	default: // BootKernel — standard kernel+initrd+boot
		if kernelURL == "" && initrdURL == "" {
			return "# No boot files configured\n"
		}
		params := v.Cmdline
		if kernelParams != "" && params == "" {
			params = kernelParams
		}
		if params != "" {
			return fmt.Sprintf("kernel %s %s\ninitrd %s\nboot\n", kernelURL, params, initrdURL)
		}
		return fmt.Sprintf("kernel %s\ninitrd %s\nboot\n", kernelURL, initrdURL)
	}
}

func sanitizeLabel(s string) string {
	return reLabelChars.ReplaceAllString(strings.ToLower(s), "_")
}

func distroLabel(d *Distro) string {
	return fmt.Sprintf("distro_%s", sanitizeLabel(d.Name))
}

func versionLabel(d *Distro, v *Version) string {
	return fmt.Sprintf("boot_%s_%s", sanitizeLabel(d.Name), sanitizeLabel(v.Codename))
}

func groupTitle(name string) string {
	titles := map[string]string{
		"linux":       "== Linux Distributions ==",
		"linux-i386":  "== Linux Distributions (32-bit) ==",
		"linux-arm64": "== Linux Distributions (arm64) ==",
		"bsd":         "== BSD Systems ==",
		"live":        "== Live CDs ==",
		"live-arm":    "== Live CDs (arm64) ==",
		"tools":       "== System Tools ==",
		"unix":        "== Unix ==",
		"dos":         "== DOS ==",
		"windows":     "== Windows ==",
	}
	if t, ok := titles[name]; ok {
		return t
	}
	return name
}
