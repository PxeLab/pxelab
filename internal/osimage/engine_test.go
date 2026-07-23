package osimage

import "testing"

func TestParseTreeinfoGeneral(t *testing.T) {
	centos := `[checksums]
images/pxeboot/vmlinuz = sha256:abc

[general]
arch = x86_64
family = CentOS Linux
name = CentOS Linux 8
version = 8
`
	d, v, a := parseTreeinfoGeneral(centos)
	if d != "centos" || v != "8" || a != "amd64" {
		t.Errorf("centos: got %q %q %q", d, v, a)
	}

	rocky := `[general]
arch = aarch64
family = Rocky Linux
name = Rocky Linux 9.3
version = 9.3
`
	d, v, a = parseTreeinfoGeneral(rocky)
	if d != "rocky" || v != "9.3" || a != "arm64" {
		t.Errorf("rocky: got %q %q %q", d, v, a)
	}

	alma := "[general]\nname = AlmaLinux 9\nversion = 9\n"
	d, _, _ = parseTreeinfoGeneral(alma)
	if d != "almalinux" {
		t.Errorf("almalinux: got %q", d)
	}

	// 无 [general] 段或不认识的家族应返回空
	d, _, _ = parseTreeinfoGeneral("[checksums]\nx = y\n")
	if d != "" {
		t.Errorf("empty: got %q", d)
	}

	oe := "[general]\nfamily = openEuler\nversion = 20.03-LTS\nname = openEuler-20.03-LTS\narch = aarch64\n"
	d, v, a = parseTreeinfoGeneral(oe)
	if d != "openeuler" || v != "20.03-LTS" || a != "arm64" {
		t.Errorf("openeuler: got %q %q %q", d, v, a)
	}

	ky := "[general]\nfamily = Kylin Linux Advanced Server\nversion = V10\nname = Kylin Linux Advanced Server-V10\narch = x86_64\n"
	d, v, a = parseTreeinfoGeneral(ky)
	if d != "kylin" || v != "V10" || a != "amd64" {
		t.Errorf("kylin: got %q %q %q", d, v, a)
	}
}
