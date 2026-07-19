package configgen

import (
	"fmt"
	"strings"

	"github.com/pxelab/pxelab/internal/models"
)

func generatePXELinux(entries []models.MenuEntry, serverAddr, mac string) string {
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

			fmt.Fprintf(&b, "LABEL %s\n", label)
			fmt.Fprintf(&b, "  KERNEL %s\n", kernel)
			if initrd != "" {
				fmt.Fprintf(&b, "  INITRD %s\n", initrd)
			}
			if cmdline != "" {
				fmt.Fprintf(&b, "  APPEND %s\n", cmdline)
			}
			b.WriteString("\n")

		case "local":
			fmt.Fprintf(&b, "LABEL %s\n", label)
			fmt.Fprintf(&b, "  LOCALBOOT 0\n\n")

		case "chain":
			url := applyVars(ptrStr(e.URL), serverAddr, mac)
			if url != "" {
				fmt.Fprintf(&b, "LABEL %s\n", label)
				fmt.Fprintf(&b, "  KERNEL %s\n\n", url)
			}
		}
	}

	return b.String()
}
