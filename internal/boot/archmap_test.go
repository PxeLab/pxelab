package boot

import (
	"testing"

	"github.com/insomniacslk/dhcp/iana"
	"github.com/pxelab/pxelab/internal/config"
)

func TestBootFileForArch_PreserveDefaults(t *testing.T) {
	// 不设置任何 ArchMap，应回退到内置默认值
	tests := []struct {
		arch iana.Arch
		want string
	}{
		{iana.INTEL_X86PC, "ipxe.pxe"},
		{iana.EFI_IA32, "ipxe32.efi"},
		{iana.EFI_X86_64, "ipxe.efi"},
		{iana.EFI_BC, "ipxe.efi"},
		{iana.EFI_ARM64, "ipxe-arm64.efi"},
		{iana.EFI_RISCV64, "ipxe-riscv64.efi"},
		{iana.Arch(255), "ipxe.pxe"}, // unknown → default
	}

	for _, tc := range tests {
		got := BootFileForArch(tc.arch)
		if got != tc.want {
			t.Errorf("BootFileForArch(%d) = %s; want %s", tc.arch, got, tc.want)
		}
	}
}

func TestBootFileForArch_WithConfig(t *testing.T) {
	old := globalArchMap
	t.Cleanup(func() { globalArchMap = old })

	InitArchMap(map[int]config.ArchEntry{
		int(iana.INTEL_X86PC): {IPXE: "custom.ipxe"},
		int(iana.EFI_ARM64):   {IPXE: "custom-arm64.efi"},
	})

	tests := []struct {
		arch iana.Arch
		want string
	}{
		{iana.INTEL_X86PC, "custom.ipxe"},
		{iana.EFI_ARM64, "custom-arm64.efi"},
		{iana.EFI_X86_64, "ipxe.efi"}, // 不在配置中 → 回退到默认
	}

	for _, tc := range tests {
		got := BootFileForArch(tc.arch)
		if got != tc.want {
			t.Errorf("BootFileForArch(%d) = %s; want %s", tc.arch, got, tc.want)
		}
	}
}

func TestNBPFilename_WithConfig(t *testing.T) {
	old := globalArchMap
	t.Cleanup(func() { globalArchMap = old })

	InitArchMap(map[int]config.ArchEntry{
		int(iana.INTEL_X86PC): {
			IPXE:     "my-ipxe.pxe",
			PXELinux: "my-pxelinux.0",
			GRUB:     "my-grub.efi",
		},
		int(iana.EFI_ARM64): {
			IPXE: "arm64-ipxe.efi",
			GRUB: "arm64-grub.efi",
		},
	})

	t.Run("ipxe", func(t *testing.T) {
		if got := NBPFilename(iana.INTEL_X86PC, "ipxe"); got != "my-ipxe.pxe" {
			t.Errorf("got %s, want my-ipxe.pxe", got)
		}
	})
	t.Run("pxelinux", func(t *testing.T) {
		if got := NBPFilename(iana.INTEL_X86PC, "pxelinux"); got != "my-pxelinux.0" {
			t.Errorf("got %s, want my-pxelinux.0", got)
		}
	})
	t.Run("grub2", func(t *testing.T) {
		if got := NBPFilename(iana.INTEL_X86PC, "grub2"); got != "my-grub.efi" {
			t.Errorf("got %s, want my-grub.efi", got)
		}
	})
	t.Run("grub2-arm64", func(t *testing.T) {
		if got := NBPFilename(iana.EFI_ARM64, "grub2"); got != "arm64-grub.efi" {
			t.Errorf("got %s, want arm64-grub.efi", got)
		}
	})
	t.Run("undionly-x86", func(t *testing.T) {
		if got := NBPFilename(iana.INTEL_X86PC, "undionly"); got != "undionly.kpxe" {
			t.Errorf("got %s, want undionly.kpxe", got)
		}
	})
	t.Run("undionly-arm64-fallback-ipxe", func(t *testing.T) {
		if got := NBPFilename(iana.EFI_ARM64, "undionly"); got != "arm64-ipxe.efi" {
			t.Errorf("got %s, want arm64-ipxe.efi", got)
		}
	})
}

func TestGetArchMap_RoundTrip(t *testing.T) {
	old := globalArchMap
	t.Cleanup(func() { globalArchMap = old })

	// 没有自定义配置时，返回内置默认值
	InitArchMap(nil)
	baseLen := len(GetArchMap())
	if baseLen == 0 {
		t.Fatal("没有配置时 GetArchMap 不应返回空")
	}

	// 设置自定义配置，应与默认值合并
	input := map[int]config.ArchEntry{
		0: {IPXE: "a.ipxe", PXELinux: "a.0"},
		5: {IPXE: "b.efi", GRUB: "b-grub.efi"},
	}
	InitArchMap(input)

	got := GetArchMap()
	// 自定义值应覆盖默认值
	for k, v := range input {
		if got[k] != v {
			t.Errorf("key %d: got %+v; want %+v", k, got[k], v)
		}
	}
	// 总数应 >= 自定义数 + 默认不重叠的
	if len(got) < len(input) {
		t.Fatalf("len = %d; want at least %d (should include defaults)", len(got), len(input))
	}

	// 确保 GetArchMap 返回的是副本，修改不会影响内部
	got[99] = config.ArchEntry{IPXE: "injected"}
	if _, ok := globalArchMap[99]; ok {
		t.Error("GetArchMap 修改泄露到内部状态")
	}
}
