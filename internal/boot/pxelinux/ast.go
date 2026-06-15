package pxelinux

type AST struct {
	Defaults  Label
	Labels    []Label
	OnTimeout string
	OnError   string
	MenuTitle string
	Timeout   int
	Default   string
}

type Label struct {
	Name       string
	Kernel     string
	Append     []string
	Initrd     string
	MenuLabel  string
	MenuTitle  string
	MenuIndent int
}
