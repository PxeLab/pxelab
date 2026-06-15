package dns

import (
	"testing"

	"github.com/pxego/pxego/internal/eventbus"
)

func TestNewHandler(t *testing.T) {
	bus := eventbus.New()
	h := NewHandler("8.8.8.8:53", bus)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.upstream != "8.8.8.8:53" {
		t.Errorf("expected upstream '8.8.8.8:53', got %s", h.upstream)
	}
}

func TestNewHandlerNoUpstream(t *testing.T) {
	bus := eventbus.New()
	h := NewHandler("", bus)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.upstream != "" {
		t.Errorf("expected empty upstream, got %s", h.upstream)
	}
}
