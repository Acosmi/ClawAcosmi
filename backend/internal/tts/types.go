// Package tts 提供文字转语音 (TTS) 能力。
//
// TS 对照: tts/tts.ts (1580L) — 单文件拆分为 8 个 Go 文件
//
// 主要功能:
//   - 三 Provider 合成 (OpenAI, ElevenLabs, Edge)
//   - TTS 指令解析 ([[tts:...]])
//   - 配置解析与偏好管理
//   - 音频缓存与临时文件管理
package tts

import "time"

// ---------- 常量 ----------

const (
	// DefaultTimeoutMs 默认合成超时 (30 秒)。
	DefaultTimeoutMs = 30_000

	// DefaultTtsMaxLength 默认 TTS 最大长度。
	DefaultTtsMaxLength = 1500

	// DefaultTtsSummarize 默认是否启用摘要。
	DefaultTtsSummarize = true

	// DefaultMaxTextLength 默认最大文本长度。
	DefaultMaxTextLength = 4096

	// TempFileCleanupDelay 临时文件清理延迟。
	TempFileCleanupDelay = 5 * time.Minute

	// DefaultElevenLabsBaseURL ElevenLabs 默认 API 地址。
	DefaultElevenLabsBaseURL = "https://api.elevenlabs.io"

	// DefaultElevenLabsVoiceID ElevenLabs 默认语音 ID。
	DefaultElevenLabsVoiceID = "pMsXgVXv3BLzUgSXRplE"

	// DefaultElevenLabsModelID ElevenLabs 默认模型 ID。
	DefaultElevenLabsModelID = "eleven_multilingual_v2"

	// DefaultOpenAIModel OpenAI 默认 TTS 模型。
	DefaultOpenAIModel = "gpt-4o-mini-tts"

	// DefaultOpenAIVoice OpenAI 默认语音。
	DefaultOpenAIVoice = "alloy"

	// DefaultOpenAITtsBaseURL OpenAI TTS 默认 API 地址。
	// TS 对照: tts.ts L832 getOpenAITtsBaseUrl()
	DefaultOpenAITtsBaseURL = "https://api.openai.com/v1"

	// DefaultEdgeVoice Edge TTS 默认语音。
	DefaultEdgeVoice = "en-US-MichelleNeural"

	// DefaultEdgeLang Edge TTS 默认语言。
	DefaultEdgeLang = "en-US"

	// DefaultEdgeOutputFormat Edge TTS 默认输出格式。
	DefaultEdgeOutputFormat = "audio-24khz-48kbitrate-mono-mp3"
)

// ---------- TTS 类型枚举 ----------

// TtsAutoMode TTS 自动模式。
// TS 对照: tts.ts L82
type TtsAutoMode string

const (
	AutoOff     TtsAutoMode = "off"
	AutoAlways  TtsAutoMode = "always"
	AutoInbound TtsAutoMode = "inbound"
	AutoTagged  TtsAutoMode = "tagged"
)

// ValidAutoModes 有效的自动模式集合。
var ValidAutoModes = map[TtsAutoMode]bool{
	AutoOff:     true,
	AutoAlways:  true,
	AutoInbound: true,
	AutoTagged:  true,
}

// TtsMode TTS 生成模式。
type TtsMode string

const (
	ModeFinal  TtsMode = "final"
	ModeStream TtsMode = "stream"
)

// TtsProvider TTS Provider 类型。
type TtsProvider string

const (
	ProviderOpenAI     TtsProvider = "openai"
	ProviderElevenLabs TtsProvider = "elevenlabs"
	ProviderEdge       TtsProvider = "edge"
)

// TtsProviders 所有 Provider 列表。
// TS 对照: tts.ts L504
var TtsProviders = []TtsProvider{ProviderOpenAI, ProviderElevenLabs, ProviderEdge}

// ---------- ElevenLabs 语音设置 ----------

// ElevenLabsVoiceSettings ElevenLabs 语音参数。
// TS 对照: tts.ts L53-59
type ElevenLabsVoiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarityBoost"`
	Style           float64 `json:"style"`
	UseSpeakerBoost bool    `json:"useSpeakerBoost"`
	Speed           float64 `json:"speed"`
}

