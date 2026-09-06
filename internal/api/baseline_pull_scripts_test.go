package api

import (
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/config"
)

func TestAugmentKickstartDefault(t *testing.T) {
	out, ok := augmentBaselinePull("ks.cfg content", "kickstart", "192.168.1.10:8080", "00:11:22:33:44:55", "", config.BaselineHooksConfig{})
	if !ok {
		t.Fatal("expected injection to apply")
	}
	if !strings.Contains(out, "%post --interpreter=/bin/bash") || !strings.Contains(out, "curl -fsSL 'http://192.168.1.10:8080/api/v1/baselines/pull.sh?mac=00:11:22:33:44:55'") {
		t.Fatalf("default hook missing or URL wrong:\n%s", out)
	}
	if !strings.Contains(out, baselinePullMarker) {
		t.Fatal("marker missing")
	}
}

func TestAugmentKickstartCustomTemplate(t *testing.T) {
	hooks := config.BaselineHooksConfig{Kickstart: "%post\necho custom {{URL}}\n%end"}
	out, ok := augmentBaselinePull("content", "kickstart", "10.0.0.1:8080", "", "SN-1", hooks)
	if !ok {
		t.Fatal("expected injection to apply")
	}
	if !strings.Contains(out, "echo custom http://10.0.0.1:8080/api/v1/baselines/pull.sh?sn=SN-1") {
		t.Fatalf("custom template not rendered:\n%s", out)
	}
	if strings.Contains(out, "--interpreter=/bin/bash") {
		t.Fatal("default template leaked into output")
	}
}

func TestAugmentIdempotent(t *testing.T) {
	content := "cfg\n# PxeLab baseline pull (pxelab-baseline-pull)\n"
	out, ok := augmentBaselinePull(content, "kickstart", "10.0.0.1:8080", "00:11:22:33:44:55", "", config.BaselineHooksConfig{})
	if !ok || out != content {
		t.Fatalf("expected idempotent skip, ok=%v", ok)
	}
}

func TestAugmentNoIdentity(t *testing.T) {
	_, ok := augmentBaselinePull("content", "kickstart", "10.0.0.1:8080", "", "", config.BaselineHooksConfig{})
	if ok {
		t.Fatal("expected skip when no mac/sn available")
	}
}

func TestAugmentAutoUnattendCustomTemplateEscaped(t *testing.T) {
	content := `<settings pass="oobeSystem"><component name="Microsoft-Windows-Shell-Setup" processorArchitecture="amd64">` + `</component></settings>`
	hooks := config.BaselineHooksConfig{AutoUnattend: `powershell.exe -Command "iex '{{URL}}'"`}
	out, ok := augmentBaselinePull(content, "autounattend", "10.0.0.1:8080", "00:11:22:33:44:55", "", hooks)
	if !ok {
		t.Fatal("expected injection to apply")
	}
	if !strings.Contains(out, "FirstLogonCommands") {
		t.Fatal("FirstLogonCommands missing")
	}
	// 模板含双引号，注入后必须 XML 转义
	if !strings.Contains(out, "powershell.exe -Command &quot;iex") {
		t.Fatalf("command not XML-escaped:\n%s", out)
	}
	if !strings.Contains(out, "pull.ps1") {
		t.Fatal("ps1 URL missing")
	}
}

func TestRenderHookFallback(t *testing.T) {
	if got := renderHook("", defaultPreseedHook, "http://x"); !strings.Contains(got, "in-target sh -c") {
		t.Fatal("empty template should fall back to default")
	}
	if got := renderHook("run {{URL}}", "", "http://x"); got != "run http://x" {
		t.Fatalf("unexpected render: %s", got)
	}
}
