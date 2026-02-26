package helpers

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ---------- Bootstrap 辅助 ----------

// TS 参考: src/agents/pi-embedded-helpers/bootstrap.ts

const DefaultBootstrapMaxChars = 256_000

// ResolveBootstrapMaxChars 解析 bootstrap 最大字符数。
func ResolveBootstrapMaxChars(configuredMax int) int {
	if configuredMax > 0 {
		return configuredMax
	}
	return DefaultBootstrapMaxChars
}

// EmbeddedContextFile 嵌入上下文文件。
type EmbeddedContextFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// ---------- 思维签名清洗 ----------

// TS 参考: src/agents/pi-embedded-helpers/bootstrap.ts → stripThoughtSignatures

var (
	// 思维签名: base64 编码的标记
	thoughtSignatureBase64RE = regexp.MustCompile(`[A-Za-z0-9+/]{40,}={0,3}`)
	// 思维签名: camelCase 形式
	thoughtSignatureCamelRE = regexp.MustCompile(`\b[a-z]+(?:[A-Z][a-z]+){3,}\b`)
)

// StripThoughtSignatures 移除 AI 思维签名。
func StripThoughtSignatures(text string, opts StripSignatureOpts) string {
	if text == "" {
		return text
	}
	result := text
	if opts.AllowBase64Only {
		result = thoughtSignatureBase64RE.ReplaceAllString(result, "")
	}
	if opts.IncludeCamelCase {
		result = thoughtSignatureCamelRE.ReplaceAllString(result, "")
	}
	return strings.TrimSpace(result)
}

// StripSignatureOpts 签名清洗选项。
type StripSignatureOpts struct {
	AllowBase64Only  bool
	IncludeCamelCase bool
}

// ---------- Google 辅助 ----------

// TS 参考: src/agents/pi-embedded-helpers/google.ts

// IsGoogleModelApi 检查是否 Google 模型 API。
func IsGoogleModelApi(modelApi string) bool {
	return modelApi == "google-genai" || modelApi == "google-vertex"
}

// IsAntigravityClaude 检查是否 Antigravity Claude 模型。
func IsAntigravityClaude(api, provider, modelId string) bool {
	if api == "anthropic-messages" && strings.HasPrefix(strings.ToLower(provider), "antigravity") {
		return true
	}
	lower := strings.ToLower(modelId)
	return strings.Contains(lower, "antigravity") && strings.Contains(lower, "claude")
}

// ---------- OpenAI 辅助 ----------

// TS 参考: src/agents/pi-embedded-helpers/openai.ts

// DowngradeOpenAIReasoningBlocks 降级 OpenAI reasoning blocks，防止 Responses API 拒绝孤立的
// reasoning item。遍历消息数组，对 role=assistant 且 content 为数组的消息执行：
//   - 查找 type=thinking 且含有效 reasoning signature（id 以 "rs_" 开头 + type 为 "reasoning.*"）的块
//   - 如果该 thinking 块后面没有非 thinking 块（即孤立在尾部），则丢弃该块
//   - 如果丢弃后 content 为空，则整条消息丢弃
//
// 对齐 TS: src/agents/pi-embedded-helpers/openai.ts downgradeOpenAIReasoningBlocks()
func DowngradeOpenAIReasoningBlocks(messages []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(messages))

	for _, msg := range messages {
		if msg == nil {
			out = append(out, msg)
			continue
		}

		role, _ := msg["role"].(string)
		if role != "assistant" {
			out = append(out, msg)
			continue
		}

		content, ok := msg["content"].([]interface{})
		if !ok {
			out = append(out, msg)
			continue
		}

		changed := false
		nextContent := make([]interface{}, 0, len(content))

		for i, block := range content {
			bm, ok := block.(map[string]interface{})
			if !ok {
				nextContent = append(nextContent, block)
				continue
			}

			typ, _ := bm["type"].(string)
			if typ != "thinking" {
				nextContent = append(nextContent, block)
				continue
			}

			// 检查 thinkingSignature 是否为有效的 OpenAI reasoning signature
			if !isValidReasoningSignature(bm["thinkingSignature"]) {
				nextContent = append(nextContent, block)
				continue
			}

			// 检查后面是否有非 thinking 块
			if hasFollowingNonThinkingBlock(content, i) {
				nextContent = append(nextContent, block)
				continue
			}

			// 丢弃这个孤立的 thinking block
			changed = true
		}

		if !changed {
			out = append(out, msg)
			continue
		}

		if len(nextContent) == 0 {
			// 整条消息丢弃
			continue
		}

		// 浅拷贝消息，替换 content
		newMsg := make(map[string]interface{}, len(msg))
		for k, v := range msg {
			newMsg[k] = v
		}
		newMsg["content"] = nextContent
		out = append(out, newMsg)
	}

	return out
}

// isValidReasoningSignature 检查 thinkingSignature 是否为有效的 OpenAI reasoning signature。
// 有效条件：id 以 "rs_" 开头 且 type 为 "reasoning" 或 "reasoning.*"。
// signature 可以是 JSON 字符串或 map。
func isValidReasoningSignature(val interface{}) bool {
	if val == nil {
		return false
	}

	var candidate map[string]interface{}

	switch v := val.(type) {
	case map[string]interface{}:
		candidate = v
	case string:
		trimmed := strings.TrimSpace(v)
		if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
			return false
		}
		if err := json.Unmarshal([]byte(trimmed), &candidate); err != nil {
			return false
		}
	default:
		return false
	}

	id, _ := candidate["id"].(string)
	typ, _ := candidate["type"].(string)

	if !strings.HasPrefix(id, "rs_") {
		return false
	}
	return typ == "reasoning" || strings.HasPrefix(typ, "reasoning.")
}

// hasFollowingNonThinkingBlock 检查 content[index] 后面是否存在 type 不为 "thinking" 的块。
func hasFollowingNonThinkingBlock(content []interface{}, index int) bool {
	for i := index + 1; i < len(content); i++ {
		block := content[i]
		bm, ok := block.(map[string]interface{})
		if !ok {
			return true // 非 map 类型视为非 thinking
		}
		typ, _ := bm["type"].(string)
		if typ != "thinking" {
			return true
		}
	}
	return false
}

// ---------- Thinking 辅助 ----------

// TS 参考: src/agents/pi-embedded-helpers/thinking.ts

// PickFallbackThinkingLevel 选择后备思维级别。
func PickFallbackThinkingLevel(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	if fallback != "" {
		return fallback
	}
	return "off"
}

// ---------- 消息去重 ----------

// TS 参考: src/agents/pi-embedded-helpers/messaging-dedupe.ts

var normalizeSpaceRE = regexp.MustCompile(`\s+`)

// NormalizeTextForComparison 归一化文本用于比较。
func NormalizeTextForComparison(text string) string {
	lower := strings.ToLower(text)
	normalized := normalizeSpaceRE.ReplaceAllString(lower, " ")
	return strings.TrimSpace(normalized)
}

// IsMessagingToolDuplicate 检查是否为消息工具重复调用。
func IsMessagingToolDuplicate(a, b string) bool {
	return NormalizeTextForComparison(a) == NormalizeTextForComparison(b)
}
