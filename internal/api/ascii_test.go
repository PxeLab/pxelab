package api

import "testing"

func TestIsPrintableASCII(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"english letters", "Default Boot Configuration", true},
		{"letters digits symbols", "Ubuntu 22.04 LTS (x86_64)", true},
		{"leading/trailing spaces", "  BootOS  ", true},
		{"boundary space", " ", true},
		{"boundary tilde", "~", true},
		{"empty", "", true},
		{"chinese", "默认引导配置", false},
		{"mixed ascii and chinese", "BootOS 默认", false},
		{"newline", "BootOS\n", false},
		{"tab", "\t", false},
		{"emoji", "BootOS🚀", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPrintableASCII(tt.in); got != tt.want {
				t.Errorf("isPrintableASCII(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
