package runner

// ============================================================================
// Gemini 特殊处理
// 对应 TS: pi-embedded-runner/google.ts (394L) +
//          schema/clean-for-gemini.ts (377L) +
//          pi-embedded-helpers/google.ts (23L)
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/agents/llmclient"
)

// ---------- Google Provider 检测 ----------

// IsGoogleProvider 检测是否为 Google/Gemini 供应商。
// TS 对照: pi-embedded-helpers/google.ts → isGoogleModelApi()
func IsGoogleProvider(provider string) bool {
	switch provider {
	case "google-gemini-cli", "google-generative-ai", "google-antigravity":
		return true
	}
	return false
}

// IsGoogleModelAPI 检测 modelApi 是否为 Google API。
func IsGoogleModelAPI(api string) bool {
	return IsGoogleProvider(api)
}

// ---------- Gemini 不支持的 JSON Schema 关键字 ----------

// geminiUnsupportedKeywords Cloud Code Assist API 拒绝的 JSON Schema 关键字。
// TS 对照: schema/clean-for-gemini.ts → GEMINI_UNSUPPORTED_SCHEMA_KEYWORDS
var geminiUnsupportedKeywords = map[string]bool{
	"patternProperties":    true,
	"additionalProperties": true,
	"$schema":              true,
	"$id":                  true,
	"$ref":                 true,
	"$defs":                true,
	"definitions":          true,
	"examples":             true,
	"minLength":            true,
	"maxLength":            true,
	"minimum":              true,
	"maximum":              true,
	"multipleOf":           true,
	"pattern":              true,
	"format":               true,
	"minItems":             true,
	"maxItems":             true,
	"uniqueItems":          true,
	"minProperties":        true,
	"maxProperties":        true,
}

// ---------- Tool Schema 清洗 ----------

// CleanToolSchemaForGemini 递归清洗 JSON Schema，移除 Gemini 不支持的关键字。
// TS 对照: schema/clean-for-gemini.ts → cleanSchemaForGemini()
func CleanToolSchemaForGemini(schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 {
		return schema
	}
	var raw interface{}
	if err := json.Unmarshal(schema, &raw); err != nil {
		return schema
	}
	cleaned := cleanSchemaRecursive(raw)
	result, err := json.Marshal(cleaned)
	if err != nil {
		return schema
	}
	return result
}

// cleanSchemaRecursive 递归清洗 schema 节点。
func cleanSchemaRecursive(schema interface{}) interface{} {
	if schema == nil {
		return schema
	}
	switch v := schema.(type) {
	case []interface{}:
		out := make([]interface{}, len(v))
		for i, item := range v {
			out[i] = cleanSchemaRecursive(item)
		}
		return out
	case map[string]interface{}:
		return cleanSchemaObject(v)
	default:
		return schema
	}
}

// cleanSchemaObject 清洗单个 schema 对象。
func cleanSchemaObject(obj map[string]interface{}) map[string]interface{} {
	cleaned := make(map[string]interface{})

	for key, value := range obj {
		// 跳过不支持的关键字
		if geminiUnsupportedKeywords[key] {
			continue
		}

		// const → enum 转换
		if key == "const" {
			cleaned["enum"] = []interface{}{value}
			continue
		}

		// type 数组移除 "null"
		if key == "type" {
			if arr, ok := value.([]interface{}); ok {
				types := make([]interface{}, 0, len(arr))
				for _, t := range arr {
					if s, ok := t.(string); ok && s != "null" {
						types = append(types, s)
					}
				}
				if len(types) == 1 {
					cleaned["type"] = types[0]
				} else {
					cleaned["type"] = types
				}
				continue
			}
		}

		// 递归处理嵌套 schema
		switch key {
		case "properties":
			if props, ok := value.(map[string]interface{}); ok {
				cleanedProps := make(map[string]interface{})
				for k, v := range props {
					cleanedProps[k] = cleanSchemaRecursive(v)
				}
				cleaned[key] = cleanedProps
			} else {
				cleaned[key] = value
			}
		case "items":
			if arr, ok := value.([]interface{}); ok {
				out := make([]interface{}, len(arr))
				for i, item := range arr {
					out[i] = cleanSchemaRecursive(item)
				}
				cleaned[key] = out
			} else {
				cleaned[key] = cleanSchemaRecursive(value)
			}
		case "anyOf", "oneOf", "allOf":
			if arr, ok := value.([]interface{}); ok {
				out := make([]interface{}, len(arr))
				for i, variant := range arr {
					out[i] = cleanSchemaRecursive(variant)
				}
				cleaned[key] = out
			} else {
				cleaned[key] = value
			}
		default:
			cleaned[key] = value
		}
	}

	return cleaned
}

