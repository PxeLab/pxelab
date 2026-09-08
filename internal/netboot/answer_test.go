package netboot

import (
	"strings"
	"testing"

	"github.com/pxelab/pxelab/internal/models"
)

func TestAnswerDataFromHostWindowsFields(t *testing.T) {
	host := &models.Host{Name: "wl-infra-01", IP: "10.0.0.20", MAC: "aa:bb:cc:dd:ee:ff"}
	d := AnswerDataFromHost(host, "amd64")

	if d.ComputerName != "wl-infra-01" {
		t.Errorf("ComputerName = %q, want host name", d.ComputerName)
	}
	if d.HostName != "wl-infra-01" {
		t.Errorf("HostName = %q, want host name", d.HostName)
	}
	// 敏感字段无运行时来源，必须保持空（交给安装器提示），杜绝出现主机名上屏这种错误默认值
	if d.ProductKey != "" || d.AdminPassword != "" {
		t.Errorf("ProductKey/AdminPassword must be empty, got %q / %q", d.ProductKey, d.AdminPassword)
	}
	if d.Arch != "amd64" || d.KeyboardLayout != "us" || d.Disk != "/dev/sda" {
		t.Errorf("unexpected defaults: arch=%q kb=%q disk=%q", d.Arch, d.KeyboardLayout, d.Disk)
	}
}

func TestRenderAnswerTemplateAutounattendVars(t *testing.T) {
	// 模拟 autounattend 预设模板中与主机名有关的三个占位符
	content := `<ComputerName>{{.ComputerName}}</ComputerName>
<ProductKey>{{.ProductKey}}</ProductKey>
<Value>{{.AdminPassword}}</Value>`
	d := AnswerDataFromHost(&models.Host{Name: "wl-infra-01"}, "amd64")

	out, err := RenderAnswerTemplate(content, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<ComputerName>wl-infra-01</ComputerName>",
		"<ProductKey></ProductKey>",
		"<Value></Value>",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in rendered output, got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "{{.HostName") {
		t.Errorf("template must not reference removed {{.HostName}} in ProductKey/AdminPassword slots:\n%s", out)
	}
}
