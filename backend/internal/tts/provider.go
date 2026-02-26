package tts

import (
	"os"
	"regexp"
	"strings"
)

// TS 对照: tts/tts.ts L422-515 (Provider 路由)

// ---------- API Key 解析 ----------

// ResolveTtsApiKey 解析指定 Provider 的 API Key。
// TS 对照: tts.ts L491-502
func ResolveTtsApiKey(config ResolvedTtsConfig, provider TtsProvider) string {
	switch provider {
	case ProviderElevenLabs:
		if config.ElevenLabs.APIKey != "" {
			return config.ElevenLabs.APIKey
		}
		if key := os.Getenv("ELEVENLABS_API_KEY"); key != "" {
			return key
		}
		return os.Getenv("XI_API_KEY")
	case ProviderOpenAI:
		if config.OpenAI.APIKey != "" {
			return config.OpenAI.APIKey
		}
		return os.Getenv("OPENAI_API_KEY")
	default:
		return ""
	}
}

// ---------- Provider 选择 ----------

// GetTtsProvider 获取当前 TTS Provider。
// 优先级: prefs > config > 自动检测 (有 API key 的优先)。
// TS 对照: tts.ts L422-438
func GetTtsProvider(config ResolvedTtsConfig, prefsPath string) TtsProvider {
	prefs := readPrefs(prefsPath)
	if prefs.Tts != nil && prefs.Tts.Provider != "" {
		return prefs.Tts.Provider
	}
	if config.ProviderSource == "config" {
		return config.Provider
	}
	// 自动检测: 有 API key 就用
	if ResolveTtsApiKey(config, ProviderOpenAI) != "" {
		return ProviderOpenAI
	}
	if ResolveTtsApiKey(config, ProviderElevenLabs) != "" {
		return ProviderElevenLabs
	}
	return ProviderEdge
}

// ResolveTtsProviderOrder 解析 Provider 尝试顺序。
// TS 对照: tts.ts L506-508
func ResolveTtsProviderOrder(primary TtsProvider) []TtsProvider {
	order := []TtsProvider{primary}
	for _, p := range TtsProviders {
		if p != primary {
			order = append(order, p)
		}
	}
	return order
}

// IsTtsProviderConfigured 判断 Provider 是否已配置。
// TS 对照: tts.ts L510-515
func IsTtsProviderConfigured(config ResolvedTtsConfig, provider TtsProvider) bool {
	if provider == ProviderEdge {
		return config.Edge.Enabled
	}
	return ResolveTtsApiKey(config, provider) != ""
}

// ---------- 输出格式 ----------

// ResolveOutputFormat 根据频道解析输出格式。
// TS 对照: tts.ts L476-481
func ResolveOutputFormat(channelID string) OutputFormat {
	if channelID == "telegram" {
		return TelegramOutput
	}
	return DefaultOutput
}

// ---------- 验证辅助 ----------

// validVoiceIDRe ElevenLabs voice ID 验证正则。
var validVoiceIDRe = regexp.MustCompile(`^[a-zA-Z0-9]{10,40}$`)

// IsValidVoiceID 验证 ElevenLabs voice ID。
// TS 对照: tts.ts L517-519
func IsValidVoiceID(voiceID string) bool {
	return validVoiceIDRe.MatchString(voiceID)
}

// NormalizeElevenLabsBaseURL 规范化 ElevenLabs base URL。
// TS 对照: tts.ts L521-527
func NormalizeElevenLabsBaseURL(baseURL string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return DefaultElevenLabsBaseURL
	}
	return strings.TrimRight(trimmed, "/")
}
