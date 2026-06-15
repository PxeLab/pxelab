package ipxe

import (
	"strings"
	"testing"
)

func TestEngineRenderDirect(t *testing.T) {
	e := New()
	data := TemplateData{
		NextServer: "192.168.1.10",
		BootFile:   "ipxe.efi",
		OS: &OSData{
			Kernel:  "http://192.168.1.10/boot/vmlinuz",
			Initrd:  "http://192.168.1.10/boot/initrd.img",
			Cmdline: "console=tty0",
		},
	}

	out, err := e.Render("direct", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#!ipxe") {
		t.Error("expected #!ipxe header")
	}
	if !strings.Contains(out, "kernel http://192.168.1.10/boot/vmlinuz") {
		t.Error("expected kernel URL")
	}
	if !strings.Contains(out, "initrd http://192.168.1.10/boot/initrd.img") {
		t.Error("expected initrd URL")
	}
	if !strings.Contains(out, "boot") {
		t.Error("expected boot command")
	}
}

func TestEngineRenderLocal(t *testing.T) {
	e := New()
	out, err := e.Render("local", TemplateData{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "exit") {
		t.Error("expected exit command")
	}
}

func TestEngineRenderChain(t *testing.T) {
	e := New()
	data := TemplateData{
		URL: "http://bootserver/boot.ipxe",
	}
	out, err := e.Render("chain", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "chain http://bootserver/boot.ipxe") {
		t.Error("expected chain URL")
	}
}

func TestEngineRenderUnknown(t *testing.T) {
	e := New()
	out, err := e.Render("nonexistent", TemplateData{})
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Error("expected empty output for unknown template")
	}
}
