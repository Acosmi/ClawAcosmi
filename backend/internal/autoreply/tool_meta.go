package autoreply

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TS 对照: auto-reply/tool-meta.ts (144L)

// ToolAggregate 工具使用汇总。
type ToolAggregate struct {
	Name   string
	Count  int
	Prefix string
}

// FormatToolAggregate 格式化工具使用汇总。
// TS 对照: tool-meta.ts L20-50
func FormatToolAggregate(tools []ToolAggregate) string {
	if len(tools) == 0 {
		return ""
	}
	var parts []string
	for _, t := range tools {
		if t.Count <= 1 {
			parts = append(parts, t.Name)
		} else {
			parts = append(parts, fmt.Sprintf("%s×%d", t.Name, t.Count))
		}
	}
	return strings.Join(parts, ", ")
}

// ShortenToolPath 缩短工具路径用于显示。
// TS 对照: tool-meta.ts L60-90
func ShortenToolPath(toolPath string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 40
	}
	if len(toolPath) <= maxLen {
		return toolPath
	}
	// 尝试仅显示文件名
	base := filepath.Base(toolPath)
	if len(base) <= maxLen {
		return "…/" + base
	}
	// 截断到最大长度
	return toolPath[:maxLen-1] + "…"
}

// FormatToolPrefix 格式化工具前缀标签。
// TS 对照: tool-meta.ts L100-130
func FormatToolPrefix(toolName string) string {
	if toolName == "" {
		return ""
	}
	return fmt.Sprintf("[%s] ", toolName)
}
