package api

// Baseline pull exec-bundle + answer-file injection helpers.
//
// 自动应答模板只需要“几条固定命令”就能在安装后把该主机配置的初始化基线拉下来执行。
// 平台侧提供两类可直接执行的聚合产物，供应答钩子引用：
//   GET /api/v1/baselines/pull.sh  — POSIX sh，逐个执行 type=shell 的脚本
//   GET /api/v1/baselines/pull.ps1 — PowerShell，逐个执行 type=powershell 的脚本
// 身份按 global.identity_attr 取 mac 或 sn；无配置一律空产物（exit 0），不影响安装。
// 两个产物末尾都会追加一条"装机完成"回报（POST /install-tasks/report-by-mac，R7）。

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pxelab/pxelab/internal/config"
)

const baselinePullMarker = "pxelab-baseline-pull"

// installReportMarker 标记聚合产物末尾的"装机完成"回报段（R7）。
const installReportMarker = "pxelab-install-report"

// 内置默认钩子模板。{{URL}} 在注入时替换为聚合产物地址（pull.sh / pull.ps1）。
// 与 global.baseline_hooks 配置一一对应；配置留空即用这里的默认值。
const (
	defaultKickstartHook = `%post --interpreter=/bin/bash
# PxeLab baseline pull (pxelab-baseline-pull)
set +e
if command -v curl >/dev/null 2>&1; then curl -fsSL '{{URL}}' | bash
elif command -v wget >/dev/null 2>&1; then wget -qO- '{{URL}}' | bash
fi
%end`

	defaultPreseedHook = `in-target sh -c "if command -v curl >/dev/null 2>&1; then curl -fsSL '{{URL}}' | sh; else wget -qO- '{{URL}}' | sh; fi" || true`

	defaultSubiquityHook = `late-commands:
  - sh -c "curl -fsSL '{{URL}}' | sh || true"`

	defaultAutoUnattendHook = `powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "try { $u='{{URL}}'; (New-Object Net.WebClient).DownloadString($u) | Invoke-Expression } catch {}"`
)

// DefaultBaselineHooks 返回内置默认钩子模板（供设置页展示/重置）。
func DefaultBaselineHooks() config.BaselineHooksConfig {
	return config.BaselineHooksConfig{
		Kickstart:    defaultKickstartHook,
		Preseed:      defaultPreseedHook,
		Subiquity:    defaultSubiquityHook,
		AutoUnattend: defaultAutoUnattendHook,
	}
}

// renderHook 用 url 替换模板中的 {{URL}}；tpl 为空时用 fallback 默认模板。
func renderHook(tpl, fallback, url string) string {
	if strings.TrimSpace(tpl) == "" {
		tpl = fallback
	}
	return strings.ReplaceAll(tpl, "{{URL}}", url)
}

// identityParam 按配置的身份键生成查询参数（mac/sn）。
func (h *BaselineHandler) identityParam(mac, sn string) string {
	if h.identityAttr == "sn" {
		return "sn=" + sn
	}
	return "mac=" + mac
}

// shQuote 把字符串安全嵌入单引号 shell 词（' → '\''）。
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// psQuote 把字符串安全嵌入 PowerShell 单引号字面量（' → ''）。
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// shReportFunc 是 pull.sh 中的上报函数：curl 优先、wget 兜底，任何失败静默。
// 输出尾部先剥离 JSON 不允许的控制字符，再做 JSON 字符串转义、换行拼接为 \n。
const shReportFunc = `_pxelab_report() {
  # 参数：脚本名 序号 退出码 耗时(ms) 日志文件；report 不可达静默失败，不影响后续脚本
  _pxelab_name=$(printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g')
  _pxelab_tail=$(tail -c 4000 "$5" 2>/dev/null | tr '\011' ' ' | tr -d '\000-\010\013-\037\177' | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | awk 'NR>1{printf "\\n"}{printf "%s",$0}')
  _pxelab_body="{\"script_name\":\"$_pxelab_name\",\"seq\":$2,\"exit_code\":$3,\"duration_ms\":$4,\"output_tail\":\"$_pxelab_tail\"}"
  if command -v curl >/dev/null 2>&1; then
    curl -fsS -m 5 -H 'Content-Type: application/json' -d "$_pxelab_body" "$_pxelab_report_url" >/dev/null 2>&1 || true
  elif command -v wget >/dev/null 2>&1; then
    wget -q -T 5 -O /dev/null --header='Content-Type: application/json' --post-data="$_pxelab_body" "$_pxelab_report_url" >/dev/null 2>&1 || true
  fi
}
`

