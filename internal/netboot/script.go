package netboot

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/pxelab/pxelab/internal/config"
)

var reLabelChars = regexp.MustCompile(`[^a-z0-9_]+`)

// archMap maps iPXE ${arch} values to catalog arch field values
var archMap = map[string]string{
	"x86_64": "amd64",
	"x86":    "i386",
	"arm64":  "arm64",
	"armhf":  "armhf",
}

// GenerateNetbootScript generates the full multi-level iPXE menu.
// If arch is non-empty, only distros/versions matching that architecture are included.
// platform may be "efi" or "pc" (not yet used for filtering, reserved for future use).
// menuTitle overrides the default catalog menu title.
// groups provides configurable group ordering, titles, and enabled/disabled state.
// proxyHTTPS controls whether HTTPS boot URLs are proxied through the local HTTP server.
func GenerateNetbootScript(c *Catalog, serverAddr, arch, platform, menuTitle string, groups []config.CatalogGroup, task *BootTaskInfo, proxyHTTPS bool) string {
	archFilter := archMap[arch]

	// Build group config lookup maps
	groupTitles := make(map[string]string)
	groupEnabled := make(map[string]bool)
	hasGroups := len(groups) > 0
	for _, g := range groups {
		groupTitles[g.Name] = g.Title
		groupEnabled[g.Name] = g.Enabled
	}

	versionMatchesArch := func(v *Version) bool {
		if archFilter == "" {
			return true
		}
		return v.Arch == archFilter
	}

	distroHasArch := func(d *Distro) bool {
		if archFilter == "" {
			return true
		}
		for _, v := range d.Versions {
			if v.Enabled && versionMatchesArch(v) {
				return true
			}
		}
		return false
	}

	var b strings.Builder
	b.WriteString("#!ipxe\n\n")
	b.WriteString(":netboot_menu\n")
	if menuTitle == "" {
		menuTitle = "[OS] Netboot OS Install Catalog"
	}
	b.WriteString(fmt.Sprintf("menu %s\n\n", menuTitle))
	b.WriteString("item local    Boot from local disk\n")

	groupsList := c.Groups()

	// Filter and sort groups by config
	if hasGroups {
		var filtered []Group
		for _, g := range groupsList {
			if enabled, ok := groupEnabled[g.Name]; ok && !enabled {
				continue
			}
			filtered = append(filtered, g)
		}
		// Sort filtered groups by configured order
		sort.SliceStable(filtered, func(i, j int) bool {
			oi, oki := 0, false
			oj, okj := 0, false
			for _, cfg := range groups {
				if cfg.Name == filtered[i].Name {
					oi = cfg.Order
					oki = true
				}
				if cfg.Name == filtered[j].Name {
					oj = cfg.Order
					okj = true
				}
			}
			if !oki {
				return false
			}
			if !okj {
				return true
			}
			return oi < oj
		})
		groupsList = filtered
	}

	for _, g := range groupsList {
		var matching []*Distro
		for _, d := range g.Distros {
			if distroHasArch(d) {
				matching = append(matching, d)
			}
		}
		if len(matching) == 0 {
			continue
		}
		title := groupTitleFallback(g.Name, groupTitles)
		b.WriteString(fmt.Sprintf("item --gap %s\n", title))
		for _, d := range matching {
			label := distroLabel(d)
			b.WriteString(fmt.Sprintf("item %s %s %s\n", label, "   ", d.Name))
		}
	}

	b.WriteString("item --gap\n")
	b.WriteString("item exit    Reboot\n")
	b.WriteString("choose selected || goto exit\n")
	b.WriteString("goto ${selected}\n\n")

	// Generate per-distro submenus
	for _, g := range groupsList {
		for _, d := range g.Distros {
			if !distroHasArch(d) {
				continue
			}
			label := distroLabel(d)
			b.WriteString(fmt.Sprintf(":%s\n", label))
			b.WriteString(fmt.Sprintf("menu %s - Select Version\n", d.Name))
			for _, v := range d.Versions {
				if !v.Enabled || !versionMatchesArch(v) {
					continue
				}
				versionLabel := versionLabel(d, v)
				b.WriteString(fmt.Sprintf("item %s %s %s\n", versionLabel, "   ", v.Name))
			}
			b.WriteString(fmt.Sprintf("item back_%s    <- Back\n", label))
			b.WriteString("choose selected || goto netboot_menu\n")
			b.WriteString("goto ${selected}\n\n")

			// Generate per-version boot entries
			for _, v := range d.Versions {
				if !v.Enabled || !versionMatchesArch(v) {
					continue
				}
				versionLabel := versionLabel(d, v)
				b.WriteString(fmt.Sprintf(":%s\n", versionLabel))
				b.WriteString(GenerateBootLine(v, serverAddr, "/boot/netboot", d.KernelParams, task, proxyHTTPS))
				b.WriteString("\n")
			}

			// Back label for this distro
			b.WriteString(fmt.Sprintf(":back_%s\n", label))
			b.WriteString("goto netboot_menu\n\n")
		}
	}

	b.WriteString(":local\n")
	b.WriteString("exit\n")

	b.WriteString(":exit\n")
	b.WriteString("exit 0\n")
	return b.String()
}

