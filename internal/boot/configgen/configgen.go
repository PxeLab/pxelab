package configgen

import (
	"fmt"
	"net"
	"strings"

	"github.com/pxelab/pxelab/internal/models"
)

type Format string

const (
	FormatPXELinux Format = "pxelinux"
	FormatGRUB2    Format = "grub2"
)

// Generate generates a native boot config from profile menu entries.
// Returns empty string if no entries are compatible with the given format.
func Generate(entries []models.MenuEntry, format Format, serverAddr, mac string) (string, error) {
	switch format {
	case FormatPXELinux:
		return generatePXELinux(entries, serverAddr, mac), nil
	case FormatGRUB2:
		return generateGRUB2(entries, serverAddr, mac), nil
	default:
		return "", fmt.Errorf("unsupported config format: %s", format)
	}
}

func applyVars(s, serverAddr, mac string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "{{.URL}}", "http://"+serverAddr)
	s = strings.ReplaceAll(s, "{{.MAC}}", mac)
	s = strings.ReplaceAll(s, "{{.NextServer}}", stripPort(serverAddr))
	return s
}

func bootURL(serverAddr, path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	path = strings.TrimPrefix(path, "/")
	return "http://" + serverAddr + "/boot/" + path
}

func stripPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
