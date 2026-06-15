package pxelinux

import (
	"fmt"
	"strings"
)

func Generate(ast *AST, nextServer string) string {
	var sb strings.Builder

	sb.WriteString("#!ipxe\n")
	if ast.Timeout > 0 {
		sb.WriteString(fmt.Sprintf("set menu-timeout %d\n", ast.Timeout/10))
	}
	if ast.Default != "" {
		sb.WriteString(fmt.Sprintf("set menu-default %s\n", ast.Default))
	}

	sb.WriteString(":menu\n")
	title := ast.MenuTitle
	if title == "" {
		title = "PXE Boot Menu"
	}
	sb.WriteString(fmt.Sprintf("menu %s\n", title))

	for _, label := range ast.Labels {
		menuLabel := label.MenuLabel
		if menuLabel == "" {
			menuLabel = label.Name
		}
		sb.WriteString(fmt.Sprintf("item %s %s\n", label.Name, menuLabel))
	}

	sb.WriteString("choose --timeout ${menu-timeout} --default ${menu-default} selected || goto shell\n")
	sb.WriteString("goto ${selected}\n\n")

	for _, label := range ast.Labels {
		sb.WriteString(fmt.Sprintf(":%s\n", label.Name))
		if label.Kernel != "" {
			kernel := rewritePath(label.Kernel, nextServer)
			sb.WriteString(fmt.Sprintf("kernel %s %s\n", kernel, strings.Join(label.Append, " ")))
			if label.Initrd != "" {
				initrd := rewritePath(label.Initrd, nextServer)
				sb.WriteString(fmt.Sprintf("initrd %s\n", initrd))
			}
			sb.WriteString("boot\n")
		} else {
			sb.WriteString("exit\n")
		}
	}

	return sb.String()
}

func rewritePath(path, server string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "tftp://") {
		return path
	}
	return fmt.Sprintf("http://%s/boot/%s", server, strings.TrimPrefix(path, "/"))
}
