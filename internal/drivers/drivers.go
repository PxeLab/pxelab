// Package drivers 集中承载驱动包（boot 根目录 drivers/<名称>/）的领域逻辑：
// 目录列举、架构过滤、包内架构推导，以及两种 PE 阶段驱动交付方式的文本生成：
//
//   - HTTP 下载（默认）：PE 内 curl/certutil 拉取 /boot/drivers/<包>/ 后用 pnputil 安装；
//   - iPXE initrd 直投（可选）：每个驱动文件作为具名 initrd 注入 WinPE 内存盘的
//     %SYSTEMROOT%\System32，再由 install.bat 落地到 X:\pxelab-drivers\ 后 pnputil 安装。
//
// 两种方式共享同一份「文件清单 + 架构过滤 + 注入文件名派生」实现，供
// internal/api（install.bat 内容）与 internal/httpd（iPXE 菜单 initrd 行）
// 复用，保证菜单侧与 install.bat 侧的注入文件名必然一致。
package drivers

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// 驱动交付方式（netboot.driver_delivery）。
const (
	DeliveryHTTP   = "http"   // 默认：PE 内 HTTP 下载（curl/certutil）+ pnputil
	DeliveryInitrd = "initrd" // iPXE initrd 直投 + install.bat 落地 + pnputil
)

// PullMarker 标记首登/PE 编排中的驱动安装命令（幂等检测用）。
const PullMarker = "pxelab-driver-pull"

// NameRe 限定驱动包名：防目录遍历，也保证能安全嵌入 PowerShell 单引号字符串与 URL。
var NameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ArchDirs 驱动包内可能出现的架构子目录名（大小写不敏感）。
var ArchDirs = map[string]bool{
	"x64": true, "amd64": true, "x86_64": true,
	"x86": true, "i386": true, "i486": true, "i586": true, "i686": true, "386": true,
	"arm64": true, "aarch64": true,
	"ia64": true,
}

// NormalizeDelivery 归一化驱动交付方式；空/未知值回落默认 http。
func NormalizeDelivery(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), DeliveryInitrd) {
		return DeliveryInitrd
	}
	return DeliveryHTTP
}

// ValidDelivery 报告 mode 是否为受支持的交付方式（空视为合法 http）。
func ValidDelivery(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", DeliveryHTTP, DeliveryInitrd:
		return true
	}
	return false
}

// NormalizeArch 把平台/目录架构名归一为 x64/x86/arm64；无法识别返回 ""。
func NormalizeArch(arch string) string {
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

// FileInfo 驱动包内的一个文件：相对路径（正斜杠）+ 字节大小。
type FileInfo struct {
	Rel  string
	Size int64
}

// ListPackFilesInfo 递归列出驱动包内全部文件（含字节大小，按相对路径排序）。
// 包名非法或目录不存在/为空返回错误。
func ListPackFilesInfo(bootRoot, name string) ([]FileInfo, error) {
	if !NameRe.MatchString(name) {
		return nil, fmt.Errorf("无效的驱动包名: %q", name)
	}
	dir := filepath.Join(bootRoot, "drivers", name)
	var files []FileInfo
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		var size int64
		if info, err := d.Info(); err == nil {
			size = info.Size()
		}
		files = append(files, FileInfo{Rel: filepath.ToSlash(rel), Size: size})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })
	if len(files) == 0 {
		return nil, fmt.Errorf("驱动包为空: %s", name)
	}
	return files, nil
}

// ListPackFiles 递归列出驱动包内全部文件的相对路径（正斜杠分隔，排序后返回）。
// 包名非法或目录不存在/为空返回错误。
func ListPackFiles(bootRoot, name string) ([]string, error) {
	infos, err := ListPackFilesInfo(bootRoot, name)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(infos))
	for _, fi := range infos {
		out = append(out, fi.Rel)
	}
	return out, nil
}

