package configgen

import (
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/models"
)

func strPtr(s string) *string { return &s }

func TestGeneratePXELinux(t *testing.T) {
	entries := []models.MenuEntry{
		{Label: "ubuntu", Type: "direct", Kernel: strPtr("/ubuntu/vmlinuz"), Initrd: strPtr("/ubuntu/initrd.img"), Cmdline: strPtr("root=/dev/nfs")},
		{Label: "local", Type: "local"},
	}

	result := generatePXELinux(entries, "192.168.1.1", "")

	if !strings.Contains(result, "LABEL ubuntu") {
		t.Error("expected LABEL ubuntu")
	}
	if !strings.Contains(result, "KERNEL http://192.168.1.1/boot/ubuntu/vmlinuz") {
		t.Error("expected KERNEL with HTTP URL")
	}
	if !strings.Contains(result, "INITRD http://192.168.1.1/boot/ubuntu/initrd.img") {
		t.Error("expected INITRD with HTTP URL")
	}
	if !strings.Contains(result, "APPEND root=/dev/nfs") {
		t.Error("expected APPEND with cmdline")
	}
	if !strings.Contains(result, "LOCALBOOT 0") {
		t.Error("expected LOCALBOOT 0")
	}
}

func TestGenerateGRUB2(t *testing.T) {
	entries := []models.MenuEntry{
		{Label: "ubuntu", Type: "direct", Kernel: strPtr("/ubuntu/vmlinuz"), Initrd: strPtr("/ubuntu/initrd.img"), Cmdline: strPtr("root=/dev/nfs")},
		{Label: "local", Type: "local"},
	}

	result := generateGRUB2(entries, "192.168.1.1", "")

	if !strings.Contains(result, `menuentry "ubuntu"`) {
		t.Error("expected menuentry ubuntu")
	}
	if !strings.Contains(result, "linux http://192.168.1.1/boot/ubuntu/vmlinuz") {
		t.Error("expected linux with HTTP URL")
	}
	if !strings.Contains(result, "initrd http://192.168.1.1/boot/ubuntu/initrd.img") {
		t.Error("expected initrd with HTTP URL")
	}
	if !strings.Contains(result, "root=/dev/nfs") {
		t.Error("expected cmdline in linux line")
	}
	if !strings.Contains(result, "exit") {
		t.Error("expected exit for local entry")
	}
}

func TestEmptyEntries(t *testing.T) {
	if got := generatePXELinux(nil, "", ""); got != "" {
		t.Error("expected empty string for nil entries")
	}
	if got := generateGRUB2(nil, "", ""); got != "" {
		t.Error("expected empty string for nil entries")
	}
	if got := generatePXELinux([]models.MenuEntry{}, "", ""); got != "" {
		t.Error("expected empty string for empty entries")
	}
}

func TestVariableReplacement(t *testing.T) {
	entries := []models.MenuEntry{
		{Label: "test", Type: "direct", Kernel: strPtr("vmlinuz"), Cmdline: strPtr("server={{.URL}} mac={{.MAC}} ns={{.NextServer}}")},
	}

	result := generatePXELinux(entries, "192.168.1.1:8080", "aa:bb:cc:dd:ee:ff")

	if !strings.Contains(result, "server=http://192.168.1.1:8080") {
		t.Error("expected {{.URL}} replacement")
	}
	if !strings.Contains(result, "mac=aa:bb:cc:dd:ee:ff") {
		t.Error("expected {{.MAC}} replacement")
	}
	if !strings.Contains(result, "ns=192.168.1.1") {
		t.Error("expected {{.NextServer}} with stripped port")
	}
}

func TestSkippedEntries(t *testing.T) {
	entries := []models.MenuEntry{
		{Label: "custom", Type: "custom", Script: strPtr("echo hello")},
		{Label: "san", Type: "sanboot"},
		{Label: "wds", Type: "wds"},
	}

	result := generatePXELinux(entries, "", "")
	if result != "" {
		t.Error("expected empty string for custom/sanboot/wds")
	}
}

func TestChainEntry(t *testing.T) {
	entries := []models.MenuEntry{
		{Label: "netboot", Type: "chain", URL: strPtr("http://boot.xyz/menu.ipxe")},
	}

	result := generatePXELinux(entries, "192.168.1.1", "")
	if !strings.Contains(result, "KERNEL http://boot.xyz/menu.ipxe") {
		t.Error("expected KERNEL with chain URL")
	}

	result2 := generateGRUB2(entries, "192.168.1.1", "")
	if !strings.Contains(result2, "chainloader http://boot.xyz/menu.ipxe") {
		t.Error("expected chainloader")
	}
}