// DefaultElevenLabsVoiceSettings 默认语音设置。
// TS 对照: tts.ts L53-59
var DefaultElevenLabsVoiceSettings = ElevenLabsVoiceSettings{
	Stability:       0.5,
	SimilarityBoost: 0.75,
	Style:           0.0,
	UseSpeakerBoost: true,
	Speed:           1.0,
}

// ---------- 输出格式 ----------

// OutputFormat 输出格式配置。
// TS 对照: tts.ts L61-80
type OutputFormat struct {
	OpenAI          string
	ElevenLabs      string
	Extension       string
	VoiceCompatible bool
}

// TelegramOutput Telegram 输出格式。
// TS 对照: tts.ts L61-68
var TelegramOutput = OutputFormat{
	OpenAI:          "opus",
	ElevenLabs:      "opus_48000_64",
	Extension:       ".opus",
	VoiceCompatible: true,
}

// DefaultOutput 默认输出格式。
// TS 对照: tts.ts L70-75
var DefaultOutput = OutputFormat{
	OpenAI:          "mp3",
	ElevenLabs:      "mp3_44100_128",
	Extension:       ".mp3",
	VoiceCompatible: false,
}

// ---------- 已解析配置 ----------

// ResolvedTtsConfig 解析后的 TTS 配置。
// TS 对照: tts.ts L84-128
type ResolvedTtsConfig struct {
	Auto           TtsAutoMode               `json:"auto"`
	Mode           TtsMode                   `json:"mode"`
	Provider       TtsProvider               `json:"provider"`
	ProviderSource string                    `json:"providerSource"`
	SummaryModel   string                    `json:"summaryModel,omitempty"`
	ModelOverrides ResolvedTtsModelOverrides `json:"modelOverrides"`
	ElevenLabs     ElevenLabsConfig          `json:"elevenlabs"`
	OpenAI         OpenAITtsConfig           `json:"openai"`
	Edge           EdgeTtsConfig             `json:"edge"`
	PrefsPath      string                    `json:"prefsPath,omitempty"`
	MaxTextLength  int                       `json:"maxTextLength"`
	TimeoutMs      int                       `json:"timeoutMs"`
}

// ElevenLabsConfig ElevenLabs 配置。
type ElevenLabsConfig struct {
	APIKey                 string                  `json:"apiKey,omitempty"`
	BaseURL                string                  `json:"baseUrl"`
	VoiceID                string                  `json:"voiceId"`
	ModelID                string                  `json:"modelId"`
	Seed                   *int                    `json:"seed,omitempty"`
	ApplyTextNormalization string                  `json:"applyTextNormalization,omitempty"`
	LanguageCode           string                  `json:"languageCode,omitempty"`
	VoiceSettings          ElevenLabsVoiceSettings `json:"voiceSettings"`
}

// OpenAITtsConfig OpenAI TTS 配置。
type OpenAITtsConfig struct {
	APIKey  string `json:"apiKey,omitempty"`
	Model   string `json:"model"`
	Voice   string `json:"voice"`
	BaseURL string `json:"baseUrl,omitempty"` // 自定义端点 (e.g. LocalAI, Kokoro)
}

// EdgeTtsConfig Edge TTS 配置。
type EdgeTtsConfig struct {
	Enabled                bool   `json:"enabled"`
	Voice                  string `json:"voice"`
	Lang                   string `json:"lang"`
	OutputFormat           string `json:"outputFormat"`
	OutputFormatConfigured bool   `json:"outputFormatConfigured"`
	Pitch                  string `json:"pitch,omitempty"`
	Rate                   string `json:"rate,omitempty"`
	Volume                 string `json:"volume,omitempty"`
	SaveSubtitles          bool   `json:"saveSubtitles"`
	Proxy                  string `json:"proxy,omitempty"`
	TimeoutMs              int    `json:"timeoutMs,omitempty"`
}

// ---------- 模型覆盖策略 ----------

// ResolvedTtsModelOverrides 解析后的模型覆盖策略。
// TS 对照: tts.ts L140-149
type ResolvedTtsModelOverrides struct {
	Enabled            bool `json:"enabled"`
	AllowText          bool `json:"allowText"`
	AllowProvider      bool `json:"allowProvider"`
	AllowVoice         bool `json:"allowVoice"`
	AllowModelID       bool `json:"allowModelId"`
	AllowVoiceSettings bool `json:"allowVoiceSettings"`
	AllowNormalization bool `json:"allowNormalization"`
	AllowSeed          bool `json:"allowSeed"`
}