// PackBytes 返回文件清单的总字节数。
func PackBytes(files []FileInfo) int64 {
	var n int64
	for _, f := range files {
		n += f.Size
	}
	return n
}

// Projection 一个驱动包在指定架构下的交付规模投影（用于 initrd 直投规模评估：
// Files 即该包将生成的 iPXE initrd 行数）。
type Projection struct {
	Pack       string   `json:"pack"`
	Archs      []string `json:"archs"`
	TotalFiles int      `json:"total_files"`
	TotalBytes int64    `json:"total_bytes"`
	InfFiles   int      `json:"inf_files"`
	Files      int      `json:"files"` // 架构过滤后（= initrd 行数）
	Bytes      int64    `json:"bytes"` // 架构过滤后
	FilesList  []string `json:"files_list"`
}

// Project 计算驱动包在 arch 下的规模投影。arch 为空表示不做架构过滤。
// 包名非法/不存在/为空返回错误。
func Project(bootRoot, name, arch string) (*Projection, error) {
	infos, err := ListPackFilesInfo(bootRoot, name)
	if err != nil {
		return nil, err
	}
	rels := make([]string, 0, len(infos))
	sizeOf := make(map[string]int64, len(infos))
	for _, fi := range infos {
		rels = append(rels, fi.Rel)
		sizeOf[fi.Rel] = fi.Size
	}
	total := PackBytes(infos)
	inf := 0
	for _, fi := range infos {
		if strings.HasSuffix(strings.ToLower(fi.Rel), ".inf") {
			inf++
		}
	}

	kept := rels
	if arch != "" {
		kept = FilterByArch(rels, arch)
	}
	var keptBytes int64
	for _, r := range kept {
		keptBytes += sizeOf[r]
	}
	return &Projection{
		Pack:       name,
		Archs:      PackArchs(rels),
		TotalFiles: len(infos),
		TotalBytes: total,
		InfFiles:   inf,
		Files:      len(kept),
		Bytes:      keptBytes,
		FilesList:  kept,
	}, nil
}

