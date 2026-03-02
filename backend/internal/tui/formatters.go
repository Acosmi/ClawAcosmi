// formatters.go — TUI 消息格式化器
//
// 对齐 TS: src/tui/tui-formatters.ts(220L) — 差异 F-01 (P0)
// 9 个纯函数，无副作用，无隐藏依赖。
package tui

import (
	"fmt"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/agents/helpers"
	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// ResolveFinalAssistantText 选择最终助手输出文本。
// 优先 finalText → streamedText → "(no output)"。
// TS 参考: tui-formatters.ts L4-17
func ResolveFinalAssistantText(finalText, streamedText string) string {
	if strings.TrimSpace(finalText) != "" {
		return finalText
	}
	if strings.TrimSpace(streamedText) != "" {
		return streamedText
	}
	return "(no output)"
}

// ComposeThinkingAndContent 组合 thinking + content 文本。
// showThinking=true 时在 thinking 前加 "[thinking]\n" 前缀。
// TS 参考: tui-formatters.ts L19-36
func ComposeThinkingAndContent(thinkingText, contentText string, showThinking bool) string {
	trimThinking := strings.TrimSpace(thinkingText)
	trimContent := strings.TrimSpace(contentText)

	var parts []string
	if showThinking && trimThinking != "" {
		parts = append(parts, "[thinking]\n"+trimThinking)
	}
	if trimContent != "" {
		parts = append(parts, trimContent)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// ExtractThinkingFromMessage 从消息中提取 thinking 块。
// 遍历 content 数组找 type=thinking 的块。
// TS 参考: tui-formatters.ts L42-66
func ExtractThinkingFromMessage(message interface{}) string {
	if message == nil {
		return ""
	}
	record, ok := message.(map[string]interface{})
	if !ok {
		return ""
	}
	content := record["content"]
	// string content 无 thinking 块
	if _, ok := content.(string); ok {
		return ""
	}
	arr, ok := content.([]interface{})
	if !ok {
		return ""
	}

	var parts []string
	for _, block := range arr {
		rec, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if rec["type"] == "thinking" {
			if thinking, ok := rec["thinking"].(string); ok {
				parts = append(parts, thinking)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// ExtractContentFromMessage 从消息中提取文本内容（不含 thinking）。
// 遍历 content 数组找 type=text 的块，error fallback。
// TS 参考: tui-formatters.ts L72-114
func ExtractContentFromMessage(message interface{}) string {
	if message == nil {
		return ""
	}
	record, ok := message.(map[string]interface{})
	if !ok {
		return ""
	}
	content := record["content"]

	// string content 直接返回
	if s, ok := content.(string); ok {
		return strings.TrimSpace(s)
	}

	// 非数组时检查 error
	arr, ok := content.([]interface{})
	if !ok {
		if stopReason, _ := record["stopReason"].(string); stopReason == "error" {
			errorMessage, _ := record["errorMessage"].(string)
			return helpers.FormatRawAssistantErrorForUi(errorMessage)
		}
		return ""
	}

	// 遍历 text 块
	var parts []string
	for _, block := range arr {
		rec, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if rec["type"] == "text" {
			if text, ok := rec["text"].(string); ok {
				parts = append(parts, text)
			}
		}
	}

	// 无 text 块时检查 error
	if len(parts) == 0 {
		if stopReason, _ := record["stopReason"].(string); stopReason == "error" {
			errorMessage, _ := record["errorMessage"].(string)
			return helpers.FormatRawAssistantErrorForUi(errorMessage)
		}
	}

	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// extractTextBlocks 从 content 中提取文本块（内部辅助）。
// TS 参考: tui-formatters.ts L116-149
func extractTextBlocks(content interface{}, includeThinking bool) string {
	if s, ok := content.(string); ok {
		return strings.TrimSpace(s)
	}
	arr, ok := content.([]interface{})
	if !ok {
		return ""
	}

	var thinkingParts, textParts []string
	for _, block := range arr {
		rec, ok := block.(map[string]interface{})
		if !ok {
			continue
		}
		if rec["type"] == "text" {
			if text, ok := rec["text"].(string); ok {
				textParts = append(textParts, text)
			}
		}
		if includeThinking && rec["type"] == "thinking" {
			if thinking, ok := rec["thinking"].(string); ok {
				thinkingParts = append(thinkingParts, thinking)
			}
		}
	}

	return ComposeThinkingAndContent(
		strings.TrimSpace(strings.Join(thinkingParts, "\n")),
		strings.TrimSpace(strings.Join(textParts, "\n")),
		includeThinking,
	)
}

// ExtractTextFromMessage 统一提取消息文本入口。
// includeThinking=true 时包含 thinking 块。
// TS 参考: tui-formatters.ts L151-171
func ExtractTextFromMessage(message interface{}, includeThinking bool) string {
	if message == nil {
		return ""
	}
	record, ok := message.(map[string]interface{})
	if !ok {
		return ""
	}
	text := extractTextBlocks(record["content"], includeThinking)
	if text != "" {
		return text
	}

	stopReason, _ := record["stopReason"].(string)
	if stopReason != "error" {
		return ""
	}
	errorMessage, _ := record["errorMessage"].(string)
	return helpers.FormatRawAssistantErrorForUi(errorMessage)
}

// IsCommandMessage 检查消息是否为 command 消息。
// TS 参考: tui-formatters.ts L173-178
func IsCommandMessage(message interface{}) bool {
	if message == nil {
		return false
	}
	record, ok := message.(map[string]interface{})
	if !ok {
		return false
	}
	cmd, ok := record["command"].(bool)
	return ok && cmd
}

// FormatTokens 格式化 token 使用量（简短格式）。
// 输出: "tokens 1.2k/8k (15%)" 或 "tokens 1.2k" 或 "tokens ?"
// TS 参考: tui-formatters.ts L180-193
func FormatTokens(total, context *int) string {
	if total == nil && context == nil {
		return "tokens ?"
	}
	totalLabel := "?"
	if total != nil {
		totalLabel = autoreply.FormatTokenCount(*total)
	}
	if context == nil {
		return fmt.Sprintf("tokens %s", totalLabel)
	}
	var pctStr string
	if total != nil && *context > 0 {
		pct := (*total * 100) / *context
		if pct > 999 {
			pct = 999
		}
		pctStr = fmt.Sprintf(" (%d%%)", pct)
	}
	return fmt.Sprintf("tokens %s/%s%s", totalLabel, autoreply.FormatTokenCount(*context), pctStr)
}

// FormatContextUsageLine 格式化完整 context usage 行。
// 输出: "tokens 1.2k/8k (1.5k left, 15%)"
// TS 参考: tui-formatters.ts L195-209
func FormatContextUsageLine(total, context, remaining, percent *int) string {
	totalLabel := "?"
	if total != nil {
		totalLabel = autoreply.FormatTokenCount(*total)
	}
	ctxLabel := "?"
	if context != nil {
		ctxLabel = autoreply.FormatTokenCount(*context)
	}

	var extras []string
	if remaining != nil {
		extras = append(extras, fmt.Sprintf("%s left", autoreply.FormatTokenCount(*remaining)))
	}
	if percent != nil {
		pct := *percent
		if pct > 999 {
			pct = 999
		}
		extras = append(extras, fmt.Sprintf("%d%%", pct))
	}

	extra := ""
	if len(extras) > 0 {
		extra = fmt.Sprintf(" (%s)", strings.Join(extras, ", "))
	}
	return fmt.Sprintf("tokens %s/%s%s", totalLabel, ctxLabel, extra)
}

// AsString 类型安全地将 any 转为 string。
// TS 参考: tui-formatters.ts L211-219
func AsString(value interface{}, fallback string) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fallback
	}
}
