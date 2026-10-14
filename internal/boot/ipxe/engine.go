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

	// WDS/Windows wimboot 应答注入：预渲染的 initrd 行（可选，含行尾换行），
	// 由 netboot.WimbootAnswerInitrds 生成；为非空时插入 wds 引导块。
	AnswerInitrds string
	// WIMIndex > 0 时在 wimboot kernel 行追加 index=<n>（选择 boot.wim 镜像索引）。
	WIMIndex int

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
