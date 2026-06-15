package pxelinux

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	input := `default ubuntu
timeout 100

menu title PXE Boot Menu

label ubuntu
  menu label Ubuntu 24.04
  kernel /ubuntu/vmlinuz
  append initrd=/ubuntu/initrd.img root=/dev/nfs netboot=nfs

label local
  menu label Boot from Local Disk
  local
`
	ast, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if ast.Default != "ubuntu" {
		t.Fatalf("expected default=ubuntu, got %s", ast.Default)
	}
	if len(ast.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(ast.Labels))
	}
	if ast.Labels[0].Kernel != "/ubuntu/vmlinuz" {
		t.Fatalf("expected kernel=/ubuntu/vmlinuz, got %s", ast.Labels[0].Kernel)
	}

	script := Generate(ast, "192.168.1.10")
	if !strings.Contains(script, "http://192.168.1.10/boot/ubuntu/vmlinuz") {
		t.Fatal("generated script should have HTTP URL")
	}
}
