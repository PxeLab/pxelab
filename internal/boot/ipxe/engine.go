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
	BootNetboot BootType = "netboot"
	BootCustom  BootType = "custom"
)

type TemplateData struct {
	NextServer string
	BootFile   string
	MAC        string
	IP         string
	Hostname   string
	URL        string
	WIM        string
	Menu       *MenuData
	OS         *OSData

	// Netboot fields
	KernelURL string
	InitrdURL string

	// SAN boot options (used by standalone sanboot template)
	SANAction     string
	SANNoDescribe bool
	SANDrive      string
	SANKeepSAN    bool
}

type MenuData struct {
	Title     string
	Entries   []MenuEntryData
	Timeout   int // seconds
	Default   int
	TimeoutDS int // deciseconds for choose --timeout
}

type MenuEntryData struct {
	Label   string
	Type    BootType
	Kernel  string
	Initrd  string
	Cmdline string
	URL     string
	WIM     string
	Script  string

	// SAN boot options
	SANAction     string // "boot" | "hook" | "zap" | "unhook"
	SANNoDescribe bool   // --no-describe flag
	SANDrive      string // --drive flag, e.g. "0x80"
	SANKeepSAN    bool   // set keep-san 1
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
