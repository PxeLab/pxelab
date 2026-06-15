package dhcp

import (
	"net"
	"testing"
)

func TestIPRangeContains(t *testing.T) {
	r, err := NewIPRange("192.168.1.10", "192.168.1.20")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		ip    string
		want  bool
	}{
		{"192.168.1.10", true},
		{"192.168.1.15", true},
		{"192.168.1.20", true},
		{"192.168.1.9", false},
		{"192.168.1.21", false},
		{"10.0.0.1", false},
	}

	for _, tc := range tests {
		got := r.Contains(net.ParseIP(tc.ip))
		if got != tc.want {
			t.Errorf("Contains(%s) = %v; want %v", tc.ip, got, tc.want)
		}
	}
}

func TestNewIPRangeInvalid(t *testing.T) {
	_, err := NewIPRange("invalid", "192.168.1.20")
	if err == nil {
		t.Error("expected error for invalid start IP")
	}

	_, err = NewIPRange("192.168.1.10", "invalid")
	if err == nil {
		t.Error("expected error for invalid end IP")
	}
}

func TestBytesCompare(t *testing.T) {
	a := net.ParseIP("192.168.1.10")
	b := net.ParseIP("192.168.1.20")
	c := net.ParseIP("192.168.1.10")

	if bytesCompare(a, b) != -1 {
		t.Error("expected a < b")
	}
	if bytesCompare(b, a) != 1 {
		t.Error("expected b > a")
	}
	if bytesCompare(a, c) != 0 {
		t.Error("expected a == c")
	}
}

func TestIncIP(t *testing.T) {
	ip := net.ParseIP("192.168.1.10").To4()
	incIP(ip)
	if ip.String() != "192.168.1.11" {
		t.Errorf("expected 192.168.1.11, got %s", ip)
	}

	ip = net.ParseIP("192.168.1.255").To4()
	incIP(ip)
	if ip.String() != "192.168.2.0" {
		t.Errorf("expected 192.168.2.0, got %s", ip)
	}
}