// psReportFunc 是 pull.ps1 中的上报函数：Invoke-RestMethod 优先、WebClient 兜底，任何失败静默。
const psReportFunc = `function Send-PxeLabReport($Name, $Seq, $ExitCode, $DurationMs, $OutputTail) {
  # 上报失败静默，不影响后续脚本
  try {
    $body = @{ script_name = $Name; seq = $Seq; exit_code = $ExitCode; duration_ms = $DurationMs; output_tail = $OutputTail } | ConvertTo-Json -Compress
    try {
      Invoke-RestMethod -Uri $PxeLabReportUrl -Method Post -ContentType 'application/json' -Body $body -TimeoutSec 5 | Out-Null
    } catch {
      (New-Object Net.WebClient).UploadString($PxeLabReportUrl, 'POST', $body) | Out-Null
    }
  } catch {}
}
`

// buildReportURL 生成聚合产物内嵌的 report 地址：用请求方已访问的地址（r.Host），身份参数原样透传。
func buildReportURL(r *http.Request) string {
	return fmt.Sprintf("http://%s/api/v1/baselines/report?%s", r.Host, r.URL.RawQuery)
}

// buildInstallReportURL 生成"装机完成"回报地址（R7）：按 mac/sn 反查活跃任务，身份参数原样透传。
func buildInstallReportURL(r *http.Request) string {
	return fmt.Sprintf("http://%s/api/v1/install-tasks/report-by-mac?%s", r.Host, r.URL.RawQuery)
}

// shInstallReportSnippet 是 pull.sh 末尾的装机完成回报：curl 优先、wget 兜底，失败静默。
const shInstallReportSnippet = `if command -v curl >/dev/null 2>&1; then
  curl -fsS -m 5 -H 'Content-Type: application/json' -d '{"status":"done"}' "$_pxelab_install_report_url" >/dev/null 2>&1 || true
elif command -v wget >/dev/null 2>&1; then
  wget -q -T 5 -O /dev/null --header='Content-Type: application/json' --post-data='{"status":"done"}' "$_pxelab_install_report_url" >/dev/null 2>&1 || true
fi
`

// psInstallReportSnippet 是 pull.ps1 末尾的装机完成回报：失败静默。
const psInstallReportSnippet = `try {
  Invoke-RestMethod -Uri $PxeLabInstallReportUrl -Method Post -ContentType 'application/json' -Body '{"status":"done"}' -TimeoutSec 5 | Out-Null
} catch {}
`

