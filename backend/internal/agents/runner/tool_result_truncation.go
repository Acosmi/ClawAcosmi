package runner

import (
	"math"
	"strings"
)

// TS 对照: agents/pi-embedded-runner/tool-result-truncation.ts (329L)
// 工具结果截断 — 防止超大工具输出撑爆上下文窗口。

// ---------- 常量 ----------

const (
	// MaxToolResultContextShare 单条工具结果占上下文窗口的最大比例 (30%)。
	MaxToolResultContextShare = 0.3

	// HardMaxToolResultChars 单条工具结果的硬性字符上限。
	// 即使 2M token 窗口，单条工具结果也不应超过 ~400K 字符 (~100K tokens)。
	HardMaxToolResultChars = 400_000

	// MinKeepChars 截断时至少保留的字符数。
	MinKeepChars = 2_000

	// TruncationSuffix 截断后追加的尾部提示。
	TruncationSuffix = "\n\n⚠️ [Content truncated — original was too large for the model's context window. " +
		"The content above is a partial view. If you need more, request specific sections or use " +
		"offset/limit parameters to read smaller chunks.]"
)

// ---------- 消息类型 ----------

// ToolResultContentBlock 工具结果消息中的内容块。
type ToolResultContentBlock struct {
	Type string `json:"type"`           // "text" | "image" | ...
	Text string `json:"text,omitempty"` // type=="text" 时有效
}

// ToolResultMessage 工具结果消息（简化版，用于截断逻辑）。
type ToolResultMessage struct {
	Role    string                   `json:"role"`              // "toolResult"
	Content []ToolResultContentBlock `json:"content,omitempty"` // 内容块列表
}

// ---------- 核心函数 ----------

// TruncateToolResultText 截断单条文本，保留开头部分。
// 优先在换行符处截断以避免截断行中间。
// TS 对照: tool-result-truncation.ts L39-51
func TruncateToolResultText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}

	keepChars := maxChars - len(TruncationSuffix)
	if keepChars < MinKeepChars {
		keepChars = MinKeepChars
	}

	// 优先在换行符处截断
	cutPoint := keepChars
	lastNewline := strings.LastIndex(text[:keepChars], "\n")
	threshold := int(float64(keepChars) * 0.8)
	if lastNewline > threshold {
		cutPoint = lastNewline
	}

	return text[:cutPoint] + TruncationSuffix
}

// CalculateMaxToolResultChars 根据上下文窗口 token 数计算工具结果允许的最大字符数。
// 使用 ~4 chars ≈ 1 token 的粗略启发式。
// TS 对照: tool-result-truncation.ts L60-65
func CalculateMaxToolResultChars(contextWindowTokens int) int {
	maxTokens := int(math.Floor(float64(contextWindowTokens) * MaxToolResultContextShare))
	maxChars := maxTokens * 4
	if maxChars > HardMaxToolResultChars {
		return HardMaxToolResultChars
	}
	return maxChars
}

// GetToolResultTextLength 计算工具结果消息中所有 text 内容块的总字符长度。
// TS 对照: tool-result-truncation.ts L70-88
func GetToolResultTextLength(msg *ToolResultMessage) int {
	if msg == nil || msg.Role != "toolResult" {
		return 0
	}
	total := 0
	for _, block := range msg.Content {
		if block.Type == "text" {
			total += len(block.Text)
		}
	}
	return total
}

// TruncateToolResultMessage 截断工具结果消息中的 text 内容块。
// 按各 text 块占总长度的比例分配截断预算。
// 返回新消息，不修改原始消息。
// TS 对照: tool-result-truncation.ts L94-125
func TruncateToolResultMessage(msg *ToolResultMessage, maxChars int) *ToolResultMessage {
	if msg == nil {
		return nil
	}

	totalTextChars := GetToolResultTextLength(msg)
	if totalTextChars <= maxChars {
		return msg
	}

	// 按比例分配截断预算
	newContent := make([]ToolResultContentBlock, len(msg.Content))
	for i, block := range msg.Content {
		if block.Type != "text" || block.Text == "" {
			newContent[i] = block
			continue
		}

		blockShare := float64(len(block.Text)) / float64(totalTextChars)
		blockBudget := int(math.Floor(float64(maxChars) * blockShare))
		if blockBudget < MinKeepChars {
			blockBudget = MinKeepChars
		}

		newContent[i] = ToolResultContentBlock{
			Type: "text",
			Text: TruncateToolResultText(block.Text, blockBudget),
		}
	}

	return &ToolResultMessage{
		Role:    msg.Role,
		Content: newContent,
	}
}

// ---------- 批量操作 ----------

// TruncateResult 截断操作结果。
type TruncateResult struct {
	Messages       []*ToolResultMessage
	TruncatedCount int
}

// TruncateOversizedToolResultsInMessages 对消息数组做预防性截断。
// 用于发送给 LLM 之前的保护，不修改原始消息。
// TS 对照: tool-result-truncation.ts L272-292
func TruncateOversizedToolResultsInMessages(messages []*ToolResultMessage, contextWindowTokens int) TruncateResult {
	maxChars := CalculateMaxToolResultChars(contextWindowTokens)
	var truncatedCount int

	result := make([]*ToolResultMessage, len(messages))
	for i, msg := range messages {
		if msg == nil || msg.Role != "toolResult" {
			result[i] = msg
			continue
		}
		textLength := GetToolResultTextLength(msg)
		if textLength <= maxChars {
			result[i] = msg
			continue
		}
		truncatedCount++
		result[i] = TruncateToolResultMessage(msg, maxChars)
	}

	return TruncateResult{
		Messages:       result,
		TruncatedCount: truncatedCount,
	}
}

// IsOversizedToolResult 检测单条消息是否超限。
// TS 对照: tool-result-truncation.ts L297-303
func IsOversizedToolResult(msg *ToolResultMessage, contextWindowTokens int) bool {
	if msg == nil || msg.Role != "toolResult" {
		return false
	}
	maxChars := CalculateMaxToolResultChars(contextWindowTokens)
	return GetToolResultTextLength(msg) > maxChars
}

// SessionLikelyHasOversizedToolResults 启发式判断消息列表中是否存在超大工具结果。
// TS 对照: tool-result-truncation.ts L310-328
func SessionLikelyHasOversizedToolResults(messages []*ToolResultMessage, contextWindowTokens int) bool {
	maxChars := CalculateMaxToolResultChars(contextWindowTokens)
	for _, msg := range messages {
		if msg == nil || msg.Role != "toolResult" {
			continue
		}
		if GetToolResultTextLength(msg) > maxChars {
			return true
		}
	}
	return false
}
