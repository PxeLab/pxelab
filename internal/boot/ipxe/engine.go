package ipxe

import (
	"bytes"
	"text/template"
)

type BootType string

const (
	BootMenu    BootType = "menu"
	BootDirect  BootType = "direct"
	BootLocal   BootType = "local"
	BootChain   BootType = "chain"
	BootSANBoot BootType = "sanboot"
	BootWDS     BootType = "wds"
)

type TemplateData struct {
	NextServer string
	BootFile   string
	MAC        string
	IP         string
	Hostname   string
	Menu       *MenuData
	OS         *OSData
}

type MenuData struct {
	Title   string
	Entries []MenuEntryData
	Timeout int
	Default int
}

type MenuEntryData struct {
	Label   string
	Type    BootType
	Kernel  string
	Initrd  string
	Cmdline string
	URL     string
	WIM     string
}

type OSData struct {
	Kernel  string
	Initrd  string
	Cmdline string
}

type Engine struct {
	templates map[string]*template.Template
}

func New() *Engine {
	e := &Engine{templates: make(map[string]*template.Template)}
	for name, text := range builtinTemplates {
		e.templates[name] = template.Must(template.New(name).Parse(text))
	}
	return e
}

func (e *Engine) Render(name string, data TemplateData) (string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		return "", nil
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