// GenerateDistroScript generates a menu for a single distro
func GenerateDistroScript(d *Distro, serverAddr, bootPrefix string, task *BootTaskInfo, proxyHTTPS bool) string {
	var b strings.Builder
	b.WriteString("#!ipxe\n\n")
	b.WriteString(fmt.Sprintf("menu %s\n\n", d.Name))
	for _, v := range d.Versions {
		if !v.Enabled {
			continue
		}
		versionLabel := versionLabel(d, v)
		b.WriteString(fmt.Sprintf("item %s %s %s\n", versionLabel, "   ", v.Name))
	}
	b.WriteString("choose selected || goto exit\n\n")

	for _, v := range d.Versions {
		if !v.Enabled {
			continue
		}
		versionLabel := versionLabel(d, v)
		b.WriteString(fmt.Sprintf(":%s\n", versionLabel))
		b.WriteString(GenerateBootLine(v, serverAddr, bootPrefix, d.KernelParams, task, proxyHTTPS))
		b.WriteString("\n")
	}

	b.WriteString(":exit\n")
	b.WriteString("exit 0\n")
	return b.String()
}

// GenerateBootLine generates the kernel+initrd boot line for a version.
// Supports multiple boot types: kernel (default), memdisk, sanboot, memtest, wimboot.
// If task is non-nil, answer parameters are injected into the cmdline.
// proxyHTTPS controls whether HTTPS URLs are proxied through the local HTTP server.
func GenerateBootLine(v *Version, serverAddr, bootPrefix, kernelParams string, task *BootTaskInfo, proxyHTTPS bool) string {
	var kernelURL, initrdURL string

	if v.Local != nil {
		kernelURL = resolveLocalURL(serverAddr, bootPrefix, v.Local.Kernel)
		initrdURL = resolveLocalURL(serverAddr, bootPrefix, v.Local.Initrd)
	} else if v.Remote != nil {
		kernelURL = proxyRemoteURL(serverAddr, bootPrefix, v.Remote.Kernel, proxyHTTPS)
		initrdURL = proxyRemoteURL(serverAddr, bootPrefix, v.Remote.Initrd, proxyHTTPS)
	}

	switch v.BootType {
	case BootMemtest:
		if kernelURL == "" {
			return "# No boot file configured\n"
		}
		return fmt.Sprintf("kernel %s\nboot\n", kernelURL)

	case BootWimboot:
		if kernelURL == "" || initrdURL == "" {
			return "# Windows PE requires wimboot URL (kernel) and Windows base URL (initrd)\n"
		}
		return generateWimbootLine(kernelURL, initrdURL, task)

	case BootMemdisk:
		if initrdURL == "" {
			return "# No boot file configured\n"
		}
		return fmt.Sprintf("kernel memdisk\ninitrd %s\nboot\n", initrdURL)

	case BootSanboot:
		url := initrdURL
		if url == "" {
			url = kernelURL
		}
		var b strings.Builder
		if v.SANKeepSAN {
			b.WriteString("set keep-san 1\n")
		}
		switch v.SANAction {
		case "hook":
			b.WriteString(fmt.Sprintf("sanhook %s\n", url))
		case "zap":
			b.WriteString("sanunhook\n")
		case "unhook":
			b.WriteString("sanhook\n")
		default:
			b.WriteString("sanboot")
			if v.SANNoDescribe {
				b.WriteString(" --no-describe")
			}
			if v.SANDrive != "" {
				b.WriteString(fmt.Sprintf(" --drive %s", v.SANDrive))
			}
			if url != "" {
				b.WriteString(fmt.Sprintf(" %s", url))
			}
			b.WriteString("\n")
		}
		return b.String()

	default: // BootKernel — standard kernel+initrd+boot
		if kernelURL == "" && initrdURL == "" {
			return "# No boot files configured\n"
		}
		params := buildCmdline(v.Cmdline, kernelParams, task)
		if params != "" {
			return fmt.Sprintf("kernel %s %s\ninitrd %s\nboot\n", kernelURL, params, initrdURL)
		}
		return fmt.Sprintf("kernel %s\ninitrd %s\nboot\n", kernelURL, initrdURL)
	}
}

// WimbootAnswerInitrds 返回把应答文件注入 wimboot 启动的 WinPE 所需的 initrd 行。
//
// wimboot 会把每个具名 initrd 文件放进 WinPE 内存盘的 X:\Windows\System32\<name>
// （见 https://ipxe.org/wimboot "Injected files"），因此：
//   - winpeshl 路线：模板内容作为 install.bat，winpeshl.ini 通过 ?file= 单独返回；
//     winpeshl.exe 从 %SYSTEMROOT%\System32 读取 winpeshl.ini，install.bat 同样落在
//     该目录，故 winpeshl.ini 里的 AppPath 必须指向 %SYSTEMROOT%\System32\install.bat。
//   - 其它路线：autounattend.xml。
//
// 必须用 -n <name> 指定注入文件名：initrd 的行尾位置参数是 image arguments，不改变
// 文件名；若省略 -n，wimboot 会用 URL 的 basename（此处为 task id），导致文件名错误
// 甚至多个文件同名互相覆盖。
func WimbootAnswerInitrds(answerURL, answerType string) []string {
	if answerURL == "" {
		return nil
	}
	if answerType == "winpeshl" {
		return []string{
			fmt.Sprintf("initrd -n install.bat %s", answerURL),
			fmt.Sprintf("initrd -n winpeshl.ini %s?file=winpeshl.ini", answerURL),
		}
	}
	return []string{fmt.Sprintf("initrd -n autounattend.xml %s", answerURL)}
}

