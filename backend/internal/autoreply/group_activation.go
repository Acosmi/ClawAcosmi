package autoreply

import "strings"

// TS 对照: auto-reply/group-activation.ts

// GroupActivationMode 群组激活模式。
type GroupActivationMode string

const (
	GroupActivationMention GroupActivationMode = "mention"
	GroupActivationAlways  GroupActivationMode = "always"
)

// NormalizeGroupActivation 规范化群组激活模式。
// TS 对照: group-activation.ts L5-14
func NormalizeGroupActivation(raw string) (GroupActivationMode, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "mention":
		return GroupActivationMention, true
	case "always":
		return GroupActivationAlways, true
	}
	return "", false
}

// ParseActivationCommand 解析 /activation 命令。
// TS 对照: group-activation.ts L16-34
func ParseActivationCommand(raw string) (hasCommand bool, mode GroupActivationMode) {
	if raw == "" {
		return false, ""
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false, ""
	}
	normalized := NormalizeCommandBody(trimmed, nil)
	// 简单匹配 /activation [mode]
	lower := strings.ToLower(normalized)
	if !strings.HasPrefix(lower, "/activation") {
		return false, ""
	}
	rest := strings.TrimSpace(normalized[len("/activation"):])
	if rest != "" && !isAlpha(rest) {
		return false, ""
	}
	// 确保是完整命令（无多余内容）
	parts := strings.Fields(normalized)
	if len(parts) > 2 {
		return false, ""
	}
	if len(parts) == 1 {
		m, _ := NormalizeGroupActivation(rest)
		return true, m
	}
	m, _ := NormalizeGroupActivation(parts[1])
	return true, m
}

// isAlpha 判断字符串是否仅包含字母。
func isAlpha(s string) bool {
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return false
		}
	}
	return true
}
