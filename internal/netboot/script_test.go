package netboot

import (
	"strings"
	"testing"
)

func TestGenerateNetbootScript(t *testing.T) {
	cat := &Catalog{
		Distros: []*Distro{
			{
				Name:      "Ubuntu",
				Enabled:   true,
				MenuGroup: "linux",
				Versions: []*Version{{
					Codename: "noble",
					Name:     "24.04 LTS",
					Arch:     "amd64",
					Enabled:  true,
					Local: &FileRef{
						Kernel: "ubuntu/noble/vmlinuz",
						Initrd: "ubuntu/noble/initrd",
					},
				}},
			},
			{
				Name:      "FreeBSD",
				Enabled:   true,
				MenuGroup: "bsd",
				Versions: []*Version{{
					Codename: "14.2",
					Name:     "14.2-RELEASE",
					Arch:     "amd64",
					Enabled:  true,
					Remote: &FileRef{
						Kernel: "http://example.com/freebsd/kernel",
						Initrd: "http://example.com/freebsd/initrd",
					},
				}},
			},
		},
	}
	script := GenerateNetbootScript(cat, "192.168.1.10")
	if !strings.Contains(script, "#!ipxe") {
		t.Fatal("expected #!ipxe header")
	}
	if !strings.Contains(script, "Ubuntu") {
		t.Fatal("expected Ubuntu menu entry")
	}
	if !strings.Contains(script, "FreeBSD") {
		t.Fatal("expected FreeBSD menu entry")
	}
	if !strings.Contains(script, "192.168.1.10") {
		t.Fatal("expected server IP in script")
	}
}

func TestGenerateDistroScript(t *testing.T) {
	d := &Distro{
		Name:      "Ubuntu",
		Enabled:   true,
		MenuGroup: "linux",
		Mirror:    "http://archive.ubuntu.com",
		Versions: []*Version{
			{
				Codename: "noble",
				Name:     "24.04 LTS",
				Arch:     "amd64",
				Enabled:  true,
				Remote: &FileRef{
					Kernel: "http://archive.ubuntu.com/ubuntu/dists/noble/linux",
					Initrd: "http://archive.ubuntu.com/ubuntu/dists/noble/initrd.gz",
				},
			},
			{
				Codename: "jammy",
				Name:     "22.04 LTS",
				Arch:     "amd64",
				Enabled:  false,
			},
		},
	}
	script := GenerateDistroScript(d, "192.168.1.10", "/boot/netboot")
	if !strings.Contains(script, "noble") {
		t.Fatal("expected noble version entry")
	}
	if strings.Contains(script, "jammy") {
		t.Fatal("did not expect disabled jammy entry")
	}
	if !strings.Contains(script, ":boot_ubuntu_noble") {
		t.Fatal("expected boot label for noble")
	}
}

func TestGenerateBootLineLocalPreferred(t *testing.T) {
	v := &Version{
		Codename: "noble",
		Arch:     "amd64",
		Enabled:  true,
		Local: &FileRef{
			Kernel: "ubuntu/24.04/vmlinuz",
			Initrd: "ubuntu/24.04/initrd",
		},
		Remote: &FileRef{
			Kernel: "http://example.com/linux",
			Initrd: "http://example.com/initrd.gz",
		},
	}
	line := GenerateBootLine(v, "192.168.1.10", "/boot/netboot", "")
	if !strings.Contains(line, "192.168.1.10/boot/netboot/ubuntu/24.04/vmlinuz") {
		t.Fatalf("expected local kernel path, got: %s", line)
	}
}

func TestGenerateBootLineRemoteFallback(t *testing.T) {
	v := &Version{
		Codename: "noble",
		Arch:     "amd64",
		Enabled:  true,
		Remote: &FileRef{
			Kernel: "http://example.com/linux",
			Initrd: "http://example.com/initrd.gz",
		},
	}
	line := GenerateBootLine(v, "192.168.1.10", "/boot/netboot", "")
	if !strings.Contains(line, "http://example.com/linux") {
		t.Fatalf("expected remote kernel URL, got: %s", line)
	}
}
