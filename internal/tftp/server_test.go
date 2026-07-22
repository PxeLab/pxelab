package tftp

import (
	"context"
	"testing"
	"time"

	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/config"
	"github.com/pxelab/pxelab/internal/eventbus"
)

func TestNewServer(t *testing.T) {
	bus := eventbus.New()
	bootFS := boot.NewBootFileServer(t.TempDir())

	s := NewServer(config.TFTPConfig{Port: 0}, bootFS, bus)
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.Name() != "TFTP" {
		t.Errorf("expected name 'TFTP', got %s", s.Name())
	}
}

func TestServerStartStop(t *testing.T) {
	bus := eventbus.New()
	bootFS := boot.NewBootFileServer(t.TempDir())

	s := NewServer(config.TFTPConfig{Port: 0}, bootFS, bus)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start should not block and return nil
	if err := s.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// Stop should not block and return nil
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer stopCancel()
	if err := s.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
}
