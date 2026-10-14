package netboot

import (
	"fmt"
	"strings"

	"github.com/pxelab/pxelab/internal/models"
)

// VersionOverrideApply applies overlay version overrides to a Version.
// Returns a new Version (does not modify the original).
func VersionOverrideApply(v *Version, ov models.VersionOverride) *Version {
	cloned := *v

	if ov.Enabled != nil {
		cloned.Enabled = *ov.Enabled
	}
	if ov.RemoteKernel != "" || ov.RemoteInitrd != "" {
		if cloned.Remote == nil {
			cloned.Remote = &FileRef{}
		}
		if ov.RemoteKernel != "" {
			cloned.Remote.Kernel = ov.RemoteKernel
		}
		if ov.RemoteInitrd != "" {
			cloned.Remote.Initrd = ov.RemoteInitrd
		}
	}
	if ov.Cmdline != "" {
		cloned.Cmdline = ov.Cmdline
	}
	if ov.AnswerParam != "" {
		cloned.AnswerParam = ov.AnswerParam
	}

	return &cloned
}

// DistroMergeOverlay merges DB overlay fields into a Distro.
// Returns a new Distro (does not modify the original).
// Version-level overrides from the overlay are applied on top.
func DistroMergeOverlay(distro *Distro, overlay *models.NetbootOverlay) *Distro {
	if overlay == nil {
		return distro
	}

	cloned := *distro

	if overlay.Mirror != "" {
		cloned.Mirror = overlay.Mirror
	}
	if overlay.LocalBase != "" {
		cloned.LocalBase = overlay.LocalBase
	}
	if overlay.KernelParams != "" {
		if cloned.KernelParams != "" {
			cloned.KernelParams = cloned.KernelParams + " " + overlay.KernelParams
		} else {
			cloned.KernelParams = overlay.KernelParams
		}
	}

	// Apply version-level overrides
	ovs, err := overlay.GetVersionOverrides()
	if err != nil || len(ovs) == 0 {
		return &cloned
	}

	// Build lookup map keyed by "codename:arch"
	overrideMap := make(map[string]models.VersionOverride)
	for _, ov := range ovs {
		key := ov.Codename + ":" + ov.Arch
		overrideMap[key] = ov
	}

	for i, v := range cloned.Versions {
		key := v.Codename + ":" + v.Arch
		if ov, ok := overrideMap[key]; ok {
			cloned.Versions[i] = VersionOverrideApply(v, ov)
		}
	}

	return &cloned
}

// BootTaskInfo carries install task information for boot line generation.
type BootTaskInfo struct {
	ID           string
	AnswerURL    string // pre-computed: http://server/api/v1/netboot/answer/<task_id>
	AnswerParam  string // from version overlay (e.g. "autoinstall ds=nocloud-net;s={{.AnswerURL}}")
	AnswerType   string // "subiquity", "kickstart", "preseed", "autoyast", "autounattend", "winpeshl"
	ExtraCmdline string // additional kernel parameters from the task
	WIMIndex     int    // >0 选择 boot.wim 镜像索引（多映像介质），追加到 wimboot kernel 行
}

// BuildBootTaskInfo 由安装任务 + 其发行版 overlay 组装引导注入信息。AnswerURL 始终
// 计算，以便 catalog 级 answer_param 回落（overlay 未配置时）也能注入应答文件 URL。
// answer_type 独立于 answer_param 解析：winpeshl 类 overlay 往往只配 answer_type 而
// 无 answer_param，也需要把 AnswerType 传给 wimboot 引导行。
func BuildBootTaskInfo(task *models.InstallTask, overlay *models.NetbootOverlay, serverAddr string) *BootTaskInfo {
	info := &BootTaskInfo{
		ID:           task.ID,
		AnswerURL:    fmt.Sprintf("http://%s/api/v1/netboot/answer/%s", serverAddr, task.ID),
		ExtraCmdline: task.ExtraCmdline,
		WIMIndex:     task.WIMIndex,
	}
	if overlay == nil {
		return info
	}
	ovs, _ := overlay.GetVersionOverrides()
	for _, ov := range ovs {
		if ov.Codename != task.VersionCodename {
			continue
		}
		if ov.AnswerParam != "" {
			info.AnswerParam = ov.AnswerParam
		}
		if ov.AnswerType != "" {
			info.AnswerType = ov.AnswerType
		}
	}
	return info
}

// InjectAnswerParam replaces {{.AnswerURL}} placeholder with the actual URL and returns the final parameter string.
func InjectAnswerParam(answerParam, answerURL string) string {
	if answerParam == "" || answerURL == "" {
		return ""
	}
	// Simple string replacement; template engine not needed since we only have one variable
	result := strings.ReplaceAll(answerParam, "{{.AnswerURL}}", answerURL)
	return strings.TrimSpace(result)
}