// PullShellScript 输出一段可直接 `| sh` 执行的脚本：按序运行该主机的 shell 基线。
// 每条脚本执行时捕获退出码/耗时/输出尾部，随后向 /baselines/report 上报一次；
// 上报不可达静默失败，单条脚本失败也不中断后续脚本。
func (h *BaselineHandler) PullShellScript(w http.ResponseWriter, r *http.Request) {
	host, found := h.resolveHost(r)
	if !found {
		w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
		w.Write([]byte("#!/bin/sh\nexit 0\n"))
		return
	}
	scripts, err := h.collectScripts(r.Context(), host, "shell")
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# generated by PxeLab " + baselinePullMarker + "\n")
	if len(scripts) > 0 {
		b.WriteString("_pxelab_report_url=" + shQuote(buildReportURL(r)) + "\n")
		b.WriteString(shReportFunc)
	}
	for i, sc := range scripts {
		fmt.Fprintf(&b, "\ncat > /tmp/pxelab-init-%d.sh <<'PXEBL'\n%s\nPXEBL\n", i, sc.Content)
		fmt.Fprintf(&b, "_pxelab_start=$(date +%%s)\nsh /tmp/pxelab-init-%d.sh > /tmp/pxelab-init-%d.log 2>&1\n_pxelab_ec=$?\n", i, i)
		b.WriteString("_pxelab_dur=$(( ($(date +%s) - _pxelab_start) * 1000 ))\n")
		fmt.Fprintf(&b, "[ \"$_pxelab_ec\" -ne 0 ] && echo %s\n", shQuote("[pxelab] script failed (non-fatal): "+sc.Name))
		fmt.Fprintf(&b, "_pxelab_report %s %d \"$_pxelab_ec\" \"$_pxelab_dur\" /tmp/pxelab-init-%d.log\n", shQuote(sc.Name), sc.Seq, i)
	}
	// R7：聚合脚本执行到末尾即装机流程走完，自动回报"装机完成"（无基线脚本也回报）。
	b.WriteString("\n# PxeLab install report (" + installReportMarker + ")\n")
	b.WriteString("_pxelab_install_report_url=" + shQuote(buildInstallReportURL(r)) + "\n")
	b.WriteString(shInstallReportSnippet)
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Write([]byte(b.String()))
}

// PullPowerShellScript 输出一段可直接执行的 PowerShell：按序运行该主机的 powershell 基线。
// 每条脚本 try/catch 捕获退出状态/耗时/输出尾部后上报，失败静默不中断。
func (h *BaselineHandler) PullPowerShellScript(w http.ResponseWriter, r *http.Request) {
	host, found := h.resolveHost(r)
	if !found {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("# no baseline\n"))
		return
	}
	scripts, err := h.collectScripts(r.Context(), host, "powershell")
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	var b strings.Builder
	b.WriteString("# generated by PxeLab " + baselinePullMarker + "\n")
	if len(scripts) > 0 {
		b.WriteString("$PxeLabReportUrl = " + psQuote(buildReportURL(r)) + "\n")
		b.WriteString(psReportFunc)
	}
	for _, sc := range scripts {
		fmt.Fprintf(&b, "\n$_pxelabStart = Get-Date\n$_pxelabEc = 0\n$_pxelabOut = ''\n$_pxelabLines = @()\n")
		fmt.Fprintf(&b, "try {\n# ---- %s ----\n& {\n%s\n} 2>&1 | Tee-Object -Variable _pxelabLines\n$_pxelabOut = ($_pxelabLines | Out-String)\n", sc.Name, sc.Content)
		b.WriteString("} catch {\n$_pxelabEc = 1\n$_pxelabOut = \"$_pxelabOut\" + $_.Exception.Message\nWrite-Warning ('[pxelab] script failed (non-fatal): ' + $_.Exception.Message)\n}\n")
		b.WriteString("$_pxelabDur = [int]((Get-Date) - $_pxelabStart).TotalMilliseconds\nif ($_pxelabOut.Length -gt 4000) { $_pxelabOut = $_pxelabOut.Substring($_pxelabOut.Length - 4000) }\n")
		fmt.Fprintf(&b, "Send-PxeLabReport %s %d $_pxelabEc $_pxelabDur $_pxelabOut\n", psQuote(sc.Name), sc.Seq)
	}
	// R7：聚合脚本执行到末尾即装机流程走完，自动回报"装机完成"（无基线脚本也回报）。
	b.WriteString("\n# PxeLab install report (" + installReportMarker + ")\n")
	b.WriteString("$PxeLabInstallReportUrl = " + psQuote(buildInstallReportURL(r)) + "\n")
	b.WriteString(psInstallReportSnippet)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(b.String()))
}

