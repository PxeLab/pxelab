package httpd

import (
	"strings"
	"testing"
	"text/template"
)

// TestAutoexecTemplate tests the autoexec template using a mini-render similar to the handler.
func TestAutoexecTemplate(t *testing.T) {
	tplData := struct{ Server string }{Server: "192.168.1.10:8080"}
	var buf strings.Builder
	tmpl, err := template.New("autoexec.ipxe").ParseFS(autoexecIPXEFS, "autoexec.ipxe")
	if err != nil {
		t.Fatal("ParseFS:", err)
	}
	if err := tmpl.Execute(&buf, tplData); err != nil {
		t.Fatal("Execute:", err)
	}
	result := buf.String()

	if !strings.Contains(result, "#!ipxe") {
		t.Error("Missing #!ipxe header")
	}
	if !strings.Contains(result, ":fs_failsafe") {
		t.Error("Missing :fs_failsafe label")
	}
	if !strings.Contains(result, ":fs_retry") {
		t.Error("Missing :fs_retry label")
	}
	if !strings.Contains(result, ":fs_localboot") {
		t.Error("Missing :fs_localboot label")
	}
	if !strings.Contains(result, "chain http://192.168.1.10:8080/boot/ipxe/menu?mac=${net0/mac}") {
		t.Error("Missing chain to menu endpoint")
	}
	if strings.Contains(result, "choose --timeout") {
		t.Error("autoexec should not contain choose --timeout (no menu in autoexec)")
	}
}
