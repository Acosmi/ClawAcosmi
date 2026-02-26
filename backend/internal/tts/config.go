package tts

import (
	"os"
	"strings"
)

// TS 对照: tts/tts.ts L208-303 (配置解析)

// ---------- 配置解析 ----------

// TtsRawConfig 原始 TTS 配置（对应 config YAML/JSON）。
// TS 对照: config/types.tts.ts TtsConfig
type TtsRawConfig struct {
	Auto           string                  `json:"auto,omitempty"`
	Enabled        *bool                   `json:"enabled,omitempty"`
	Mode           string                  `json:"mode,omitempty"`
	Provider       string                  `json:"provider,omitempty"`
	SummaryModel   string                  `json:"summaryModel,omitempty"`
	ModelOverrides *TtsModelOverrideConfig `json:"modelOverrides,omitempty"`
	ElevenLabs     *ElevenLabsRawConfig    `json:"elevenlabs,omitempty"`
	OpenAI         *OpenAITtsRawConfig     `json:"openai,omitempty"`
	Edge           *EdgeTtsRawConfig       `json:"edge,omitempty"`
	PrefsPath      string                  `json:"prefsPath,omitempty"`
	MaxTextLength  int                     `json:"maxTextLength,omitempty"`
	TimeoutMs      int                     `json:"timeoutMs,omitempty"`
}

// TtsModelOverrideConfig 模型覆盖原始配置。
type TtsModelOverrideConfig struct {
	Enabled            *bool `json:"enabled,omitempty"`
	AllowText          *bool `json:"allowText,omitempty"`
	AllowProvider      *bool `json:"allowProvider,omitempty"`
	AllowVoice         *bool `json:"allowVoice,omitempty"`
	AllowModelID       *bool `json:"allowModelId,omitempty"`
	AllowVoiceSettings *bool `json:"allowVoiceSettings,omitempty"`
	AllowNormalization *bool `json:"allowNormalization,omitempty"`
	AllowSeed          *bool `json:"allowSeed,omitempty"`
}

// ElevenLabsRawConfig ElevenLabs 原始配置。
type ElevenLabsRawConfig struct {
	APIKey                 string                      `json:"apiKey,omitempty"`
	BaseURL                string                      `json:"baseUrl,omitempty"`
	VoiceID                string                      `json:"voiceId,omitempty"`
	ModelID                string                      `json:"modelId,omitempty"`
	Seed                   *int                        `json:"seed,omitempty"`
	ApplyTextNormalization string                      `json:"applyTextNormalization,omitempty"`
	LanguageCode           string                      `json:"languageCode,omitempty"`
	VoiceSettings          *ElevenLabsVoiceRawSettings `json:"voiceSettings,omitempty"`
}

