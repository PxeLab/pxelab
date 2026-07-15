package dns

import (
	"testing"

	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
	"github.com/pxelab/pxelab/internal/store"
)

func TestParseUpstreams(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"8.8.8.8:53", []string{"8.8.8.8:53"}},
		{"8.8.8.8:53 1.1.1.1:53", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"8.8.8.8:53,1.1.1.1:53", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"8.8.8.8:53, 1.1.1.1:53", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"8.8.8.8", []string{"8.8.8.8:53"}},
		{"8.8.8.8 1.1.1.1", []string{"8.8.8.8:53", "1.1.1.1:53"}},
		{"dns.example.com", []string{"dns.example.com:53"}},
		{"dns.example.com:5353", []string{"dns.example.com:5353"}},
		{"2001:4860:4860::8888", []string{"[2001:4860:4860::8888]:53"}},
		{"", nil},
	}
	for _, tc := range tests {
		got := parseUpstreams(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("parseUpstreams(%q) = %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseUpstreams(%q)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestNewHandler(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	cfg := &config.Config{DNS: config.DNSConfig{Upstream: "8.8.8.8:53"}}
	h := NewHandler(cfg, st, bus)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if len(h.upstreams) != 1 || h.upstreams[0] != "8.8.8.8:53" {
		t.Errorf("expected upstreams ['8.8.8.8:53'], got %v", h.upstreams)
	}
}

func TestNewHandlerNoUpstream(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	cfg := &config.Config{}
	h := NewHandler(cfg, st, bus)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if len(h.upstreams) != 0 {
		t.Errorf("expected empty upstreams, got %v", h.upstreams)
	}
}

func TestNewHandlerMultiUpstream(t *testing.T) {
	bus := eventbus.New()
	st := store.NewMemory()
	cfg := &config.Config{DNS: config.DNSConfig{Upstream: "8.8.8.8:53 1.1.1.1:53"}}
	h := NewHandler(cfg, st, bus)
	if len(h.upstreams) != 2 {
		t.Errorf("expected 2 upstreams, got %d: %v", len(h.upstreams), h.upstreams)
	}
}
