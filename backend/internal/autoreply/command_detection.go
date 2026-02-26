package autoreply

import "strings"

// TS 对照: auto-reply/command-detection.ts (89L)

// HasControlCommand 判断文本是否包含控制命令。
// TS 对照: command-detection.ts L5-30
func HasControlCommand(text string) bool {
	if text == "" {
		return false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	// 快速路径：不以 / 开头则不可能是命令
	if trimmed[0] != '/' {
		return false
	}
	return IsCommandMessage(trimmed, nil)
}

// IsControlCommandMessage 判断文本是否完全是控制命令（无额外内容）。
// TS 对照: command-detection.ts L32-55
func IsControlCommandMessage(text string) bool {
	if text == "" {
		return false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if trimmed[0] != '/' {
		return false
	}
	result := ResolveTextCommand(trimmed, nil)
	return result != nil
}

// InlineCommandToken 内联命令令牌。
type InlineCommandToken struct {
	Name  string
	Value string
}

// HasInlineCommandTokens 判断文本是否包含内联命令令牌 [[token]]。
// TS 对照: command-detection.ts L57-72
func HasInlineCommandTokens(text string) bool {
	return strings.Contains(text, "[[") && strings.Contains(text, "]]")
}

// ShouldComputeCommandAuthorized 判断是否需要计算命令授权。
// TS 对照: command-detection.ts L74-89
func ShouldComputeCommandAuthorized(text string) bool {
	if text == "" {
		return false
	}
	// 如果包含 / 或 [[ 则可能有命令
	return strings.Contains(text, "/") || strings.Contains(text, "[[")
}
