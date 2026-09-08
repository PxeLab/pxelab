package netboot

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/pxelab/pxelab/internal/models"
)

// AnswerData holds all variables available to answer templates.
type AnswerData struct {
	HostName       string
	HostIP         string
	HostMAC        string
	HostCIDR       string
	Gateway        string
	DNSServers     string
	Disk           string
	KeyboardLayout string
	Arch           string

	// Windows-specific
	ProductKey    string
	ComputerName  string
	JoinDomain    string
	DomainOU      string
	AdminPassword string
	TimeZone      string
}

// AnswerDataFromHost builds AnswerData from a Host model with sensible defaults.
func AnswerDataFromHost(host *models.Host, arch string) AnswerData {
	return AnswerData{
		HostName:       host.Name,
		HostIP:         host.IP,
		HostMAC:        host.MAC,
		Disk:           "/dev/sda",
		KeyboardLayout: "us",
		Arch:           arch,
		// Windows 字段：ComputerName 以主机名为默认值；ProductKey/
		// AdminPassword 等敏感值无运行时来源，保持为空（安装器会提示）。
		ComputerName: host.Name,
	}
}

// RenderAnswerTemplate renders an answer template with the given data.
func RenderAnswerTemplate(content string, data AnswerData) (string, error) {
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
	}

	tmpl, err := template.New("answer").Funcs(funcMap).Parse(content)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
