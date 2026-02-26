// history.go — 历史消息验证 + Token 截断。
//
// 验证 transcript 消息格式，按 context window 预算截断历史，
// 为 Runner 的 AttemptParams 准备消息列表。
//
// TS 参考: src/gateway/session-utils.fs.ts → readSessionMessages, capArrayByJsonBytes
//
//	src/gateway/session-utils.ts → listSessionsFromStore (token 截断逻辑)
package session

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---------- 常量 ----------

// 合法的消息 role 值。
var validRoles = map[string]bool{
	"user":      true,
	"assistant": true,
	"system":    true,
	"tool":      true,
}

// ---------- PrepareParams ----------

// PrepareParams 准备消息的参数。
type PrepareParams struct {
	SessionID    string // 会话 ID
	SessionFile  string // transcript 文件路径
	StorePath    string // session store 路径
	MaxTokens    int    // context window token 上限
	SystemTokens int    // 系统提示词已占用的 token 数
}

// ---------- ValidateHistoryMessages ----------

// ValidateHistoryMessages 验证并清洗历史消息列表。
// 过滤无效条目（缺少 role、role 非法、content 为空）。
// TS 参考: session-utils.fs.ts → readSessionMessages (隐含的验证逻辑)
func ValidateHistoryMessages(msgs []map[string]interface{}) []map[string]interface{} {
	if len(msgs) == 0 {
		return nil
	}

	result := make([]map[string]interface{}, 0, len(msgs))
	for _, msg := range msgs {
		// 1. 必须有 role 字段
		roleRaw, hasRole := msg["role"]
		if !hasRole {
			continue
		}
		role, ok := roleRaw.(string)
		if !ok || role == "" {
			continue
		}

		// 2. role 必须合法
		if !validRoles[strings.TrimSpace(strings.ToLower(role))] {
			continue
		}

		// 3. 规范化 content 字段
		normalized := normalizeMessageContent(msg)
		if normalized != nil {
			result = append(result, normalized)
		}
	}

	return result
}

// ---------- TruncateByTokenBudget ----------

// TruncateByTokenBudget 按 token 预算截断消息列表。
// 从最新消息向最旧保留，直到达到 token 上限。
// 返回保留的消息列表（保持原始顺序）。
// TS 参考: session-utils.ts capArrayByJsonBytes (按字节截断的变体)
func TruncateByTokenBudget(msgs []map[string]interface{}, maxTokens int) []map[string]interface{} {
	if maxTokens <= 0 || len(msgs) == 0 {
		return msgs
	}

	// 从最新向最旧累加，找到截断点
	usedTokens := 0
	startIdx := len(msgs)
	for i := len(msgs) - 1; i >= 0; i-- {
		msgTokens := EstimateMessageTokens(msgs[i])
		if usedTokens+msgTokens > maxTokens {
			break
		}
		usedTokens += msgTokens
		startIdx = i
	}

	if startIdx >= len(msgs) {
		return nil // 所有消息都超出预算
	}

	return msgs[startIdx:]
}

// ---------- EstimateMessageTokens ----------

// EstimateMessageTokens 估算单条消息的 token 数。
// 将消息序列化为 JSON 后使用 EstimatePromptTokens 估算。
func EstimateMessageTokens(msg map[string]interface{}) int {
	// 优先从 content 字段估算
	if content, ok := msg["content"]; ok {
		switch c := content.(type) {
		case string:
			return EstimatePromptTokens(c) + 4 // +4 for role/metadata overhead
		case []interface{}:
			total := 4 // overhead
			for _, block := range c {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if text, ok := blockMap["text"].(string); ok {
						total += EstimatePromptTokens(text)
					}
				}
			}
			return total
		}
	}

	// 回退：整条消息序列化后估算
	data, err := json.Marshal(msg)
	if err != nil {
		return 50 // 默认保守估计
	}
	return EstimatePromptTokens(string(data))
}

// ---------- PrepareMessagesForAttempt ----------

// PrepareMessagesForAttempt 为 Runner attempt 准备消息列表。
// 管线：读取 → 验证 → 截断 → 返回。
// TS 参考: session-utils.fs.ts readSessionMessages → capArrayByJsonBytes
func PrepareMessagesForAttempt(mgr *SessionManager, params PrepareParams) ([]map[string]interface{}, error) {
	// 1. 读取消息
	msgs, err := mgr.LoadSessionMessages(params.SessionID, params.SessionFile)
	if err != nil {
		return nil, fmt.Errorf("读取 session 消息失败: %w", err)
	}

	if len(msgs) == 0 {
		return nil, nil
	}

	// 2. 验证
	msgs = ValidateHistoryMessages(msgs)
	if len(msgs) == 0 {
		return nil, nil
	}

	// 3. Token 截断
	if params.MaxTokens > 0 {
		// 可用 token 数 = 总上限 - 系统提示词占用
		availableTokens := params.MaxTokens - params.SystemTokens
		if availableTokens <= 0 {
			availableTokens = params.MaxTokens / 2 // 至少给历史一半空间
		}
		msgs = TruncateByTokenBudget(msgs, availableTokens)
	}

	return msgs, nil
}

// ---------- 内部辅助 ----------

// normalizeMessageContent 规范化消息内容字段。
// 确保 content 为 []interface{} 格式（与 LLM API 兼容）。
func normalizeMessageContent(msg map[string]interface{}) map[string]interface{} {
	content, hasContent := msg["content"]
	if !hasContent {
		// 检查是否有 text 字段（某些旧格式）
		if text, ok := msg["text"].(string); ok && text != "" {
			normalized := make(map[string]interface{}, len(msg))
			for k, v := range msg {
				normalized[k] = v
			}
			normalized["content"] = []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": text,
				},
			}
			delete(normalized, "text")
			return normalized
		}
		return nil
	}

	switch c := content.(type) {
	case string:
		// 字符串 → content block 数组
		if strings.TrimSpace(c) == "" {
			return nil
		}
		normalized := make(map[string]interface{}, len(msg))
		for k, v := range msg {
			normalized[k] = v
		}
		normalized["content"] = []interface{}{
			map[string]interface{}{
				"type": "text",
				"text": c,
			},
		}
		return normalized

	case []interface{}:
		// 已是数组格式，检查非空
		if len(c) == 0 {
			return nil
		}
		return msg

	default:
		// 未知格式，保留原样
		return msg
	}
}