// SanitizeToolsForGoogle 清洗工具 schema 以兼容 Gemini API。
// TS 对照: google.ts → sanitizeToolsForGoogle()
func SanitizeToolsForGoogle(tools []llmclient.ToolDef, provider string) []llmclient.ToolDef {
	if !IsGoogleProvider(provider) {
		return tools
	}
	result := make([]llmclient.ToolDef, len(tools))
	for i, tool := range tools {
		result[i] = llmclient.ToolDef{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: CleanToolSchemaForGemini(tool.InputSchema),
		}
	}
	return result
}

// FindUnsupportedSchemaKeywords 递归查找 schema 中不支持的关键字。
// TS 对照: google.ts → findUnsupportedSchemaKeywords()
func FindUnsupportedSchemaKeywords(schema json.RawMessage) []string {
	if len(schema) == 0 {
		return nil
	}
	var raw interface{}
	if err := json.Unmarshal(schema, &raw); err != nil {
		return nil
	}
	var violations []string
	findUnsupportedRecursive(raw, "root", &violations)
	return violations
}

func findUnsupportedRecursive(schema interface{}, path string, violations *[]string) {
	if schema == nil {
		return
	}
	switch v := schema.(type) {
	case []interface{}:
		for i, item := range v {
			findUnsupportedRecursive(item, fmt.Sprintf("%s[%d]", path, i), violations)
		}
	case map[string]interface{}:
		for key, value := range v {
			if geminiUnsupportedKeywords[key] {
				*violations = append(*violations, fmt.Sprintf("%s.%s", path, key))
			}
			if value != nil {
				findUnsupportedRecursive(value, fmt.Sprintf("%s.%s", path, key), violations)
			}
		}
	}
}

// ---------- Antigravity Thinking Blocks 清洗 ----------

// antigravitySignatureRE base64 签名格式验证。
var antigravitySignatureRE = regexp.MustCompile(`^[A-Za-z0-9+/]+={0,2}$`)

