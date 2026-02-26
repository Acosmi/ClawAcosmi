package runner

// ============================================================================
// 会话转录修复
// 对应 TS: session-transcript-repair.ts (319L)
// 核心功能:
//   1. SanitizeToolCallInputs — 移除缺少 input 的 tool_use 块
//   2. SanitizeToolUseResultPairing — 修复 tool_use / tool_result 配对
// ============================================================================

import (
	"github.com/anthropic/open-acosmi/internal/agents/llmclient"
)

// toolCallTypes 工具调用块类型集合。
// TS 对照: TOOL_CALL_TYPES = Set(["toolCall", "toolUse", "functionCall"])
// Go 端 ContentBlock.Type 统一为 "tool_use"，但需兼容历史数据
var toolCallTypes = map[string]bool{
	"tool_use":     true,
	"toolCall":     true,
	"functionCall": true,
}

// ---------- sanitizeToolCallInputs ----------

// SanitizeToolCallInputs 移除 assistant 消息中缺少 input 的 tool call 块。
// 如果移除后 assistant 消息为空，则整个消息被丢弃。
// TS 对照: session-transcript-repair.ts → sanitizeToolCallInputs()
func SanitizeToolCallInputs(messages []llmclient.ChatMessage) []llmclient.ChatMessage {
	changed := false
	out := make([]llmclient.ChatMessage, 0, len(messages))

	for _, msg := range messages {
		if msg.Role != "assistant" || len(msg.Content) == 0 {
			out = append(out, msg)
			continue
		}

		nextContent := make([]llmclient.ContentBlock, 0, len(msg.Content))
		droppedInMsg := 0

		for _, block := range msg.Content {
			if toolCallTypes[block.Type] && !hasToolCallInput(block) {
				droppedInMsg++
				changed = true
				continue
			}
			nextContent = append(nextContent, block)
		}

		if droppedInMsg > 0 {
			if len(nextContent) == 0 {
				// 整个 assistant 消息被清空 → 丢弃
				changed = true
				continue
			}
			out = append(out, llmclient.ChatMessage{
				Role:    msg.Role,
				Content: nextContent,
			})
			continue
		}
		out = append(out, msg)
	}

	if changed {
		return out
	}
	return messages
}

// hasToolCallInput 检查工具调用块是否有有效输入。
func hasToolCallInput(block llmclient.ContentBlock) bool {
	return len(block.Input) > 0
}

// ---------- sanitizeToolUseResultPairing ----------

// SanitizeToolUseResultPairing 修复 tool_use / tool_result 配对关系。
// - 把匹配的 toolResult 移到对应 assistant tool_use 之后
// - 为缺失的 tool_use 插入合成的错误 toolResult
// - 丢弃重复的 toolResult
// TS 对照: session-transcript-repair.ts → sanitizeToolUseResultPairing()
func SanitizeToolUseResultPairing(messages []llmclient.ChatMessage) []llmclient.ChatMessage {
	changed := false
	out := make([]llmclient.ChatMessage, 0, len(messages))
	seenResultIDs := make(map[string]bool)

	i := 0
	for i < len(messages) {
		msg := messages[i]

		if msg.Role != "assistant" {
			// 孤立的 tool_result (不紧跟在 assistant 后面) → 丢弃
			if msg.Role == "tool_result" || isToolResultRole(msg) {
				changed = true
				i++
				continue
			}
			out = append(out, msg)
			i++
			continue
		}

		// 提取 assistant 消息中的 tool call IDs
		toolCalls := extractToolCallIDs(msg)
		if len(toolCalls) == 0 {
			out = append(out, msg)
			i++
			continue
		}

		toolCallIDSet := make(map[string]bool, len(toolCalls))
		for _, tc := range toolCalls {
			toolCallIDSet[tc.id] = true
		}

		// 扫描后续消息，收集匹配的 toolResult
		spanResults := make(map[string]llmclient.ChatMessage)
		var remainder []llmclient.ChatMessage

		j := i + 1
		for j < len(messages) {
			next := messages[j]

			if next.Role == "assistant" {
				break
			}

			if isToolResultRole(next) {
				resultID := extractToolResultID(next)
				if resultID != "" && toolCallIDSet[resultID] {
					if seenResultIDs[resultID] {
						// 重复 → 丢弃
						changed = true
						j++
						continue
					}
					if _, exists := spanResults[resultID]; !exists {
						spanResults[resultID] = next
					}
					j++
					continue
				}
				// 不匹配的 toolResult → 丢弃
				changed = true
				j++
				continue
			}

			remainder = append(remainder, next)
			j++
		}

		// 输出 assistant 消息
		out = append(out, msg)

		// 按 tool call 顺序附加 toolResult (或合成缺失的)
		if len(spanResults) > 0 && len(remainder) > 0 {
			changed = true
		}

		for _, tc := range toolCalls {
			if existing, ok := spanResults[tc.id]; ok {
				if !seenResultIDs[tc.id] {
					seenResultIDs[tc.id] = true
					out = append(out, existing)
				}
			} else {
				// 缺失 → 插入合成的 error toolResult
				synthetic := makeMissingToolResult(tc.id, tc.name)
				changed = true
				seenResultIDs[tc.id] = true
				out = append(out, synthetic)
			}
		}

		// 追加剩余非 toolResult 消息
		for _, rem := range remainder {
			out = append(out, rem)
		}

		i = j
	}

	if changed {
		return out
	}
	return messages
}

// ---------- 辅助类型和函数 ----------

type toolCallRef struct {
	id   string
	name string
}

// extractToolCallIDs 从 assistant 消息的 content 中提取 tool call 信息。
func extractToolCallIDs(msg llmclient.ChatMessage) []toolCallRef {
	var refs []toolCallRef
	for _, block := range msg.Content {
		if toolCallTypes[block.Type] && block.ID != "" {
			refs = append(refs, toolCallRef{id: block.ID, name: block.Name})
		}
	}
	return refs
}

// isToolResultRole 检查消息是否为 tool_result 角色。
func isToolResultRole(msg llmclient.ChatMessage) bool {
	return msg.Role == "tool_result" || msg.Role == "toolResult"
}

// extractToolResultID 从 toolResult 消息中提取对应的 tool call ID。
func extractToolResultID(msg llmclient.ChatMessage) string {
	// 优先使用 content block 中的 ToolUseID
	for _, block := range msg.Content {
		if block.ToolUseID != "" {
			return block.ToolUseID
		}
	}
	return ""
}

// makeMissingToolResult 创建合成的缺失 toolResult。
// TS 对照: session-transcript-repair.ts → makeMissingToolResult()
func makeMissingToolResult(toolCallID, toolName string) llmclient.ChatMessage {
	if toolName == "" {
		toolName = "unknown"
	}
	return llmclient.ChatMessage{
		Role: "tool_result",
		Content: []llmclient.ContentBlock{
			{
				Type:      "text",
				Text:      "[openacosmi] missing tool result in session history; inserted synthetic error result for transcript repair.",
				ToolUseID: toolCallID,
			},
		},
	}
}
