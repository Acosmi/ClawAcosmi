package transcript

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
)

// ---------- Tool Call ID 清洗 ----------

// TS 参考: src/agents/tool-call-id.ts (222 行)

// ToolCallIdMode ID 清洗模式。
type ToolCallIdMode string

const (
	ToolIdStrict  ToolCallIdMode = "strict"
	ToolIdStrict9 ToolCallIdMode = "strict9"
)

const strict9Len = 9

var nonAlphanumericRE = regexp.MustCompile(`[^a-zA-Z0-9]`)

// SanitizeToolCallId 清洗 tool call ID。
// strict: 仅 [a-zA-Z0-9]
// strict9: 仅 [a-zA-Z0-9], 长度 9 (Mistral 要求)
func SanitizeToolCallId(id string, mode ToolCallIdMode) string {
	if id == "" {
		if mode == ToolIdStrict9 {
			return "defaultid"
		}
		return "defaulttoolid"
	}

	alphanumeric := nonAlphanumericRE.ReplaceAllString(id, "")

	if mode == ToolIdStrict9 {
		if len(alphanumeric) >= strict9Len {
			return alphanumeric[:strict9Len]
		}
		if len(alphanumeric) > 0 {
			return shortHash(alphanumeric, strict9Len)
		}
		return shortHash("sanitized", strict9Len)
	}

	if len(alphanumeric) > 0 {
		return alphanumeric
	}
	return "sanitizedtoolid"
}

// IsValidToolId 检查 tool call ID 是否有效。
func IsValidToolId(id string, mode ToolCallIdMode) bool {
	if id == "" {
		return false
	}
	if mode == ToolIdStrict9 {
		return len(id) == 9 && nonAlphanumericRE.FindString(id) == ""
	}
	return nonAlphanumericRE.FindString(id) == ""
}

// shortHash 短哈希。
func shortHash(text string, length int) string {
	h := sha1.Sum([]byte(text))
	hex := hex.EncodeToString(h[:])
	if length > len(hex) {
		length = len(hex)
	}
	return hex[:length]
}

// MakeUniqueToolId 生成唯一 tool ID。
func MakeUniqueToolId(id string, used map[string]bool, mode ToolCallIdMode) string {
	const maxLen = 40

	if mode == ToolIdStrict9 {
		base := SanitizeToolCallId(id, mode)
		if len(base) >= strict9Len {
			candidate := base[:strict9Len]
			if !used[candidate] {
				return candidate
			}
		}
		for i := 0; i < 1000; i++ {
			hashed := shortHash(id+":"+itoa(i), strict9Len)
			if !used[hashed] {
				return hashed
			}
		}
		return shortHash(id+":fallback", strict9Len)
	}

	base := SanitizeToolCallId(id, mode)
	if len(base) > maxLen {
		base = base[:maxLen]
	}
	if !used[base] {
		return base
	}

	hash := shortHash(id, 8)
	maxBase := maxLen - len(hash)
	clipped := base
	if len(clipped) > maxBase {
		clipped = clipped[:maxBase]
	}
	candidate := clipped + hash
	if !used[candidate] {
		return candidate
	}

	for i := 2; i < 1000; i++ {
		suffix := "x" + itoa(i)
		next := candidate
		if len(next)+len(suffix) > maxLen {
			next = next[:maxLen-len(suffix)]
		}
		next = next + suffix
		if !used[next] {
			return next
		}
	}

	return candidate[:maxLen-10] + shortHash(id+":ts", 10)
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return strings.Join([]string{itoa(i / 10), string(rune('0' + i%10))}, "")
}

// ---------- 转录策略 ----------

// TS 参考: src/agents/transcript-policy.ts (124 行)

// TranscriptSanitizeMode 转录清洗模式。
type TranscriptSanitizeMode string

const (
	SanitizeFull       TranscriptSanitizeMode = "full"
	SanitizeImagesOnly TranscriptSanitizeMode = "images-only"
)