// buildPullURL 生成应答模板注入时使用的聚合产物 URL。
func buildPullURL(baseURL, execExt, identityParam string) string {
	return fmt.Sprintf("http://%s/api/v1/baselines/pull.%s?%s", baseURL, execExt, identityParam)
}

// augmentBaselinePull 在渲染后的应答文件上追加/合并“拉取并执行基线”的钩子。
// 返回注入后的内容；applied=false 表示该类型暂时无法安全注入（调用方应告警）。
// 幂等：内容已含标记则跳过。
// hooks 为 global.baseline_hooks 自定义模板，空字段回落到内置默认。
func augmentBaselinePull(content, answerType, baseURL, mac, sn string, hooks config.BaselineHooksConfig) (string, bool) {
	if strings.Contains(content, baselinePullMarker) {
		return content, true
	}
	// 同时带上可用身份键；/baselines/pull.{sh,ps1} 按 global.identity_attr 取用其一
	parts := []string{}
	if mac != "" {
		parts = append(parts, "mac="+mac)
	}
	if sn != "" {
		parts = append(parts, "sn="+sn)
	}
	if len(parts) == 0 {
		return content, false // 无可用身份，跳过注入
	}
	ip := strings.Join(parts, "&")
	shURL := buildPullURL(baseURL, "sh", ip)
	psURL := buildPullURL(baseURL, "ps1", ip)

	switch answerType {
	case "preseed", "subiquity":
		return augmentLinuxPreseed(content, shURL, hooks)
	case "kickstart":
		return augmentKickstart(content, shURL, hooks)
	case "autounattend":
		return augmentAutoUnattend(content, psURL, hooks)
	default:
		return content, false
	}
}

// augmentLinuxPreseed 覆盖 debian-installer preseed 与 Ubuntu subiquity(autoinstall, YAML)。
func augmentLinuxPreseed(content, shURL string, hooks config.BaselineHooksConfig) (string, bool) {
	block := ""
	switch {
	case strings.Contains(content, "d-i preseed/late_command string"):
		// 已有 late_command：合并为一条链（preseed 同一 key 重复时后者覆盖前者）
		lines := strings.Split(content, "\n")
		var kept []string
		var existing []string
		for _, ln := range lines {
			trim := strings.TrimSpace(ln)
			if strings.HasPrefix(trim, "d-i preseed/late_command string") {
				existing = append(existing, strings.TrimSpace(strings.TrimPrefix(trim, "d-i preseed/late_command string")))
				continue
			}
			kept = append(kept, ln)
		}
		all := append(existing, renderHook(hooks.Preseed, defaultPreseedHook, shURL))
		block = "d-i preseed/late_command string " + strings.Join(all, " && ")
		content = strings.Join(kept, "\n")
	case strings.Contains(content, "late-commands:"):
		// subiquity：已有 late-commands 列表时无法安全插入，交由人工确认（返回未注入）
		return content, false
	default:
		block = renderHook(hooks.Subiquity, defaultSubiquityHook, shURL)
	}
	return content + "\n# PxeLab baseline pull (" + baselinePullMarker + ")\n" + block + "\n", true
}

// augmentKickstart 在末尾追加 %post 段（anaconda 允许多个 %post）。
func augmentKickstart(content, shURL string, hooks config.BaselineHooksConfig) (string, bool) {
	return content + "\n" + renderHook(hooks.Kickstart, defaultKickstartHook, shURL) + "\n", true
}