// generateWimbootLine generates a wimboot-based Windows boot line.
// If task is present with answer info, it injects the answer file as an initrd entry.
func generateWimbootLine(kernelURL, initrdURL string, task *BootTaskInfo) string {
	index := ""
	if task != nil && task.WIMIndex > 0 {
		index = fmt.Sprintf(" index=%d", task.WIMIndex)
	}

	answer := ""
	if task != nil && task.AnswerURL != "" {
		if lines := WimbootAnswerInitrds(task.AnswerURL, task.AnswerType); len(lines) > 0 {
			answer = strings.Join(lines, "\n") + "\n"
		}
	}

	return fmt.Sprintf("kernel %s%s\n"+
		"initrd -n bootmgr %s/bootmgr bootmgr\n"+
		"initrd -n bootmgr.efi %s/bootmgr.efi bootmgr.efi\n"+
		"initrd -n bcd %s/boot/bcd bcd\n"+
		"initrd -n boot.sdi %s/boot/boot.sdi boot.sdi\n"+
		"initrd -n boot.wim %s/sources/boot.wim boot.wim\n"+
		"%s"+
		"boot\n",
		kernelURL, index,
		initrdURL, initrdURL, initrdURL, initrdURL, initrdURL,
		answer)
}

// buildCmdline assembles the final kernel command line, including answer parameter injection.
func buildCmdline(versionCmdline, distroKernelParams string, task *BootTaskInfo) string {
	params := versionCmdline
	if distroKernelParams != "" && params == "" {
		params = distroKernelParams
	}

	if task != nil {
		if task.ExtraCmdline != "" {
			if params != "" {
				params = params + " " + task.ExtraCmdline
			} else {
				params = task.ExtraCmdline
			}
		}
		answerParam := task.AnswerParam
		if answerParam != "" {
			injected := InjectAnswerParam(answerParam, task.AnswerURL)
			if injected != "" {
				if params != "" {
					params = params + " " + injected
				} else {
					params = injected
				}
			}
		}
	}

	return params
}

func sanitizeLabel(s string) string {
	return reLabelChars.ReplaceAllString(strings.ToLower(s), "_")
}

// proxyRemoteURL rewrites HTTPS URLs to go through PxeLab's HTTP proxy,
// so that iPXE firmware without HTTPS support can fetch them.
// When proxyHTTPS is false, the original URL is returned unchanged.
func proxyRemoteURL(serverAddr, bootPrefix, remoteURL string, proxyHTTPS bool) string {
	if !proxyHTTPS {
		return remoteURL
	}
	if strings.HasPrefix(remoteURL, "https://") {
		stripped := strings.TrimPrefix(remoteURL, "https://")
		return fmt.Sprintf("http://%s%s/proxy/https/%s", serverAddr, bootPrefix, stripped)
	}
	return remoteURL
}

// resolveLocalURL resolves a local boot file reference. Relative paths are
// prefixed with the boot prefix; absolute http(s) URLs (used for wimboot and
// locally extracted Windows ISOs) pass through unchanged.
func resolveLocalURL(serverAddr, bootPrefix, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	return fmt.Sprintf("http://%s%s/%s", serverAddr, bootPrefix, ref)
}

func distroLabel(d *Distro) string {
	return fmt.Sprintf("distro_%s", sanitizeLabel(d.Name))
}

func versionLabel(d *Distro, v *Version) string {
	return fmt.Sprintf("boot_%s_%s", sanitizeLabel(d.Name), sanitizeLabel(v.Codename))
}

// groupTitleFallback returns the configured title if available, otherwise uses hardcoded defaults.
func groupTitleFallback(name string, configured map[string]string) string {
	if t, ok := configured[name]; ok && t != "" {
		return "== " + t + " =="
	}
	titles := map[string]string{
		"linux":       "== Linux Distributions ==",
		"linux-i386":  "== Linux Distributions (32-bit) ==",
		"linux-arm64": "== Linux Distributions (arm64) ==",
		"bsd":         "== BSD Systems ==",
		"live":        "== Live CDs ==",
		"live-arm":    "== Live CDs (arm64) ==",
		"tools":       "== System Tools ==",
		"unix":        "== Unix ==",
		"dos":         "== DOS ==",
		"windows":     "== Windows ==",
	}
	if t, ok := titles[name]; ok {
		return t
	}
	return "== " + name + " =="
}
