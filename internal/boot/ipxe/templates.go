package ipxe

var builtinTemplates = map[string]string{
	"menu": `#!ipxe
set menu-timeout {{.Menu.Timeout}}
set menu-default {{.Menu.Default}}

:menu
menu {{.Menu.Title}}
{{range $i, $entry := .Menu.Entries}}
item{{if eq $i $.Menu.Default}} --default{{end}} --key {{$i}} {{$i}} {{$entry.Label}}
{{end}}
choose --timeout ${menu-timeout} --default ${menu-default} selected || goto shell
goto ${selected}

:shell
shell
goto menu

{{range $i, $entry := .Menu.Entries}}
:{{$i}}
{{if eq $entry.Type "direct"}}
kernel {{$entry.Kernel}} {{$entry.Cmdline}}
initrd {{$entry.Initrd}}
boot
{{else if eq $entry.Type "local"}}
exit
{{else if eq $entry.Type "chain"}}
chain {{$entry.URL}}
{{else if eq $entry.Type "sanboot"}}
sanboot {{$entry.URL}}
{{else if eq $entry.Type "wds"}}
set wds-server {{$.NextServer}}
kernel wdsmgfw.efi
initrd bootmgr.exe
initrd boot.sdi
initrd {{$entry.WIM}}
boot
{{else if eq $entry.Type "netboot"}}
chain {{$.URL}}/netboot/menu.ipxe
{{end}}
{{end}}
`,

	"direct": `#!ipxe
kernel {{.OS.Kernel}} {{.OS.Cmdline}}
initrd {{.OS.Initrd}}
boot
`,

	"local": `#!ipxe
exit
`,

	"chain": `#!ipxe
chain {{.URL}}
`,

	"sanboot": `#!ipxe
sanboot {{.URL}}
`,

	"netboot": `#!ipxe
chain {{.URL}}/netboot/menu.ipxe
`,
}