// augmentAutoUnattend 在 oobeSystem 的 Microsoft-Windows-Shell-Setup 组件里注入 FirstLogonCommands。
func augmentAutoUnattend(content, psURL string, hooks config.BaselineHooksConfig) (string, bool) {
	if !strings.Contains(content, "oobeSystem") || !strings.Contains(content, "Microsoft-Windows-Shell-Setup") {
		return content, false
	}
	if strings.Contains(content, "<FirstLogonCommands") {
		return content, false // 已有 FirstLogonCommands：请人工合并
	}
	// 模板渲染后再做 XML 转义
	cmd := xmlEscape(renderHook(hooks.AutoUnattend, defaultAutoUnattendHook, psURL))
	snippet := "\n<FirstLogonCommands>\n<SynchronousCommand wcm:action=\"add\">\n<CommandLine>" + cmd +
		"</CommandLine>\n<Description>PxeLab baseline pull</Description>\n<Order>1</Order>\n</SynchronousCommand>\n</FirstLogonCommands>"

	// 定位组件开标签结束处，紧跟其后插入
	idx := strings.Index(content, `<component name="Microsoft-Windows-Shell-Setup"`)
	if idx < 0 {
		return content, false
	}
	gt := strings.Index(content[idx:], ">")
	if gt < 0 {
		return content, false
	}
	pos := idx + gt + 1
	return content[:pos] + snippet + content[pos:], true
}

// augmentAutoUnattendFirstLogon 是 augmentAutoUnattend 的编排版（R6）：在同一个
// FirstLogonCommands 里先排驱动安装命令（Order 1..N），再排基线拉取（Order N+1），
// 保证 Windows 首登时先装驱动、再执行基线脚本。
// 仅 GetAnswerFile 的 autounattend 分支使用；driverCmds 为空且 wantBaseline=true 时
// 产出与 augmentAutoUnattend 完全一致。wantBaseline=false 时 psURL 忽略。
// 幂等：内容已含基线/驱动标记则原样返回 applied=true。
func augmentAutoUnattendFirstLogon(content, psURL string, driverCmds []driverCommand, wantBaseline bool, hooks config.BaselineHooksConfig) (string, bool) {
	if strings.Contains(content, baselinePullMarker) || strings.Contains(content, driverPullMarker) {
		return content, true
	}
	if !strings.Contains(content, "oobeSystem") || !strings.Contains(content, "Microsoft-Windows-Shell-Setup") {
		return content, false
	}
	if strings.Contains(content, "<FirstLogonCommands") {
		return content, false // 已有 FirstLogonCommands：请人工合并
	}
	if len(driverCmds) == 0 && !wantBaseline {
		return content, false // 无事可排
	}

	var b strings.Builder
	b.WriteString("\n<FirstLogonCommands>")
	order := 1
	for _, dc := range driverCmds {
		b.WriteString("\n<SynchronousCommand wcm:action=\"add\">\n<CommandLine>" + xmlEscape(dc.cmd) +
			"</CommandLine>\n<Description>PxeLab driver install (" + driverPullMarker + "): " + xmlEscape(dc.name) +
			"</Description>\n<Order>" + strconv.Itoa(order) + "</Order>\n</SynchronousCommand>")
		order++
	}
	if wantBaseline {
		// 模板渲染后再做 XML 转义（与 augmentAutoUnattend 同）
		cmd := xmlEscape(renderHook(hooks.AutoUnattend, defaultAutoUnattendHook, psURL))
		b.WriteString("\n<SynchronousCommand wcm:action=\"add\">\n<CommandLine>" + cmd +
			"</CommandLine>\n<Description>PxeLab baseline pull</Description>\n<Order>" + strconv.Itoa(order) + "</Order>\n</SynchronousCommand>")
	}
	b.WriteString("\n</FirstLogonCommands>")
	snippet := b.String()

	// 定位组件开标签结束处，紧跟其后插入（与 augmentAutoUnattend 同）
	idx := strings.Index(content, `<component name="Microsoft-Windows-Shell-Setup"`)
	if idx < 0 {
		return content, false
	}
	gt := strings.Index(content[idx:], ">")
	if gt < 0 {
		return content, false
	}
	pos := idx + gt + 1
	return content[:pos] + snippet + content[pos:], true
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
