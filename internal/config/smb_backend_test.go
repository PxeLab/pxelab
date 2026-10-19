package config

import "testing"

func TestNormalizeSMBBackend(t *testing.T) {
	cases := map[string]string{
		"":         SMBBackendBuiltin,
		"builtin":  SMBBackendBuiltin,
		"BUILTIN":  SMBBackendBuiltin,
		"system":   SMBBackendSystem,
		" System ": SMBBackendSystem,
		"weird":    SMBBackendBuiltin,
	}
	for in, want := range cases {
		if got := NormalizeSMBBackend(in); got != want {
			t.Errorf("NormalizeSMBBackend(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultSMBBackendIsBuiltin(t *testing.T) {
	cfg := DefaultConfig()
	if got := NormalizeSMBBackend(cfg.SMB.Backend); got != SMBBackendBuiltin {
		t.Fatalf("默认后端应为 builtin, got %q", got)
	}
}
