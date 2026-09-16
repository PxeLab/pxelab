package netboot

import (
	"strings"
	"testing"
)

func TestGenerateWimbootLineWinpeShell(t *testing.T) {
	line := generateWimbootLine(
		"http://10.0.0.1/netboot/menu/wimboot",
		"http://10.0.0.1/boot/isos/win11-x64",
		&BootTaskInfo{
			AnswerURL:  "http://10.0.0.1/api/v1/netboot/answer/task_1",
			AnswerType: "winpeshl",
		},
	)
	if !strings.Contains(line, "http://10.0.0.1/api/v1/netboot/answer/task_1 install.bat") {
		t.Fatalf("expected install.bat initrd from answer URL, got:\n%s", line)
	}
	if !strings.Contains(line, "http://10.0.0.1/api/v1/netboot/answer/task_1?file=winpeshl.ini winpeshl.ini") {
		t.Fatalf("expected winpeshl.ini via ?file= param, got:\n%s", line)
	}
	if strings.Count(line, "winpeshl.ini") != 2 {
		t.Fatalf("winpeshl.ini should appear in both the ?file= URL and the local name, got:\n%s", line)
	}
	if !strings.HasPrefix(line, "kernel http://10.0.0.1/netboot/menu/wimboot") {
		t.Fatalf("expected wimboot kernel line, got:\n%s", line)
	}
}

func TestGenerateWimbootLineAutounattend(t *testing.T) {
	line := generateWimbootLine(
		"http://10.0.0.1/netboot/menu/wimboot",
		"http://10.0.0.1/boot/isos/win11-x64",
		&BootTaskInfo{
			AnswerURL:  "http://10.0.0.1/api/v1/netboot/answer/task_1",
			AnswerType: "autounattend",
		},
	)
	if !strings.Contains(line, "http://10.0.0.1/api/v1/netboot/answer/task_1 autounattend.xml") {
		t.Fatalf("expected autounattend.xml initrd, got:\n%s", line)
	}
	if strings.Contains(line, "?file=") {
		t.Fatalf("autounattend must not use ?file= suffix, got:\n%s", line)
	}
	if strings.Contains(line, "winpeshl") {
		t.Fatalf("autounattend must not reference winpeshl.ini, got:\n%s", line)
	}
}

func TestGenerateBootLineWimbootLocalAbsolute(t *testing.T) {
	v := &Version{
		Codename: "win11-x64",
		Arch:     "amd64",
		Enabled:  true,
		BootType: BootWimboot,
		Local: &FileRef{
			Kernel: "http://10.0.0.10/netboot/menu/wimboot",
			Initrd: "http://10.0.0.10/boot/isos/win11-iso",
		},
	}
	line := GenerateBootLine(v, "10.0.0.10", "/boot/netboot", "", nil, false)
	if !strings.Contains(line, "kernel http://10.0.0.10/netboot/menu/wimboot") {
		t.Fatalf("expected local wimboot kernel URL, got:\n%s", line)
	}
	if !strings.Contains(line, "http://10.0.0.10/boot/isos/win11-iso/bootmgr bootmgr") {
		t.Fatalf("expected local win base bootmgr, got:\n%s", line)
	}
	if strings.Contains(line, "/boot/netboot/http") {
		t.Fatalf("absolute local URLs must not be re-prefixed, got:\n%s", line)
	}
}

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
	script := GenerateNetbootScript(cat, "192.168.1.10", "", "", "[OS] Netboot OS Install Catalog", nil, nil, false)
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
	script := GenerateDistroScript(d, "192.168.1.10", "/boot/netboot", nil, false)
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
	line := GenerateBootLine(v, "192.168.1.10", "/boot/netboot", "", nil, false)
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
	line := GenerateBootLine(v, "192.168.1.10", "/boot/netboot", "", nil, false)
	if !strings.Contains(line, "http://example.com/linux") {
		t.Fatalf("expected remote kernel URL, got: %s", line)
	}
}