// IsValidAntigravitySignature 检查是否为有效的 base64 签名。
// TS 对照: google.ts → isValidAntigravitySignature()
func IsValidAntigravitySignature(value interface{}) bool {
	s, ok := value.(string)
	if !ok || s == "" {
		return false
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	if len(trimmed)%4 != 0 {
		return false
	}
	return antigravitySignatureRE.MatchString(trimmed)
}

// SanitizeAntigravityThinkingBlocks 清洗 assistant 消息中无效的 thinking 块。
// 移除没有有效 base64 签名的 thinking 块，保留有效的。
// TS 对照: google.ts → sanitizeAntigravityThinkingBlocks()
func SanitizeAntigravityThinkingBlocks(messages []llmclient.ChatMessage) []llmclient.ChatMessage {
	touched := false
	out := make([]llmclient.ChatMessage, 0, len(messages))

	for _, msg := range messages {
		if msg.Role != "assistant" || len(msg.Content) == 0 {
			out = append(out, msg)
			continue
		}

		nextContent := make([]llmclient.ContentBlock, 0, len(msg.Content))
		contentChanged := false

		for _, block := range msg.Content {
			if block.Type != "thinking" {
				nextContent = append(nextContent, block)
				continue
			}

			// 检查签名有效性 — ThinkingSignature 是规范化字段
			sig := block.ThinkingSignature
			if !IsValidAntigravitySignature(sig) {
				// 无效签名 → 移除此 thinking 块
				contentChanged = true
				continue
			}

			nextContent = append(nextContent, block)
		}

		if contentChanged {
			touched = true
		}

		if len(nextContent) == 0 {
			touched = true
			continue
		}

		if contentChanged {
			out = append(out, llmclient.ChatMessage{
				Role:    msg.Role,
				Content: nextContent,
			})
		} else {
			out = append(out, msg)
		}
	}

	if touched {
		return out
	}
	return messages
}

// ---------- Google Turn Ordering 修复 ----------

const googleTurnOrderBootstrapText = "(session bootstrap)"

// SanitizeGoogleTurnOrdering 修复 Gemini 要求的 user-first 消息序。
// 如果消息序以 assistant 开头，前置一个合成的 user 消息。
// TS 对照: pi-embedded-helpers/bootstrap.ts → sanitizeGoogleTurnOrdering()
func SanitizeGoogleTurnOrdering(messages []llmclient.ChatMessage) ([]llmclient.ChatMessage, bool) {
	if len(messages) == 0 {
		return messages, false
	}

	first := messages[0]

	// 如果已有 bootstrap 标记，跳过
	if first.Role == "user" && len(first.Content) > 0 {
		text := first.Content[0].Text
		if strings.TrimSpace(text) == googleTurnOrderBootstrapText {
			return messages, false
		}
	}

	// 非 assistant 开头，不需要修复
	if first.Role != "assistant" {
		return messages, false
	}

	// 前置合成的 user turn
	bootstrap := llmclient.TextMessage("user", googleTurnOrderBootstrapText)
	result := make([]llmclient.ChatMessage, 0, len(messages)+1)
	result = append(result, bootstrap)
	result = append(result, messages...)
	return result, true
}

// ApplyGoogleTurnOrderingFix 应用 Google turn ordering 修复并记录日志。
// TS 对照: google.ts → applyGoogleTurnOrderingFix()
func ApplyGoogleTurnOrderingFix(messages []llmclient.ChatMessage, modelAPI string, sessionID string, logger *slog.Logger) []llmclient.ChatMessage {
	if !IsGoogleModelAPI(modelAPI) {
		return messages
	}
	sanitized, didPrepend := SanitizeGoogleTurnOrdering(messages)
	if didPrepend && logger != nil {
		logger.Warn("google turn ordering fixup: prepended user bootstrap",
			"sessionId", sessionID)
	}
	return sanitized
}

// ---------- Session History 消毒 ----------

// SanitizeSessionHistory 多阶段 session 历史消毒管线。
// TS 对照: google.ts → sanitizeSessionHistory()
func SanitizeSessionHistory(messages []llmclient.ChatMessage, modelAPI, provider, modelID, sessionID string, logger *slog.Logger) []llmclient.ChatMessage {
	// 阶段 1: 图片消毒 (stub — 标记延迟，需 image sanitization 管线)
	sanitized := messages

	// 阶段 2: Antigravity thinking blocks 清洗
	sanitized = SanitizeAntigravityThinkingBlocks(sanitized)

	// 阶段 3a: Tool call input 修复 — 移除缺少 input 的空 tool_use 块
	sanitized = SanitizeToolCallInputs(sanitized)

	// 阶段 3b: Tool use/result 配对修复 — 确保每个 tool_use 紧跟对应 tool_result
	sanitized = SanitizeToolUseResultPairing(sanitized)

	// 阶段 4: OpenAI reasoning 降级 (stub — downgradeOpenAIReasoningBlocks)

	// 阶段 5: Google turn ordering 修复
	sanitized = ApplyGoogleTurnOrderingFix(sanitized, modelAPI, sessionID, logger)

	return sanitized
}

// ---------- Model Snapshot ----------

// ModelSnapshotEntry 模型快照条目，用于检测模型切换。
// TS 对照: google.ts → ModelSnapshotEntry
type ModelSnapshotEntry struct {
	Timestamp int64  `json:"timestamp"`
	Provider  string `json:"provider,omitempty"`
	ModelAPI  string `json:"modelApi,omitempty"`
	ModelID   string `json:"modelId,omitempty"`
}

// IsSameModelSnapshot 比较两个模型快照是否相同。
// TS 对照: google.ts → isSameModelSnapshot()
func IsSameModelSnapshot(a, b ModelSnapshotEntry) bool {
	return a.Provider == b.Provider && a.ModelAPI == b.ModelAPI && a.ModelID == b.ModelID
}

// NewModelSnapshot 创建模型快照。
func NewModelSnapshot(provider, modelAPI, modelID string) ModelSnapshotEntry {
	return ModelSnapshotEntry{
		Timestamp: time.Now().UnixMilli(),
		Provider:  provider,
		ModelAPI:  modelAPI,
		ModelID:   modelID,
	}
}

// ---------- Compaction Failure ----------

// CompactionFailureCallback compaction 失败回调类型。
type CompactionFailureCallback func(reason string)

// IsCompactionFailureError 检查错误消息是否为 compaction 失败。
// TS 对照: pi-embedded-helpers.ts → isCompactionFailureError()
func IsCompactionFailureError(msg string) bool {
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "compaction") && strings.Contains(lower, "fail")
}

// LogToolSchemasForGoogle 记录 Google 工具 schema 信息（调试用）。
// TS 对照: google.ts → logToolSchemasForGoogle()
func LogToolSchemasForGoogle(tools []llmclient.ToolDef, provider string, logger *slog.Logger) {
	if !IsGoogleProvider(provider) {
		return
	}
	if logger == nil {
		return
	}

	sanitized := SanitizeToolsForGoogle(tools, provider)
	toolNames := make([]string, len(sanitized))
	for i, t := range sanitized {
		toolNames[i] = fmt.Sprintf("%d:%s", i, t.Name)
	}

	logger.Info("google tool schema snapshot",
		"provider", provider,
		"toolCount", len(sanitized),
		"tools", toolNames)

	for i, tool := range sanitized {
		violations := FindUnsupportedSchemaKeywords(tool.InputSchema)
		if len(violations) > 0 {
			maxShow := 12
			if len(violations) > maxShow {
				violations = violations[:maxShow]
			}
			logger.Warn("google tool schema has unsupported keywords",
				"index", i,
				"tool", tool.Name,
				"violations", violations,
				"violationCount", len(violations))
		}
	}
}
