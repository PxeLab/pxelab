package baseline

import (
	"strings"
	"text/template"
)

// RenderScript 渲染脚本内容，将模板变量 {{VAR_NAME}} 替换为最终值。
// 变量按优先级合并（高 → 低）：extraVars > profileVars > baselineVars。
func RenderScript(content string, baselineVars, profileVars, extraVars map[string]string) (string, error) {
	merged := MergeVars(baselineVars, profileVars, extraVars)

	tpl, err := template.New("script").Option("missingkey=zero").Parse(content)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tpl.Execute(&buf, merged); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// MergeVars 合并多层变量，返回最终 map。
// 优先级（高 → 低）：extra > profile > baseline
func MergeVars(baselineVars, profileVars, extraVars map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range baselineVars {
		merged[k] = v
	}
	if profileVars != nil {
		for k, v := range profileVars {
			merged[k] = v
		}
	}
	if extraVars != nil {
		for k, v := range extraVars {
			merged[k] = v
		}
	}
	return merged
}
