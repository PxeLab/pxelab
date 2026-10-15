package api

// R6 驱动包：约定 boot 根目录下 drivers/<名称>/ 即一个驱动包（内含 .inf 等）。
// 包内文件经既有 /boot/ 公开文件服务匿名拉取；首登命令在应答注入时按
// 当时的目录快照生成（下载清单 + pnputil 安装），无需新增下载端点。
//
//   GET /api/v1/driver-packages — 列出可用驱动包（扫描 drivers/ 目录）

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pxelab/pxelab/internal/boot"
)

// driverPullMarker 标记首登编排中的驱动安装命令（幂等检测用）。
const driverPullMarker = "pxelab-driver-pull"

// driverNameRe 限定驱动包名：防目录遍历，也保证能安全嵌入 PowerShell 单引号字符串与 URL。
var driverNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type DriverPackageHandler struct {
	bootFS *boot.BootFileServer
}

type driverPackageDTO struct {
	Name     string `json:"name"`
	Files    int    `json:"files"`
	InfFiles int    `json:"inf_files"`
	// Archs 包内出现的架构（归一化 x64/x86/arm64，去重稳定排序）。
	// 空数组 = 扁平布局（无架构子目录），全架构通用。
	Archs []string `json:"archs"`
}

// driverPackArchs 从包内文件清单推导出现的架构（借鉴项 3）：首段命中架构
// 子目录（x64/、amd64/、x86_64/、arm64/…）即归一化计入，去重并按
// x64/x86/arm64 稳定排序。扁平布局（无架构子目录）返回空切片。
func driverPackArchs(files []string) []string {
	seen := make(map[string]bool, 3)
	for _, f := range files {
		i := strings.Index(f, "/")
		if i <= 0 {
			continue // 顶层文件：通用
		}
		seg := f[:i]
		if !driverArchDirs[strings.ToLower(seg)] {
			continue // 非架构目录：通用
		}
		if a := normalizeDriverArch(seg); a != "" {
			seen[a] = true
		}
	}
	out := make([]string, 0, len(seen))
	for _, a := range []string{"x64", "x86", "arm64"} {
		if seen[a] {
			out = append(out, a)
		}
	}
	return out
}

