package runner

// ============================================================================
// Reply Directives 解析 + Reasoning 流处理
// TS 对照: auto-reply/reply/reply-directives.ts + handlers.messages.ts
// ============================================================================

import "strings"

// ReplyDirective 回复指令类型。
type ReplyDirective struct {
	Type  string // "auto_reply" | "reasoning" | "stop"
	Value string // 附带值
}

// directivePrefix 指令前缀标记。
const directivePrefix = "[openacosmi:"

// ParseReplyDirectives 从 assistant text 中解析 reply directives。
// TS 对照: reply-directives.ts → parseReplyDirectives()
func ParseReplyDirectives(text string) []ReplyDirective {
	var directives []ReplyDirective
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, directivePrefix) {
			continue
		}
		// 提取指令内容: [openacosmi:TYPE VALUE]
		end := strings.Index(trimmed, "]")
		if end < 0 {
			continue
		}
		inner := trimmed[len(directivePrefix):end]
		parts := strings.SplitN(inner, " ", 2)
		dtype := strings.TrimSpace(parts[0])
		dvalue := ""
		if len(parts) > 1 {
			dvalue = strings.TrimSpace(parts[1])
		}
		directives = append(directives, ReplyDirective{
			Type:  dtype,
			Value: dvalue,
		})
	}
	return directives
}

// StripDirectiveLines 从文本中移除指令行。
func StripDirectiveLines(text string) string {
	lines := strings.Split(text, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, directivePrefix) {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

// ---------- Reasoning 流 ----------

// ReasoningLevel 推理级别。
type ReasoningLevel string

const (
	ReasoningNone    ReasoningLevel = ""
	ReasoningSummary ReasoningLevel = "summary"
	ReasoningFull    ReasoningLevel = "full"
)

// ReasoningChunk 推理流片段。
type ReasoningChunk struct {
	Level ReasoningLevel
	Text  string
}
