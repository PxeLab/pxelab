package ipxe

var builtinTemplates = map[string]string{
	"menu": `#!ipxe
	set menu-default {{.Menu.Default}}
	:menu
	menu {{.Menu.Title}}
	{{range $i, $entry := .Menu.Entries}}
	item{{if eq $i $.Menu.Default}} --default{{end}} --key {{$i}} {{$i}} {{$entry.Label}}
	{{end}}
	{{if .Menu.Timeout}}choose --timeout {{.Menu.TimeoutDS}} --default {{.Menu.Default}} selected || goto {{.Menu.Default}}
	{{else}}choose selected || goto shell
	{{end}}
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
	{{else if eq $entry.Type "sanboot"}}{{if $entry.SANKeepSAN}}
	set keep-san 1
	{{end}}{{if eq $entry.SANAction "hook"}}sanhook {{$entry.URL}}
	{{else if eq $entry.SANAction "zap"}}sanunhook
	{{else if eq $entry.SANAction "unhook"}}sanhook
	{{else}}sanboot{{if $entry.SANNoDescribe}} --no-describe{{end}}{{if $entry.SANDrive}} --drive {{$entry.SANDrive}}{{end}}{{if $entry.URL}} {{$entry.URL}}{{end}}
	{{end}}
	{{else if eq $entry.Type "wds"}}
	kernel {{$entry.URL}}
	initrd -n bootmgr {{$entry.WIM}}/bootmgr bootmgr
	initrd -n bootmgr.efi {{$entry.WIM}}/bootmgr.efi bootmgr.efi
	initrd -n bcd {{$entry.WIM}}/boot/bcd bcd
	initrd -n boot.sdi {{$entry.WIM}}/boot/boot.sdi boot.sdi
	initrd -n boot.wim {{$entry.WIM}}/sources/boot.wim boot.wim
	boot
	{{else if eq $entry.Type "netboot"}}
	chain {{$.URL}}/netboot/menu.ipxe
	{{else if eq $entry.Type "custom"}}
	{{$entry.Script}}
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
{{- if .SANKeepSAN}}set keep-san 1
{{end -}}
{{- if eq .SANAction "hook"}}sanhook {{.URL}}
{{- else if eq .SANAction "zap"}}sanunhook
{{- else if eq .SANAction "unhook"}}sanhook
{{- else}}sanboot{{if .SANNoDescribe}} --no-describe{{end}}{{if .SANDrive}} --drive {{.SANDrive}}{{end}}{{if .URL}} {{.URL}}{{end}}
{{end -}}
	`,

	"custom": `#!ipxe
	{{.Script}}
	`,

	"netboot": `#!ipxe
		chain {{.URL}}/netboot/menu.ipxe
	`,
}