// listDriverPackFiles 递归列出驱动包内全部文件的相对路径（正斜杠分隔，排序后返回）。
// 包名非法或目录不存在/为空返回错误。
func listDriverPackFiles(bootRoot, name string) ([]string, error) {
	if !driverNameRe.MatchString(name) {
		return nil, fmt.Errorf("无效的驱动包名: %q", name)
	}
	dir := filepath.Join(bootRoot, "drivers", name)
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("驱动包为空: %s", name)
	}
	return files, nil
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
		files, err := listDriverPackFiles(h.bootFS.Root(), name)
		if err != nil {
			continue // 非法名/非目录/空包不列入
		}
		inf := 0
		for _, f := range files {
			if strings.HasSuffix(strings.ToLower(f), ".inf") {
				inf++
			}
		}
		packs = append(packs, driverPackageDTO{
			Name:     name,
			Files:    len(files),
			InfFiles: inf,
			Archs:    driverPackArchs(files),
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

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

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
		files = filterDriverFilesByArch(files, arch)
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

// driverArchDirs 驱动包内可能出现的架构子目录名（大小写不敏感）。
var driverArchDirs = map[string]bool{
	"x64": true, "amd64": true, "x86_64": true,
	"x86": true, "i386": true, "i486": true, "i586": true, "i686": true, "386": true,
	"arm64": true, "aarch64": true,
	"ia64": true,
}

// normalizeDriverArch 把平台/目录架构名归一为 x64/x86/arm64；无法识别返回 ""。
func normalizeDriverArch(arch string) string {
	switch strings.ToLower(strings.TrimSpace(arch)) {
	case "x64", "amd64", "x86_64":
		return "x64"
	case "x86", "i386", "i486", "i586", "i686", "386":
		return "x86"
	case "arm64", "aarch64":
		return "arm64"
	}
	return ""
}

// filterDriverFilesByArch 按架构过滤驱动包文件（借用项 3 的轻量版）：
//   - 仅当包内存在架构子目录（如 x64/）时才启用过滤，避免误伤扁平布局的老包；
//   - 顶层文件与非架构子目录视为通用，始终保留；
//   - 架构子目录仅保留与目标架构匹配者。
//
// arch 无法识别或包内无架构子目录时原样返回。
func filterDriverFilesByArch(files []string, arch string) []string {
	target := normalizeDriverArch(arch)
	if target == "" {
		return files
	}
	hasArchSubdir := false
	for _, f := range files {
		if i := strings.Index(f, "/"); i > 0 && driverArchDirs[strings.ToLower(f[:i])] {
			hasArchSubdir = true
			break
		}
	}
	if !hasArchSubdir {
		return files
	}
	out := make([]string, 0, len(files))
	for _, f := range files {
		i := strings.Index(f, "/")
		if i <= 0 {
			out = append(out, f) // 顶层：通用
			continue
		}
		seg := strings.ToLower(f[:i])
		if !driverArchDirs[seg] {
			out = append(out, f) // 非架构目录：通用
			continue
		}
		if normalizeDriverArch(seg) == target {
			out = append(out, f)
		}
	}
	return out
}

// buildPeDriverInstallBlock 生成 winpeshl install.bat 的 PE 阶段驱动加载块（批处理）：
// 把该驱动包全部文件下载到 PE 可写临时目录，并在 setup.exe 之前用 pnputil
// （缺失时回退 drvload）安装，使 boot-critical 驱动在 Windows Setup 扫描磁盘前就绪。
// 下载器优先 curl.exe（Win10+ WinPE 自带），回退 certutil -urlcache（WinPE 长期自带）。
// 全程 >nul 2>nul 静默、不设 setlocal：任何失败都不阻断后续 install.bat（驱动为可选增强）。
func buildPeDriverInstallBlock(baseURL, name string, files []string) string {
	dest := `X:\pxelab-drivers\` + name
	var b strings.Builder
	b.WriteString("rem PxeLab PE driver load (" + driverPullMarker + "): " + name + "\n")
	b.WriteString(`set "_pxelab_drv=` + dest + `"` + "\n")
	b.WriteString(`if not exist "%_pxelab_drv%" md "%_pxelab_drv%" >nul 2>nul` + "\n")
	// 先建子目录（保留包内相对结构，pnputil /subdirs 需要）
	subdirs := map[string]bool{}
	for _, f := range files {
		if idx := strings.LastIndex(f, "/"); idx > 0 {
			subdirs[f[:idx]] = true
		}
	}
	for _, sub := range sortedKeys(subdirs) {
		local := strings.ReplaceAll(sub, "/", `\`)
		b.WriteString(`if not exist "%_pxelab_drv%\` + local + `" md "%_pxelab_drv%\` + local + `" >nul 2>nul` + "\n")
	}
	// 下载器探测一次：默认 certutil，存在 curl 时改用 curl
	b.WriteString(`set "_pxelab_dl=certutil"` + "\n")
	b.WriteString(`where curl.exe >nul 2>nul && set "_pxelab_dl=curl"` + "\n")
	for _, f := range files {
		segments := strings.Split(f, "/")
		for i, seg := range segments {
			segments[i] = url.PathEscape(seg)
		}
		fileURL := fmt.Sprintf("http://%s/boot/drivers/%s/%s", baseURL, url.PathEscape(name), strings.Join(segments, "/"))
		destPath := `%_pxelab_drv%\` + strings.ReplaceAll(f, "/", `\`)
		b.WriteString(`if "%_pxelab_dl%"=="curl" (curl.exe -fsSL -o "` + destPath + `" "` + fileURL + `") else (certutil -urlcache -split -f "` + fileURL + `" "` + destPath + `" >nul 2>nul)` + "\n")
	}
	// 安装：WinPE 3.0+ 的 pnputil；缺失时用 WinPE 专用的轻量 drvload 逐个 .inf 兜底
	b.WriteString(`where pnputil.exe >nul 2>nul` + "\n")
	b.WriteString(`if %errorlevel%==0 (pnputil /add-driver "%_pxelab_drv%\*.inf" /subdirs /install >nul 2>nul) else (for /r "%_pxelab_drv%" %%i in (*.inf) do drvload "%%i" >nul 2>nul)` + "\n")
	return b.String()
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