// ---------- 指令解析结果 ----------

// TtsDirectiveOverrides 指令覆盖。
// TS 对照: tts.ts L151-166
type TtsDirectiveOverrides struct {
	TtsText    string                      `json:"ttsText,omitempty"`
	Provider   TtsProvider                 `json:"provider,omitempty"`
	OpenAI     *TtsDirectiveOpenAIOverride `json:"openai,omitempty"`
	ElevenLabs *TtsDirectiveELOverride     `json:"elevenlabs,omitempty"`
}

// TtsDirectiveOpenAIOverride OpenAI 指令覆盖。
type TtsDirectiveOpenAIOverride struct {
	Voice string `json:"voice,omitempty"`
	Model string `json:"model,omitempty"`
}

// TtsDirectiveELOverride ElevenLabs 指令覆盖。
type TtsDirectiveELOverride struct {
	VoiceID                string                   `json:"voiceId,omitempty"`
	ModelID                string                   `json:"modelId,omitempty"`
	Seed                   *int                     `json:"seed,omitempty"`
	ApplyTextNormalization string                   `json:"applyTextNormalization,omitempty"`
	LanguageCode           string                   `json:"languageCode,omitempty"`
	VoiceSettings          *ElevenLabsVoiceSettings `json:"voiceSettings,omitempty"`
}

// TtsDirectiveParseResult 指令解析结果。
// TS 对照: tts.ts L168-174
type TtsDirectiveParseResult struct {
	CleanedText  string                `json:"cleanedText"`
	TtsText      string                `json:"ttsText,omitempty"`
	HasDirective bool                  `json:"hasDirective"`
	Overrides    TtsDirectiveOverrides `json:"overrides"`
	Warnings     []string              `json:"warnings,omitempty"`
}

// ---------- 合成结果 ----------

// TtsResult TTS 合成结果。
// TS 对照: tts.ts L176-184
type TtsResult struct {
	Success         bool   `json:"success"`
	AudioPath       string `json:"audioPath,omitempty"`
	Error           string `json:"error,omitempty"`
	LatencyMs       int64  `json:"latencyMs,omitempty"`
	Provider        string `json:"provider,omitempty"`
	OutputFormat    string `json:"outputFormat,omitempty"`
	VoiceCompatible bool   `json:"voiceCompatible,omitempty"`
}

// TtsTelephonyResult 电话 TTS 结果。
// TS 对照: tts.ts L186-194
type TtsTelephonyResult struct {
	Success      bool   `json:"success"`
	AudioBuffer  []byte `json:"audioBuffer,omitempty"`
	Error        string `json:"error,omitempty"`
	LatencyMs    int64  `json:"latencyMs,omitempty"`
	Provider     string `json:"provider,omitempty"`
	OutputFormat string `json:"outputFormat,omitempty"`
	SampleRate   int    `json:"sampleRate,omitempty"`
}

// ---------- 状态跟踪 ----------

// TtsStatusEntry TTS 状态条目。
// TS 对照: tts.ts L196-204
type TtsStatusEntry struct {
	Timestamp  int64  `json:"timestamp"`
	Success    bool   `json:"success"`
	TextLength int    `json:"textLength"`
	Summarized bool   `json:"summarized"`
	Provider   string `json:"provider,omitempty"`
	LatencyMs  int64  `json:"latencyMs,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ---------- 偏好类型 ----------

// TtsUserPrefs 用户 TTS 偏好。
// TS 对照: tts.ts L130-138
type TtsUserPrefs struct {
	Tts *TtsUserPrefsInner `json:"tts,omitempty"`
}

// TtsUserPrefsInner 用户 TTS 偏好内部结构。
type TtsUserPrefsInner struct {
	Auto      TtsAutoMode `json:"auto,omitempty"`
	Enabled   *bool       `json:"enabled,omitempty"`
	Provider  TtsProvider `json:"provider,omitempty"`
	MaxLength int         `json:"maxLength,omitempty"`
	Summarize *bool       `json:"summarize,omitempty"`
}
