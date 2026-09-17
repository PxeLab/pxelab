package api

// R6 驱动包：约定 boot 根目录下 drivers/<名称>/ 即一个驱动包（内含 .inf 等）。
// 包内文件经既有 /boot/ 公开文件服务匿名拉取；首登命令在应答注入时按
// 当时的目录快照生成（下载清单 + pnputil 安装），无需新增下载端点。
//
//   GET /api/v1/driver-packages — 列出可用驱动包（扫描 drivers/ 目录）

import (
	"fmt"
	"io/fs"
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
		packs = append(packs, driverPackageDTO{Name: name, Files: len(files), InfFiles: inf})
	}
	OK(w, map[string]any{"packages": packs})
}

// driverCommand 一个驱动包的首登安装命令（name 用于编排描述）。
type driverCommand struct {
	name string
	cmd  string
}

// psSingleQuoted 把字符串嵌入 PowerShell 单引号字面量（' → ''）。
// 驱动包名/文件路径已经过 driverNameRe 与 WalkDir 相对路径约束，这里只做兜底转义。
func psSingleQuoted(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// buildDriverInstallCommand 生成单个驱动包的首登安装命令（PowerShell）：
//  1. 在 %TEMP%\pxelab-drivers\<name> 重建目录结构；
//  2. WebClient 逐个拉取包内文件（平台 /boot/ 公开文件服务，匿名可达）；
//  3. pnputil /add-driver <dir>\*.inf /subdirs /install 安装。
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

// buildDriverCommands 为主机绑定的每个驱动包生成首登安装命令；bootFS 不可用、
// 包名非法或包为空的跳过并告警（不阻断其余包与基线编排）。
func (h *InstallTaskHandler) buildDriverCommands(baseURL string, packs []string) []driverCommand {
	if h.bootFS == nil || len(packs) == 0 {
		return nil
	}
	var out []driverCommand
	for _, name := range packs {
		files, err := listDriverPackFiles(h.bootFS.Root(), name)
		if err != nil {
			continue
		}
		out = append(out, driverCommand{name: name, cmd: buildDriverInstallCommand(baseURL, name, files)})
	}
	return out
}
