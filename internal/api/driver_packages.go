package api

// R6 驱动包：约定 boot 根目录下 drivers/<名称>/ 即一个驱动包（内含 .inf 等）。
// 包内文件经既有 /boot/ 公开文件服务匿名拉取；首登命令在应答注入时按
// 当时的目录快照生成（下载清单 + pnputil 安装），无需新增下载端点。
//
//   GET /api/v1/driver-packages — 列出可用驱动包（扫描 drivers/ 目录）

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pxelab/pxelab/internal/boot"
	"github.com/pxelab/pxelab/internal/drivers"
)

// driverPullMarker 标记首登编排中的驱动安装命令（幂等检测用）。
// 实现已移至 internal/drivers（供 internal/httpd 生成 initrd 直投菜单复用）。
const driverPullMarker = drivers.PullMarker

type DriverPackageHandler struct {
	bootFS *boot.BootFileServer
}

type driverPackageDTO struct {
	Name      string `json:"name"`
	Files     int    `json:"files"`
	InfFiles  int    `json:"inf_files"`
	SizeBytes int64  `json:"size_bytes"`
	// Archs 包内出现的架构（归一化 x64/x86/arm64，去重稳定排序）。
	// 空数组 = 扁平布局（无架构子目录），全架构通用。
	Archs []string `json:"archs"`
}

// driverPackArchs 见 drivers.PackArchs（包内别名，保持既有测试可用）。
func driverPackArchs(files []string) []string { return drivers.PackArchs(files) }

// listDriverPackFiles 见 drivers.ListPackFiles。
func listDriverPackFiles(bootRoot, name string) ([]string, error) {
	return drivers.ListPackFiles(bootRoot, name)
}

// List 扫描 boot 根目录 drivers/，每个含文件的合法子目录即一个驱动包。
func (h *DriverPackageHandler) List(w http.ResponseWriter, r *http.Request) {
	root := filepath.Join(h.bootFS.Root(), "drivers")
	entries, err := filepath.Glob(filepath.Join(root, "*"))
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	packs := make([]driverPackageDTO, 0, len(entries))
	for _, dir := range entries {
		name := filepath.Base(dir)
		proj, err := drivers.Project(h.bootFS.Root(), name, "")
		if err != nil {
			continue // 非法名/非目录/空包不列入
		}
		packs = append(packs, driverPackageDTO{
			Name:      proj.Pack,
			Files:     proj.TotalFiles,
			InfFiles:  proj.InfFiles,
			SizeBytes: proj.TotalBytes,
			Archs:     proj.Archs,
		})
	}
	OK(w, map[string]any{"packages": packs})
}

// driverCommand 一个驱动包的首登安装命令（name 用于编排描述）。
type driverCommand struct {
	name string
	cmd  string
}

