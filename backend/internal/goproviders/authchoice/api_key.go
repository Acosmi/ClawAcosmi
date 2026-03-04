// authchoice/api_key.go — API Key 输入规范化工具
// 对应 TS 文件: src/commands/auth-choice.api-key.ts
package authchoice

import "strings"

// defaultKeyPreviewHead 预览显示的头部字符数（默认值）。
const defaultKeyPreviewHead = 4

// defaultKeyPreviewTail 预览显示的尾部字符数（默认值）。
const defaultKeyPreviewTail = 4

// NormalizeApiKeyInput 规范化 API Key 输入。
// 处理 shell 风格赋值语句（如 export KEY="value"）、引号和尾部分号。
// 对应 TS: normalizeApiKeyInput()
func NormalizeApiKeyInput(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	// 处理 shell 风格赋值: export KEY="value" 或 KEY=value
	valuePart := trimmed
	// 匹配 export? IDENTIFIER = value 模式
	if idx := findAssignmentValue(trimmed); idx >= 0 {
		valuePart = strings.TrimSpace(trimmed[idx:])
	}

	// 去除引号
	unquoted := stripQuotes(valuePart)

	// 去除尾部分号
	if strings.HasSuffix(unquoted, ";") {
		unquoted = unquoted[:len(unquoted)-1]
	}

	return strings.TrimSpace(unquoted)
}

// findAssignmentValue 查找赋值语句中值部分的起始索引。
// 返回 '=' 后第一个非空白字符的索引，如果不是赋值语句则返回 -1。
func findAssignmentValue(s string) int {
	rest := s
	// 跳过可选的 "export "
	if strings.HasPrefix(rest, "export") && len(rest) > 6 && (rest[6] == ' ' || rest[6] == '\t') {
		rest = strings.TrimSpace(rest[6:])
	}
	// 检查标识符部分
	i := 0
	if i < len(rest) && isIdentStart(rest[i]) {
		i++
		for i < len(rest) && isIdentContinue(rest[i]) {
			i++
		}
		// 跳过空白到 '='
		j := i
		for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
			j++
		}
		if j < len(rest) && rest[j] == '=' {
			offset := len(s) - len(rest)
			return offset + j + 1
		}
	}
	return -1
}

func isIdentStart(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_'
}

func isIdentContinue(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

// stripQuotes 去除首尾成对的引号（双引号、单引号或反引号）。
func stripQuotes(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') ||
			(first == '\'' && last == '\'') ||
			(first == '`' && last == '`') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// ValidateApiKeyInput 验证 API Key 输入（规范化后非空即有效）。
// 对应 TS: validateApiKeyInput
func ValidateApiKeyInput(value string) string {
	if NormalizeApiKeyInput(value) != "" {
		return ""
	}
	return "Required"
}

// FormatApiKeyPreview 格式化 API Key 预览显示。
// 显示头部和尾部部分字符，中间用 "…" 替代。
// 对应 TS: formatApiKeyPreview()
func FormatApiKeyPreview(raw string, head, tail int) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "…"
	}
	if head <= 0 {
		head = defaultKeyPreviewHead
	}
	if tail <= 0 {
		tail = defaultKeyPreviewTail
	}
	if len(trimmed) <= head+tail {
		shortHead := min(2, len(trimmed))
		shortTail := min(2, len(trimmed)-shortHead)
		if shortTail <= 0 {
			return trimmed[:shortHead] + "…"
		}
		return trimmed[:shortHead] + "…" + trimmed[len(trimmed)-shortTail:]
	}
	return trimmed[:head] + "…" + trimmed[len(trimmed)-tail:]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
