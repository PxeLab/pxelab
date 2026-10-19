package logbus

import "testing"

// TestSanitizeLogFileName 服务日志文件名必须是安全片段：小写、无路径分隔符
// （DHCP 的服务名形如 "dhcp/以太网"，直接拼路径会被当成子目录）。
func TestSanitizeLogFileName(t *testing.T) {
	cases := map[string]string{
		"HTTP":      "http",
		"SMB":       "smb",
		"smb":       "smb",
		"dhcp/以太网":  "dhcp-以太网",
		`dhcp\eth0`: "dhcp-eth0",
		"a:b*c?d":   "a-b-c-d",
		"  TFTP  ":  "tftp",
	}
	for in, want := range cases {
		if got := sanitizeLogFileName(in); got != want {
			t.Errorf("sanitizeLogFileName(%q) = %q, want %q", in, got, want)
		}
	}
}
