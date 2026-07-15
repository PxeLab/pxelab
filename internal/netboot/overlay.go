package netboot

import (
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
	ID          string
	AnswerURL   string // pre-computed: http://server/api/v1/netboot/answer/<task_id>
	AnswerParam string // from version overlay (e.g. "autoinstall ds=nocloud-net;s={{.AnswerURL}}")
	AnswerType  string // "subiquity", "kickstart", "preseed", "autoyast", "autounattend", "wimboot"
	ExtraCmdline string // additional kernel parameters from the task
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
