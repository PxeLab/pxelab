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

func TestEngineRenderSANBoot(t *testing.T) {
	e := New()
	data := TemplateData{
		URL: "iscsi:192.168.1.100::::iqn.2024-01:disk",
	}
	out, err := e.Render("sanboot", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "#!ipxe") {
		t.Error("expected #!ipxe header")
	}
	if !strings.Contains(out, "sanboot iscsi:192.168.1.100") {
		t.Error("expected sanboot with URL")
	}
}

func TestEngineRenderSANHook(t *testing.T) {
	e := New()
	data := TemplateData{
		URL:        "iscsi:192.168.1.100::::iqn.2024-01:install",
		SANAction:  "hook",
		SANKeepSAN: true,
	}
	out, err := e.Render("sanboot", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "set keep-san 1") {
		t.Error("expected keep-san 1")
	}
	if !strings.Contains(out, "sanhook iscsi:192.168.1.100") {
		t.Error("expected sanhook with URL")
	}
	if strings.Contains(out, "sanboot") {
		t.Error("should not contain sanboot for hook action")
	}
}

func TestEngineRenderSANZap(t *testing.T) {
	e := New()
	data := TemplateData{
		URL:       "iscsi:192.168.1.100::::iqn.2024-01:disk",
		SANAction: "zap",
	}
	out, err := e.Render("sanboot", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sanunhook") {
		t.Error("expected sanunhook command")
	}
	if strings.Contains(out, "sanzboot") {
		t.Error("should not contain sanzboot (not a valid iPXE command)")
	}
	if strings.Contains(out, "sanboot") {
		t.Error("should not contain sanboot for zap action")
	}
}

func TestEngineRenderSANUnhook(t *testing.T) {
	e := New()
	data := TemplateData{
		SANAction: "unhook",
	}
	out, err := e.Render("sanboot", data)
	if err != nil {
		t.Fatal(err)
	}
	if out != "sanhook\n" && !strings.Contains(out, "sanhook") {
		t.Error("expected sanhook command")
	}
}

func TestEngineRenderSANBootNoDescribe(t *testing.T) {
	e := New()
	data := TemplateData{
		URL:          "iscsi:192.168.1.100::::iqn.2024-01:disk",
		SANNoDescribe: true,
	}
	out, err := e.Render("sanboot", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "--no-describe") {
		t.Error("expected --no-describe flag")
	}
	if !strings.Contains(out, "sanboot --no-describe") {
		t.Error("expected sanboot with --no-describe")
	}
}

func TestEngineRenderSANBootDrive(t *testing.T) {
	e := New()
	data := TemplateData{
		URL:      "iscsi:192.168.1.100::::iqn.2024-01:disk",
		SANDrive: "0x80",
	}
	out, err := e.Render("sanboot", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "--drive 0x80") {
		t.Errorf("expected --drive 0x80 flag, got: %s", out)
	}
}

func TestEngineRenderSANBootNoURL(t *testing.T) {
	e := New()
	data := TemplateData{
		SANDrive: "0x81",
	}
	out, err := e.Render("sanboot", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sanboot --drive 0x81") {
		t.Errorf("expected sanboot with drive but no URL, got: %s", out)
	}
}

func TestMenuTemplateSANBoot(t *testing.T) {
	e := New()
	data := TemplateData{
		URL: "http://server",
		Menu: &MenuData{
			Title:   "SAN Test",
			Default: 0,
			Entries: []MenuEntryData{
				{
					Label:   "SAN Boot",
					Type:    BootSANBoot,
					URL:     "iscsi:192.168.1.100::::iqn.2024-01:disk",
				},
				{
					Label:       "SAN Hook Install",
					Type:        BootSANBoot,
					URL:         "iscsi:192.168.1.100::::iqn.2024-01:install",
					SANAction:   "hook",
					SANKeepSAN:  true,
				},
				{
					Label:       "SAN Zap",
					Type:        BootSANBoot,
					URL:         "iscsi:192.168.1.100::::iqn.2024-01:old",
					SANAction:   "zap",
				},
				{
					Label:         "Boot from hooked SAN",
					Type:          BootSANBoot,
					SANNoDescribe: true,
				},
			},
		},
	}
	out, err := e.Render("menu", data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "sanboot iscsi:192.168.1.100::::iqn.2024-01:disk") {
		t.Error("expected basic sanboot entry")
	}
	if !strings.Contains(out, "set keep-san 1") {
		t.Error("expected keep-san 1 in hook entry")
	}
	if !strings.Contains(out, "sanhook iscsi:192.168.1.100::::iqn.2024-01:install") {
		t.Error("expected sanhook entry")
	}
	if !strings.Contains(out, "sanunhook") {
		t.Error("expected sanunhook entry")
	}
	if !strings.Contains(out, "sanboot --no-describe") {
		t.Error("expected sanboot --no-describe for empty URL entry")
	}
}