// TranscriptPolicy 转录策略。
type TranscriptPolicy struct {
	SanitizeMode                       TranscriptSanitizeMode `json:"sanitizeMode"`
	SanitizeToolCallIds                bool                   `json:"sanitizeToolCallIds"`
	ToolCallIdMode                     ToolCallIdMode         `json:"toolCallIdMode,omitempty"`
	RepairToolUseResultPairing         bool                   `json:"repairToolUseResultPairing"`
	PreserveSignatures                 bool                   `json:"preserveSignatures"`
	NormalizeAntigravityThinkingBlocks bool                   `json:"normalizeAntigravityThinkingBlocks"`
	ApplyGoogleTurnOrdering            bool                   `json:"applyGoogleTurnOrdering"`
	ValidateGeminiTurns                bool                   `json:"validateGeminiTurns"`
	ValidateAnthropicTurns             bool                   `json:"validateAnthropicTurns"`
	AllowSyntheticToolResults          bool                   `json:"allowSyntheticToolResults"`
}

// Mistral 模型提示词。
var mistralModelHints = []string{
	"mistral", "mixtral", "codestral", "pixtral", "devstral", "ministral", "mistralai",
}

var openaiModelAPIs = map[string]bool{
	"openai": true, "openai-completions": true,
	"openai-responses": true, "openai-codex-responses": true,
}

var openaiProviders = map[string]bool{
	"openai": true, "openai-codex": true,
}

// IsOpenAiApi 检查是否为 OpenAI API。
func IsOpenAiApi(modelApi string) bool {
	return openaiModelAPIs[modelApi]
}

// IsOpenAiProvider 检查是否为 OpenAI 供应商。
func IsOpenAiProvider(provider string) bool {
	return openaiProviders[strings.ToLower(provider)]
}

// IsAnthropicApi 检查是否为 Anthropic API。
func IsAnthropicApi(modelApi, provider string) bool {
	if modelApi == "anthropic-messages" {
		return true
	}
	return strings.ToLower(provider) == "anthropic"
}

// IsGoogleModelApi 检查是否为 Google 模型 API。
func IsGoogleModelApi(modelApi string) bool {
	return modelApi == "google-genai" || modelApi == "google-vertex"
}

// IsMistralModel 检查是否为 Mistral 模型。
func IsMistralModel(provider, modelId string) bool {
	if strings.ToLower(provider) == "mistral" {
		return true
	}
	lower := strings.ToLower(modelId)
	for _, hint := range mistralModelHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

// ResolveTranscriptPolicy 解析转录策略。
func ResolveTranscriptPolicy(modelApi, provider, modelId string) TranscriptPolicy {
	normalizedProvider := strings.ToLower(provider)
	isGoogle := IsGoogleModelApi(modelApi)
	isAnthropic := IsAnthropicApi(modelApi, normalizedProvider)
	isOpenAI := IsOpenAiProvider(normalizedProvider) || (normalizedProvider == "" && IsOpenAiApi(modelApi))
	isMistral := IsMistralModel(normalizedProvider, modelId)
	isOpenRouterGemini := (normalizedProvider == "openrouter" || normalizedProvider == "openacosmi") &&
		strings.Contains(strings.ToLower(modelId), "gemini")

	needsNonImageSanitize := isGoogle || isAnthropic || isMistral || isOpenRouterGemini
	sanitizeToolCallIds := isGoogle || isMistral

	var toolCallIdMode ToolCallIdMode
	if isMistral {
		toolCallIdMode = ToolIdStrict9
	} else if sanitizeToolCallIds {
		toolCallIdMode = ToolIdStrict
	}

	repairToolUseResultPairing := isGoogle || isAnthropic

	sanitizeMode := SanitizeImagesOnly
	if !isOpenAI && needsNonImageSanitize {
		sanitizeMode = SanitizeFull
	}

	return TranscriptPolicy{
		SanitizeMode:                       sanitizeMode,
		SanitizeToolCallIds:                !isOpenAI && sanitizeToolCallIds,
		ToolCallIdMode:                     toolCallIdMode,
		RepairToolUseResultPairing:         !isOpenAI && repairToolUseResultPairing,
		PreserveSignatures:                 false, // requires isAntigravityClaude check
		NormalizeAntigravityThinkingBlocks: false, // requires isAntigravityClaude check
		ApplyGoogleTurnOrdering:            !isOpenAI && isGoogle,
		ValidateGeminiTurns:                !isOpenAI && isGoogle,
		ValidateAnthropicTurns:             !isOpenAI && isAnthropic,
		AllowSyntheticToolResults:          !isOpenAI && (isGoogle || isAnthropic),
	}
}
