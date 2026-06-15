package dhcp

import (
	"net"
	"testing"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

func TestIsPXEClient(t *testing.T) {
	t.Run("nil packet", func(t *testing.T) {
		if IsPXEClient(nil) {
			t.Error("expected false for nil packet")
		}
	})

	t.Run("PXEClient", func(t *testing.T) {
		mac, _ := net.ParseMAC("00:11:22:33:44:55")
		pkt, err := dhcpv4.NewDiscovery(mac, dhcpv4.WithOption(dhcpv4.OptClassIdentifier("PXEClient")))
		if err != nil {
			t.Fatal(err)
		}
		if !IsPXEClient(pkt) {
			t.Error("expected true for PXEClient")
		}
	})

	t.Run("non-PXE client", func(t *testing.T) {
		mac, _ := net.ParseMAC("00:11:22:33:44:55")
		pkt, err := dhcpv4.NewDiscovery(mac)
		if err != nil {
			t.Fatal(err)
		}
		if IsPXEClient(pkt) {
			t.Error("expected false for non-PXE client")
		}
	})
}

func TestDetectClientArch(t *testing.T) {
	t.Run("nil packet", func(t *testing.T) {
		_, ok := DetectClientArch(nil)
		if ok {
			t.Error("expected false for nil packet")
		}
	})

	t.Run("no arch option", func(t *testing.T) {
		mac, _ := net.ParseMAC("00:11:22:33:44:55")
		pkt, err := dhcpv4.NewDiscovery(mac)
		if err != nil {
			t.Fatal(err)
		}
		_, ok := DetectClientArch(pkt)
		if ok {
			t.Error("expected false for no arch option")
		}
	})
}

func TestBuildSubOption(t *testing.T) {
	data := []byte{0x01, 0x02, 0x03}
	result := buildSubOption(8, data)
	if len(result) != 5 {
		t.Fatalf("expected 5 bytes, got %d", len(result))
	}
	if result[0] != 8 {
		t.Errorf("expected sub-opt 8, got %d", result[0])
	}
	if result[1] != 3 {
		t.Errorf("expected length 3, got %d", result[1])
	}
	if result[2] != 0x01 || result[3] != 0x02 || result[4] != 0x03 {
		t.Error("expected data bytes 0x01, 0x02, 0x03")
	}
}