// ElevenLabsVoiceRawSettings ElevenLabs 语音原始设置。
type ElevenLabsVoiceRawSettings struct {
	Stability       *float64 `json:"stability,omitempty"`
	SimilarityBoost *float64 `json:"similarityBoost,omitempty"`
	Style           *float64 `json:"style,omitempty"`
	UseSpeakerBoost *bool    `json:"useSpeakerBoost,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
}

// OpenAITtsRawConfig OpenAI TTS 原始配置。
type OpenAITtsRawConfig struct {
	APIKey  string `json:"apiKey,omitempty"`
	Model   string `json:"model,omitempty"`
	Voice   string `json:"voice,omitempty"`
	BaseURL string `json:"baseUrl,omitempty"`
}

// EdgeTtsRawConfig Edge TTS 原始配置。
type EdgeTtsRawConfig struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	Voice         string `json:"voice,omitempty"`
	Lang          string `json:"lang,omitempty"`
	OutputFormat  string `json:"outputFormat,omitempty"`
	Pitch         string `json:"pitch,omitempty"`
	Rate          string `json:"rate,omitempty"`
	Volume        string `json:"volume,omitempty"`
	SaveSubtitles *bool  `json:"saveSubtitles,omitempty"`
	Proxy         string `json:"proxy,omitempty"`
	TimeoutMs     int    `json:"timeoutMs,omitempty"`
}

// ---------- 解析函数 ----------

// NormalizeTtsAutoMode 规范化自动模式字符串。
// TS 对照: tts.ts L208-217
func NormalizeTtsAutoMode(value string) TtsAutoMode {
	normalized := TtsAutoMode(strings.TrimSpace(strings.ToLower(value)))
	if ValidAutoModes[normalized] {
		return normalized
	}
	return ""
}

// resolveModelOverridePolicy 解析模型覆盖策略。
// TS 对照: tts.ts L219-246
func resolveModelOverridePolicy(overrides *TtsModelOverrideConfig) ResolvedTtsModelOverrides {
	if overrides == nil {
		return ResolvedTtsModelOverrides{
			Enabled:            true,
			AllowText:          true,
			AllowProvider:      true,
			AllowVoice:         true,
			AllowModelID:       true,
			AllowVoiceSettings: true,
			AllowNormalization: true,
			AllowSeed:          true,
		}
	}
	enabled := boolOrDefault(overrides.Enabled, true)
	if !enabled {
		return ResolvedTtsModelOverrides{Enabled: false}
	}
	return ResolvedTtsModelOverrides{
		Enabled:            true,
		AllowText:          boolOrDefault(overrides.AllowText, true),
		AllowProvider:      boolOrDefault(overrides.AllowProvider, true),
		AllowVoice:         boolOrDefault(overrides.AllowVoice, true),
		AllowModelID:       boolOrDefault(overrides.AllowModelID, true),
		AllowVoiceSettings: boolOrDefault(overrides.AllowVoiceSettings, true),
		AllowNormalization: boolOrDefault(overrides.AllowNormalization, true),
		AllowSeed:          boolOrDefault(overrides.AllowSeed, true),
	}
}

// boolOrDefault 从可选 bool 指针获取值。
func boolOrDefault(ptr *bool, defaultVal bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

// ResolveTtsConfig 从原始配置解析完整 TTS 配置。
// TS 对照: tts.ts L248-303
func ResolveTtsConfig(raw TtsRawConfig) ResolvedTtsConfig {
	providerSource := "default"
	if raw.Provider != "" {
		providerSource = "config"
	}

	edgeOutputFormat := strings.TrimSpace(raw.Edge.safeOutputFormat())

	auto := NormalizeTtsAutoMode(raw.Auto)
	if auto == "" {
		if raw.Enabled != nil && *raw.Enabled {
			auto = AutoAlways
		} else {
			auto = AutoOff
		}
	}

	return ResolvedTtsConfig{
		Auto:           auto,
		Mode:           TtsMode(orDefault(raw.Mode, string(ModeFinal))),
		Provider:       TtsProvider(orDefault(raw.Provider, string(ProviderEdge))),
		ProviderSource: providerSource,
		SummaryModel:   strings.TrimSpace(raw.SummaryModel),
		ModelOverrides: resolveModelOverridePolicy(raw.ModelOverrides),
		ElevenLabs: ElevenLabsConfig{
			APIKey:                 raw.ElevenLabs.safeAPIKey(),
			BaseURL:                orDefault(strings.TrimSpace(raw.ElevenLabs.safeBaseURL()), DefaultElevenLabsBaseURL),
			VoiceID:                orDefault(raw.ElevenLabs.safeVoiceID(), DefaultElevenLabsVoiceID),
			ModelID:                orDefault(raw.ElevenLabs.safeModelID(), DefaultElevenLabsModelID),
			Seed:                   raw.ElevenLabs.safeSeed(),
			ApplyTextNormalization: raw.ElevenLabs.safeApplyTextNorm(),
			LanguageCode:           raw.ElevenLabs.safeLanguageCode(),
			VoiceSettings:          resolveElevenLabsVoiceSettings(raw.ElevenLabs.safeVoiceSettings()),
		},
		OpenAI: OpenAITtsConfig{
			APIKey:  raw.OpenAI.safeAPIKey(),
			Model:   orDefault(raw.OpenAI.safeModel(), DefaultOpenAIModel),
			Voice:   orDefault(raw.OpenAI.safeVoice(), DefaultOpenAIVoice),
			BaseURL: resolveOpenAITtsBaseURL(raw.OpenAI.safeBaseURL()),
		},
		Edge: EdgeTtsConfig{
			Enabled:                raw.Edge.safeEnabled(),
			Voice:                  orDefault(strings.TrimSpace(raw.Edge.safeVoice()), DefaultEdgeVoice),
			Lang:                   orDefault(strings.TrimSpace(raw.Edge.safeLang()), DefaultEdgeLang),
			OutputFormat:           orDefault(edgeOutputFormat, DefaultEdgeOutputFormat),
			OutputFormatConfigured: edgeOutputFormat != "",
			Pitch:                  strings.TrimSpace(raw.Edge.safePitch()),
			Rate:                   strings.TrimSpace(raw.Edge.safeRate()),
			Volume:                 strings.TrimSpace(raw.Edge.safeVolume()),
			SaveSubtitles:          raw.Edge.safeSaveSubtitles(),
			Proxy:                  strings.TrimSpace(raw.Edge.safeProxy()),
			TimeoutMs:              raw.Edge.safeTimeoutMs(),
		},
		PrefsPath:     raw.PrefsPath,
		MaxTextLength: intOrDefault(raw.MaxTextLength, DefaultMaxTextLength),
		TimeoutMs:     intOrDefault(raw.TimeoutMs, DefaultTimeoutMs),
	}
}

// resolveElevenLabsVoiceSettings 解析 ElevenLabs 语音设置。
func resolveElevenLabsVoiceSettings(raw *ElevenLabsVoiceRawSettings) ElevenLabsVoiceSettings {
	d := DefaultElevenLabsVoiceSettings
	if raw == nil {
		return d
	}
	return ElevenLabsVoiceSettings{
		Stability:       floatOrDefault(raw.Stability, d.Stability),
		SimilarityBoost: floatOrDefault(raw.SimilarityBoost, d.SimilarityBoost),
		Style:           floatOrDefault(raw.Style, d.Style),
		UseSpeakerBoost: boolOrDefault(raw.UseSpeakerBoost, d.UseSpeakerBoost),
		Speed:           floatOrDefault(raw.Speed, d.Speed),
	}
}

// ---------- 辅助函数 ----------

func orDefault(value, defaultVal string) string {
	if value != "" {
		return value
	}
	return defaultVal
}

func intOrDefault(value, defaultVal int) int {
	if value > 0 {
		return value
	}
	return defaultVal
}

func floatOrDefault(ptr *float64, defaultVal float64) float64 {
	if ptr != nil {
		return *ptr
	}
	return defaultVal
}

// resolveOpenAITtsBaseURL 解析 OpenAI TTS 自定义端点 URL。
// 优先级: 配置值 → 环境变量 OPENAI_TTS_BASE_URL → 默认值。
// TS 对照: tts.ts L831-836 getOpenAITtsBaseUrl()
func resolveOpenAITtsBaseURL(configBaseURL string) string {
	if trimmed := strings.TrimSpace(configBaseURL); trimmed != "" {
		return strings.TrimRight(trimmed, "/")
	}
	if envURL := strings.TrimSpace(os.Getenv("OPENAI_TTS_BASE_URL")); envURL != "" {
		return strings.TrimRight(envURL, "/")
	}
	return DefaultOpenAITtsBaseURL
}

// IsCustomOpenAIEndpoint 判断是否使用自定义 OpenAI 端点。
// 自定义端点时跳过 model/voice 白名单校验。
// TS 对照: tts.ts L838-840 isCustomOpenAIEndpoint()
func IsCustomOpenAIEndpoint(baseURL string) bool {
	return baseURL != "" && baseURL != DefaultOpenAITtsBaseURL
}

// ---------- 安全访问器（nil-safe） ----------

func (c *ElevenLabsRawConfig) safeAPIKey() string {
	if c == nil {
		return ""
	}
	return c.APIKey
}
func (c *ElevenLabsRawConfig) safeBaseURL() string {
	if c == nil {
		return ""
	}
	return c.BaseURL
}
func (c *ElevenLabsRawConfig) safeVoiceID() string {
	if c == nil {
		return ""
	}
	return c.VoiceID
}
func (c *ElevenLabsRawConfig) safeModelID() string {
	if c == nil {
		return ""
	}
	return c.ModelID
}
func (c *ElevenLabsRawConfig) safeSeed() *int {
	if c == nil {
		return nil
	}
	return c.Seed
}
func (c *ElevenLabsRawConfig) safeApplyTextNorm() string {
	if c == nil {
		return ""
	}
	return c.ApplyTextNormalization
}
func (c *ElevenLabsRawConfig) safeLanguageCode() string {
	if c == nil {
		return ""
	}
	return c.LanguageCode
}
func (c *ElevenLabsRawConfig) safeVoiceSettings() *ElevenLabsVoiceRawSettings {
	if c == nil {
		return nil
	}
	return c.VoiceSettings
}

func (c *OpenAITtsRawConfig) safeAPIKey() string {
	if c == nil {
		return ""
	}
	return c.APIKey
}
func (c *OpenAITtsRawConfig) safeModel() string {
	if c == nil {
		return ""
	}
	return c.Model
}
func (c *OpenAITtsRawConfig) safeVoice() string {
	if c == nil {
		return ""
	}
	return c.Voice
}
func (c *OpenAITtsRawConfig) safeBaseURL() string {
	if c == nil {
		return ""
	}
	return c.BaseURL
}

func (c *EdgeTtsRawConfig) safeEnabled() bool {
	if c == nil {
		return true
	}
	return boolOrDefault(c.Enabled, true)
}
func (c *EdgeTtsRawConfig) safeVoice() string {
	if c == nil {
		return ""
	}
	return c.Voice
}
func (c *EdgeTtsRawConfig) safeLang() string {
	if c == nil {
		return ""
	}
	return c.Lang
}
func (c *EdgeTtsRawConfig) safeOutputFormat() string {
	if c == nil {
		return ""
	}
	return c.OutputFormat
}
func (c *EdgeTtsRawConfig) safePitch() string {
	if c == nil {
		return ""
	}
	return c.Pitch
}
func (c *EdgeTtsRawConfig) safeRate() string {
	if c == nil {
		return ""
	}
	return c.Rate
}
func (c *EdgeTtsRawConfig) safeVolume() string {
	if c == nil {
		return ""
	}
	return c.Volume
}
func (c *EdgeTtsRawConfig) safeSaveSubtitles() bool {
	if c == nil {
		return false
	}
	return boolOrDefault(c.SaveSubtitles, false)
}
func (c *EdgeTtsRawConfig) safeProxy() string {
	if c == nil {
		return ""
	}
	return c.Proxy
}
func (c *EdgeTtsRawConfig) safeTimeoutMs() int {
	if c == nil {
		return 0
	}
	return c.TimeoutMs
}
