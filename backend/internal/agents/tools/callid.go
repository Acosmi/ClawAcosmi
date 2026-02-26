// tools/callid.go — 工具调用 ID 管理。
// TS 参考：src/agents/tool-call-id.ts (221L)
package tools

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// ---------- Provider ID 限制 ----------

// IDStrictMode 工具调用 ID 的严格模式。
type IDStrictMode int

const (
	IDModeDefault IDStrictMode = iota
	IDModeStrict               // [a-zA-Z0-9_-]{1,64}（Mistral 要求）
	IDModeStrict9              // [a-zA-Z0-9_]{1,9}（某些 provider）
)

var (
	strictPattern  = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	strict9Pattern = regexp.MustCompile(`^[a-zA-Z0-9_]{1,9}$`)
	// 全局 ID 计数器（防碰撞）
	idCounter     int64
	idCounterLock sync.Mutex
)

// IsValidToolCallID 检查 ID 是否满足指定模式。
func IsValidToolCallID(id string, mode IDStrictMode) bool {
	switch mode {
	case IDModeStrict:
		return strictPattern.MatchString(id)
	case IDModeStrict9:
		return strict9Pattern.MatchString(id)
	default:
		return id != ""
	}
}

// SanitizeToolCallID 清洗工具调用 ID，确保合规。
// TS 参考: tool-call-id.ts sanitizeToolCallId
func SanitizeToolCallID(id string, mode IDStrictMode) string {
	if IsValidToolCallID(id, mode) {
		return id
	}

	switch mode {
	case IDModeStrict:
		return sanitizeForStrict(id)
	case IDModeStrict9:
		return sanitizeForStrict9(id)
	default:
		if id == "" {
			return generateRandomID(16)
		}
		return id
	}
}

// sanitizeForStrict 清洗为 strict 模式（[a-zA-Z0-9_-]{1,64}）。
func sanitizeForStrict(id string) string {
	if id == "" {
		return generateRandomID(16)
	}

	// 替换非法字符为 _
	var sb strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	result := sb.String()

	// 截断到 64 字符
	if len(result) > 64 {
		result = result[:64]
	}
	if result == "" {
		return generateRandomID(16)
	}
	return result
}

// sanitizeForStrict9 清洗为 strict9 模式（[a-zA-Z0-9_]{1,9}）。
func sanitizeForStrict9(id string) string {
	if id == "" {
		return generateRandomID(9)[:9]
	}

	var sb strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
		if sb.Len() >= 9 {
			break
		}
	}
	result := sb.String()
	if result == "" {
		return generateRandomID(9)[:9]
	}
	return result
}

// generateRandomID 生成随机十六进制 ID。
func generateRandomID(length int) string {
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		// fallback
		idCounterLock.Lock()
		idCounter++
		c := idCounter
		idCounterLock.Unlock()
		return fmt.Sprintf("call_%d", c)
	}
	return hex.EncodeToString(bytes)[:length]
}

// MakeUniqueToolID 生成唯一工具调用 ID（防碰撞）。
// TS 参考: tool-call-id.ts makeUniqueToolId
func MakeUniqueToolID(prefix string, existing map[string]bool, mode IDStrictMode) string {
	if prefix == "" {
		prefix = "call"
	}

	for attempt := 0; attempt < 100; attempt++ {
		suffix := generateRandomID(8)
		var id string
		if attempt == 0 {
			id = fmt.Sprintf("%s_%s", prefix, suffix)
		} else {
			id = fmt.Sprintf("%s_%s_%d", prefix, suffix, attempt)
		}
		id = SanitizeToolCallID(id, mode)
		if !existing[id] {
			return id
		}
	}
	// 极端情况 — 使用纯随机
	return generateRandomID(16)
}

// IsValidCloudCodeAssistToolID 检查 ID 是否为有效的 Cloud Code Assist tool call ID。
// TS 参考: tool-call-id.ts isValidCloudCodeAssistToolCallId
func IsValidCloudCodeAssistToolID(id string) bool {
	return IsValidToolCallID(id, IDModeStrict)
}

// SanitizeToolCallIDsInMessages 批量清洗消息列表中的工具调用 ID。
// TS 参考: tool-call-id.ts sanitizeToolCallIdsForCloudCodeAssist
func SanitizeToolCallIDsInMessages(messages []map[string]any, mode IDStrictMode) []map[string]any {
	idMapping := map[string]string{} // old → new

	for _, msg := range messages {
		role, _ := msg["role"].(string)

		if role == "assistant" {
			if toolCalls, ok := msg["tool_calls"].([]any); ok {
				for _, tc := range toolCalls {
					if call, ok := tc.(map[string]any); ok {
						if oldID, ok := call["id"].(string); ok {
							newID := SanitizeToolCallID(oldID, mode)
							if newID != oldID {
								idMapping[oldID] = newID
								call["id"] = newID
							}
						}
					}
				}
			}
		}

		if role == "tool" {
			if oldID, ok := msg["tool_call_id"].(string); ok {
				if newID, exists := idMapping[oldID]; exists {
					msg["tool_call_id"] = newID
				} else {
					newID := SanitizeToolCallID(oldID, mode)
					if newID != oldID {
						msg["tool_call_id"] = newID
					}
				}
			}
		}
	}
	return messages
}