// FilterByArch 按架构过滤驱动包文件（借鉴项 3 的轻量版）：
//   - 仅当包内存在架构子目录（如 x64/）时才启用过滤，避免误伤扁平布局的老包；
//   - 顶层文件与非架构子目录视为通用，始终保留；
//   - 架构子目录仅保留与目标架构匹配者。
//
// arch 无法识别或包内无架构子目录时原样返回。
func FilterByArch(files []string, arch string) []string {
	target := NormalizeArch(arch)
	if target == "" {
		return files
	}
	hasArchSubdir := false
	for _, f := range files {
		if i := strings.Index(f, "/"); i > 0 && ArchDirs[strings.ToLower(f[:i])] {
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
		if !ArchDirs[seg] {
			out = append(out, f) // 非架构目录：通用
			continue
		}
		if NormalizeArch(seg) == target {
			out = append(out, f)
		}
	}
	return out
}

// PackArchs 从包内文件清单推导出现的架构（借鉴项 3）：首段命中架构子目录即
// 归一化计入，去重并按 x64/x86/arm64 稳定排序。扁平布局返回空切片。
func PackArchs(files []string) []string {
	seen := make(map[string]bool, 3)
	for _, f := range files {
		i := strings.Index(f, "/")
		if i <= 0 {
			continue
		}
		seg := f[:i]
		if !ArchDirs[strings.ToLower(seg)] {
			continue
		}
		if a := NormalizeArch(seg); a != "" {
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

// EffectivePacks 驱动包继承规则（B2 的业务规则）：主机级绑定优先，否则用
// Profile 默认；两者皆空返回 nil。调用方各自负责取 hostPacks / profilePacks。
func EffectivePacks(hostPacks, profilePacks []string) []string {
	if len(hostPacks) > 0 {
		return hostPacks
	}
	if len(profilePacks) > 0 {
		return profilePacks
	}
	return nil
}

// SortedKeys 返回 map 键的排序切片（生成确定性批处理脚本）。
func SortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// fileURL 拼接驱动文件的 HTTP 下载地址（URL 逐段转义）。
func fileURL(baseURL, pack, rel string) string {
	segments := strings.Split(rel, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return fmt.Sprintf("http://%s/boot/drivers/%s/%s", baseURL, url.PathEscape(pack), strings.Join(segments, "/"))
}

// subdirsOf 收集相对路径中的子目录（生成 md 命令用）。
func subdirsOf(rels []string) []string {
	m := map[string]bool{}
	for _, rel := range rels {
		if idx := strings.LastIndex(rel, "/"); idx > 0 {
			m[rel[:idx]] = true
		}
	}
	return SortedKeys(m)
}

// pnputilBlock 生成「pnputil 安装 / drvload 兜底」命令段。
func pnputilBlock() string {
	var b strings.Builder
	b.WriteString(`where pnputil.exe >nul 2>nul` + "\n")
	b.WriteString(`if %errorlevel%==0 (pnputil /add-driver "%_pxelab_drv%\*.inf" /subdirs /install >nul 2>nul) else (for /r "%_pxelab_drv%" %%i in (*.inf) do drvload "%%i" >nul 2>nul)` + "\n")
	return b.String()
}

// PeInstallBlockHTTP 生成 winpeshl install.bat 的 PE 阶段「HTTP 下载 + 安装」块（批处理）：
// 把该驱动包全部文件下载到 PE 可写临时目录，并在 setup.exe 之前用 pnputil
// （缺失时回退 drvload）安装，使 boot-critical 驱动在 Windows Setup 扫描磁盘前就绪。
// 下载器优先 curl.exe（Win10+ WinPE 自带），回退 certutil -urlcache（WinPE 长期自带）。
// 全程 >nul 2>nul 静默、不设 setlocal：任何失败都不阻断后续 install.bat（驱动为可选增强）。
func PeInstallBlockHTTP(baseURL, name string, files []string) string {
	dest := `X:\pxelab-drivers\` + name
	var b strings.Builder
	b.WriteString("rem PxeLab PE driver load (" + PullMarker + "): " + name + "\n")
	b.WriteString(`set "_pxelab_drv=` + dest + `"` + "\n")
	b.WriteString(`if not exist "%_pxelab_drv%" md "%_pxelab_drv%" >nul 2>nul` + "\n")
	// 先建子目录（保留包内相对结构，pnputil /subdirs 需要）
	for _, sub := range subdirsOf(files) {
		local := strings.ReplaceAll(sub, "/", `\`)
		b.WriteString(`if not exist "%_pxelab_drv%\` + local + `" md "%_pxelab_drv%\` + local + `" >nul 2>nul` + "\n")
	}
	// 下载器探测一次：默认 certutil，存在 curl 时改用 curl
	b.WriteString(`set "_pxelab_dl=certutil"` + "\n")
	b.WriteString(`where curl.exe >nul 2>nul && set "_pxelab_dl=curl"` + "\n")
	for _, f := range files {
		destPath := `%_pxelab_drv%\` + strings.ReplaceAll(f, "/", `\`)
		u := fileURL(baseURL, name, f)
		b.WriteString(`if "%_pxelab_dl%"=="curl" (curl.exe -fsSL -o "` + destPath + `" "` + u + `") else (certutil -urlcache -split -f "` + u + `" "` + destPath + `" >nul 2>nul)` + "\n")
	}
	b.WriteString(pnputilBlock())
	return b.String()
}

// Injection 一个驱动文件对应的 iPXE initrd 注入项。
type Injection struct {
	Pack   string // 驱动包名
	Rel    string // 包内相对路径（正斜杠）
	Inject string // 注入到 %SYSTEMROOT%\System32 的文件名（由 Pack+Rel 确定性派生）
}

// InjectName 由 pack+rel 确定性派生注入文件名：与文件顺序无关，因此 iPXE 菜单
// （initrd 行）与 install.bat（落地 copy 命令）两侧必然指向同一文件。
// 形如 pxdrv_<sha1前12位><原扩展名>。用 NUL 作分隔符避免 pack/rel 边界歧义
// （包名受 NameRe 约束不含 NUL）。
func InjectName(pack, rel string) string {
	sum := sha1.Sum([]byte(pack + "\x00" + rel))
	return "pxdrv_" + hex.EncodeToString(sum[:])[:12] + path.Ext(rel)
}

// PlanInjections 计算把 packs（按 arch 过滤后）以 iPXE initrd 直投所需的注入计划。
// 逐个包处理：非法名/不存在/为空/无匹配架构文件的包被跳过，并把原因一并返回
// （与既有“跳过不可用驱动包”行为一致，不因单包失败而中断整批）。
func PlanInjections(bootRoot string, packs []string, arch string) ([]Injection, []error) {
	var out []Injection
	var errs []error
	for _, name := range packs {
		files, err := ListPackFiles(bootRoot, name)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if arch != "" {
			files = FilterByArch(files, arch)
		}
		if len(files) == 0 {
			errs = append(errs, fmt.Errorf("驱动包无匹配架构 %q 的文件: %s", arch, name))
			continue
		}
		for _, f := range files {
			out = append(out, Injection{Pack: name, Rel: f, Inject: InjectName(name, f)})
		}
	}
	return out, errs
}

// InitrdLines 由注入计划生成 iPXE initrd 行（每文件一行）：
// initrd -n <注入名> http://<base>/boot/drivers/<包>/<相对路径>
func InitrdLines(baseURL string, injections []Injection) []string {
	lines := make([]string, 0, len(injections))
	for _, in := range injections {
		lines = append(lines, fmt.Sprintf("initrd -n %s %s", in.Inject, fileURL(baseURL, in.Pack, in.Rel)))
	}
	return lines
}

// PeInstallBlockInitrd 由注入计划生成 install.bat 顶部「落地 + pnputil/drvload」块：
// 把已注入到 %SYSTEMROOT%\System32 的各文件 copy 到 X:\pxelab-drivers\<包>\<相对路径>
// 并重建子目录结构（pnputil /subdirs 需要），再安装。用 copy（非 move）以便重复执行。
func PeInstallBlockInitrd(injections []Injection) string {
	if len(injections) == 0 {
		return ""
	}
	// 按包分组并保持包的首次出现顺序
	byPack := map[string][]Injection{}
	var order []string
	for _, in := range injections {
		if _, ok := byPack[in.Pack]; !ok {
			order = append(order, in.Pack)
		}
		byPack[in.Pack] = append(byPack[in.Pack], in)
	}

	var b strings.Builder
	for _, pack := range order {
		list := byPack[pack]
		rels := make([]string, 0, len(list))
		for _, in := range list {
			rels = append(rels, in.Rel)
		}
		b.WriteString("rem PxeLab PE driver load via iPXE initrd (" + PullMarker + "): " + pack + "\n")
		b.WriteString(`set "_pxelab_drv=X:\pxelab-drivers\` + pack + `"` + "\n")
		b.WriteString(`if not exist "%_pxelab_drv%" md "%_pxelab_drv%" >nul 2>nul` + "\n")
		for _, sub := range subdirsOf(rels) {
			local := strings.ReplaceAll(sub, "/", `\`)
			b.WriteString(`if not exist "%_pxelab_drv%\` + local + `" md "%_pxelab_drv%\` + local + `" >nul 2>nul` + "\n")
		}
		for _, in := range list {
			dst := `%_pxelab_drv%\` + strings.ReplaceAll(in.Rel, "/", `\`)
			b.WriteString(`copy /y "%SYSTEMROOT%\System32\` + in.Inject + `" "` + dst + `" >nul 2>nul` + "\n")
		}
		b.WriteString(pnputilBlock())
	}
	return b.String()
}
