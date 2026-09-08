package configgen

import (
	"fmt"
	"strings"

	"github.com/pxelab/pxelab/internal/models"
)

func generateGRUB2(entries []models.MenuEntry, serverAddr, mac string) string {
	if len(entries) == 0 {
		return ""
	}

	var b strings.Builder

	for _, e := range entries {
		label := e.Label
		if label == "" {
			label = "default"
		}

		switch e.Type {
		case "direct":
			kernel := bootURL(serverAddr, applyVars(ptrStr(e.Kernel), serverAddr, mac))
			initrd := bootURL(serverAddr, applyVars(ptrStr(e.Initrd), serverAddr, mac))
			cmdline := applyVars(ptrStr(e.Cmdline), serverAddr, mac)

			fmt.Fprintf(&b, "menuentry \"%s\" {\n", label)
			if kernel != "" {
				fmt.Fprintf(&b, "  linux %s %s\n", kernel, cmdline)
			}
			if initrd != "" {
				fmt.Fprintf(&b, "  initrd %s\n", initrd)
			}
			b.WriteString("}\n\n")

		case "local":
			fmt.Fprintf(&b, "menuentry \"%s\" {\n", label)
			fmt.Fprintf(&b, "  exit\n")
			b.WriteString("}\n\n")

		case "chain":
			url := applyVars(ptrStr(e.URL), serverAddr, mac)
			if url != "" {
				fmt.Fprintf(&b, "menuentry \"%s\" {\n", label)
				fmt.Fprintf(&b, "  chainloader %s\n", url)
				b.WriteString("}\n\n")
			}

		default:
			// wds/sanboot/custom 依赖 iPXE 脚本，GRUB2 无法执行，输出注释提示
			fmt.Fprintf(&b, "# [grub2] \"%s\" 的类型 %s 需要 iPXE NBP，已跳过\n\n", label, e.Type)
		}
	}

	return b.String()
}
