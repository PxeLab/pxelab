package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Log.Level != "info" {
		t.Errorf("expected log level 'info', got %s", cfg.Log.Level)
	}
	if cfg.Store.DSN == "" {
		t.Error("expected non-empty DSN")
	}
	if cfg.Boot.RootDir == "" {
		t.Error("expected non-empty RootDir")
	}
}

func TestDefaultDataDir(t *testing.T) {
	dir := DefaultDataDir()
	if dir == "" {
		t.Fatal("expected non-empty data dir")
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid DHCP modes", func(t *testing.T) {
		modes := []string{"full", "proxy", "off", ""}
		for _, mode := range modes {
			cfg := &Config{
				Interfaces: []InterfaceConfig{
					{Name: "eth0", Subnets: []SubnetConfig{
						{CIDR: "192.168.1.0/24", DHCP: mode},
					}},
				},
			}
			if err := cfg.Validate(); err != nil {
				t.Errorf("expected no error for mode %q, got %v", mode, err)
			}
		}
	})

	t.Run("invalid DHCP mode", func(t *testing.T) {
		cfg := &Config{
			Interfaces: []InterfaceConfig{
				{Name: "eth0", Subnets: []SubnetConfig{
					{CIDR: "192.168.1.0/24", DHCP: "invalid"},
				}},
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid DHCP mode")
		}
	})

	t.Run("empty interfaces", func(t *testing.T) {
		cfg := &Config{}
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected no error for empty interfaces, got %v", err)
		}
	})
}