// psSingleQuoted 把字符串嵌入 PowerShell 单引号字面量（' → ”）。
// 驱动包名/文件路径已经过 driverNameRe 与 WalkDir 相对路径约束，这里只做兜底转义。
func psSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// buildDriverInstallCommand 生成单个驱动包的首登安装命令（PowerShell）：
//  1. 在 %TEMP%\pxelab-drivers\<name> 重建目录结构；
//  2. WebClient 逐个拉取包内文件（平台 /boot/ 公开文件服务，匿名可达）；
//  3. pnputil /add-driver <dir>\*.inf /subdirs /install 安装。
//
// 任何一步失败都被外层 try/catch 静默，不阻断后续首登命令。
func buildDriverInstallCommand(baseURL, name string, files []string) string {
	var b strings.Builder
	b.WriteString(`powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "try { `)
	b.WriteString(`$dest = Join-Path $env:TEMP ` + psSingleQuoted(`pxelab-drivers\`+name) + `; `)
	b.WriteString(`New-Item -ItemType Directory -Force $dest | Out-Null; `)
	// 先建子目录，再逐个下载
	subdirs := map[string]bool{}
	for _, f := range files {
		if idx := strings.LastIndex(f, "/"); idx > 0 {
			subdirs[f[:idx]] = true
		}
	}
	for _, sub := range sortedKeys(subdirs) {
		b.WriteString(`New-Item -ItemType Directory -Force (Join-Path $dest ` + psSingleQuoted(strings.ReplaceAll(sub, "/", `\`)) + `) | Out-Null; `)
	}
	b.WriteString(`$wc = New-Object Net.WebClient; `)
	for _, f := range files {
		// URL 逐段转义（空格等）；本地路径用反斜杠
		segments := strings.Split(f, "/")
		for i, seg := range segments {
			segments[i] = url.PathEscape(seg)
		}
		fileURL := fmt.Sprintf("http://%s/boot/drivers/%s/%s", baseURL, url.PathEscape(name), strings.Join(segments, "/"))
		localRel := strings.ReplaceAll(f, "/", `\`)
		b.WriteString(`$wc.DownloadFile(` + psSingleQuoted(fileURL) + `, (Join-Path $dest ` + psSingleQuoted(localRel) + `)); `)
	}
	b.WriteString(`pnputil /add-driver ($dest + '\*.inf') /subdirs /install } catch {}"`)
	return b.String()
}

// sortedKeys 见 drivers.SortedKeys。
func sortedKeys(m map[string]bool) []string { return drivers.SortedKeys(m) }

// driverPackFiles 返回某个驱动包按目标架构过滤后的文件清单。
// bootFS 不可用、包名非法/不存在/为空、或无匹配架构的文件时返回错误。
func (h *InstallTaskHandler) driverPackFiles(name, arch string) ([]string, error) {
	if h.bootFS == nil {
		return nil, fmt.Errorf("boot 文件服务不可用")
	}
	files, err := listDriverPackFiles(h.bootFS.Root(), name)
	if err != nil {
		return nil, err
	}
	if arch != "" {
		files = drivers.FilterByArch(files, arch)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("驱动包无匹配架构 %q 的文件: %s", arch, name)
	}
	return files, nil
}

// buildDriverCommands 为主机绑定的每个驱动包生成首登安装命令；bootFS 不可用、
// 包名非法、包为空或无匹配架构文件的跳过并告警（不阻断其余包与基线编排）。
// arch 为目标架构（如 amd64）——按 借用项 3 的轻量约定过滤 boot/drivers/<名称>/<arch>/。
func (h *InstallTaskHandler) buildDriverCommands(baseURL string, packs []string, arch string) []driverCommand {
	if h.bootFS == nil || len(packs) == 0 {
		return nil
	}
	var out []driverCommand
	for _, name := range packs {
		files, err := h.driverPackFiles(name, arch)
		if err != nil {
			slog.Warn("跳过不可用驱动包", "pack", name, "arch", arch, "error", err)
			continue
		}
		out = append(out, driverCommand{name: name, cmd: buildDriverInstallCommand(baseURL, name, files)})
	}
	return out
}

// ── PE 阶段运行时驱动加载（借用项 2）────────────────────────────────────────
// winpeshl install.bat 在 setup.exe 之前下载驱动包并 pnputil/drvload 安装，
// 覆盖 boot-critical 场景（存储/网卡），与首登注入互补。跨平台、无需 WIM 操作。

// filterDriverFilesByArch 见 drivers.FilterByArch。
func filterDriverFilesByArch(files []string, arch string) []string {
	return drivers.FilterByArch(files, arch)
}

// buildPeDriverInstallBlock 见 drivers.PeInstallBlockHTTP。
func buildPeDriverInstallBlock(baseURL, name string, files []string) string {
	return drivers.PeInstallBlockHTTP(baseURL, name, files)
}

// insertAfterEchoOff 把 block 插入批处理内容顶部：若首行是 @echo off / echo off，
// 则插到其后（避免回显）；否则直接前置。block 需自带行尾换行。
func insertAfterEchoOff(content, block string) string {
	if content == "" {
		return block
	}
	nl := strings.IndexByte(content, '\n')
	first, rest := content, ""
	if nl >= 0 {
		first, rest = content[:nl], content[nl+1:]
	}
	trimmed := strings.TrimSpace(strings.TrimSuffix(first, "\r"))
	if strings.EqualFold(trimmed, "@echo off") || strings.EqualFold(trimmed, "echo off") {
		return first + "\n" + block + rest
	}
	return block + content
}

// injectWinpeShellDrivers 在 winpeshl install.bat 内容顶部注入 PE 阶段驱动加载块：
// 逐个驱动包生成“下载 + pnputil/drvload 安装”的批处理段，确保在 setup.exe 之前执行。
// 返回注入后的内容；无可用驱动包（bootFS 不可用/包非法/无匹配架构）或已注入过时 ok=false/原样。
func (h *InstallTaskHandler) injectWinpeShellDrivers(content string, packs []string, arch, baseURL string) (string, bool) {
	if h.bootFS == nil || len(packs) == 0 {
		return content, false
	}
	if strings.Contains(content, driverPullMarker) {
		return content, true // 幂等：已注入
	}
	var blocks []string
	for _, name := range packs {
		files, err := h.driverPackFiles(name, arch)
		if err != nil {
			slog.Warn("winpeshl PE 驱动注入跳过驱动包", "pack", name, "arch", arch, "error", err)
			continue
		}
		blocks = append(blocks, buildPeDriverInstallBlock(baseURL, name, files))
	}
	if len(blocks) == 0 {
		return content, false
	}
	return insertAfterEchoOff(content, strings.Join(blocks, "")), true
}
