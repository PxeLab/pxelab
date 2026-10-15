//go:build windows

package osimage

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// wim*.go 主体用 DISM 的 PowerShell cmdlet（Mount-WindowsImage / Add-WindowsDriver /
// Dismount-WindowsImage / Get-WindowsDriver / Get-WindowsImage）而非 dism.exe 文本
// 输出：ConvertTo-Json 给出结构化字段名，不受系统显示语言本地化影响（dism.exe
// 的 "Index :"/“索引 :” 随系统语言变化，文本解析不可靠）。CleanupWIM 例外 ——
// Dism 模块没有对应 cmdlet，直接用 dism.exe /Cleanup-Wim（无需输出解析）。

// psQuote 把字符串嵌入 PowerShell 单引号字面量（' → ”）。
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// runPS 执行 PowerShell 脚本并返回 stdout。$ErrorActionPreference='Stop' 让
// cmdlet 错误以非零退出码结束（与 engine.go 的 extractWindowsISO 同一约定）；
// 出错时错误信息附带 stderr（避免静默失败）。
func runPS(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"$ProgressPreference='SilentlyContinue'; "+script)
	hideWindow(cmd)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = strings.TrimSpace(out.String())
		}
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("wim: powershell failed: %s", msg)
	}
	return out.String(), nil
}

// MountWIM 把 WIM 的第 index 个镜像挂载到 mountDir（须为已存在的空目录，
// 由调用方创建）。挂载需要管理员权限。
func MountWIM(wimPath, mountDir string, index int) error {
	_, err := runPS(fmt.Sprintf(
		`$ErrorActionPreference='Stop'; Mount-WindowsImage -ImagePath %s -Index %d -Path %s`,
		psQuote(wimPath), index, psQuote(mountDir)))
	return err
}

// AddDriverToWIM 向已挂载的离线镜像注入 driverDir 下全部 .inf 驱动；
// recurse 为 true 时包含子目录（驱动包常按 x64/、x86/ 分目录存放）。
func AddDriverToWIM(mountDir, driverDir string, recurse bool) error {
	rec := ""
	if recurse {
		rec = " -Recurse"
	}
	_, err := runPS(fmt.Sprintf(
		`$ErrorActionPreference='Stop'; Add-WindowsDriver -Path %s -Driver %s%s`,
		psQuote(mountDir), psQuote(driverDir), rec))
	return err
}

// UnmountWIM 卸载已挂载的镜像；commit=true 提交改动（写回 WIM），false 丢弃。
func UnmountWIM(mountDir string, commit bool) error {
	flag := "-Discard"
	if commit {
		flag = "-Commit"
	}
	_, err := runPS(fmt.Sprintf(
		`$ErrorActionPreference='Stop'; Dismount-WindowsImage -Path %s %s`,
		psQuote(mountDir), flag))
	return err
}

// ListWIMDrivers 列出离线镜像中的第三方（OEM）驱动 —— 注入结果的验证手段
// （借鉴项 5）。需镜像仍处于挂载状态。
func ListWIMDrivers(mountDir string) ([]WIMDriver, error) {
	out, err := runPS(fmt.Sprintf(
		`$ErrorActionPreference='Stop'; Get-WindowsDriver -Path %s | Select-Object ProviderName,ClassName,Driver,Version,BootCritical | ConvertTo-Json -Compress`,
		psQuote(mountDir)))
	if err != nil {
		return nil, err
	}
	return parseWIMDrivers(out)
}

// CleanupWIM 清理损坏的 WIM 挂载点数据库（dism /Cleanup-Wim），防止失败
// 操作后残留幽灵挂载点。无需输出解析（只看退出码），直接用 dism.exe ——
// PowerShell Dism 模块没有对应的 Clear-WindowsCorruptMountPointDatabase cmdlet
// （Win10/11/Server 实测不存在）。建议每次挂载操作后 defer 调用。
func CleanupWIM() error {
	cmd := exec.Command("dism", "/English", "/Cleanup-Wim")
	hideWindow(cmd)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("wim: dism /Cleanup-Wim failed: %s", msg)
	}
	return nil
}

// GetWIMInfo 列出 WIM 内全部镜像索引（dism /Get-WimInfo 的等价 cmdlet，
// 借鉴项 4：注入 index 参数的选项来源）。只读元数据，不挂载。
func GetWIMInfo(wimPath string) ([]WIMIndex, error) {
	out, err := runPS(fmt.Sprintf(
		`$ErrorActionPreference='Stop'; Get-WindowsImage -ImagePath %s | Select-Object ImageIndex,ImageName,ImageDescription,ImageSize | ConvertTo-Json -Compress`,
		psQuote(wimPath)))
	if err != nil {
		return nil, err
	}
	return parseWIMIndexes(out)
}
