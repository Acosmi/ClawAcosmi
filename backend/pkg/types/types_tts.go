package types

// TTS (文本转语音) 类型 — 继承自 src/config/types.tts.ts

// TtsProvider TTS 供应商
type TtsProvider string

const (
	TtsElevenLabs TtsProvider = "elevenlabs"
	TtsOpenAI     TtsProvider = "openai"
	TtsEdge       TtsProvider = "edge"
)

// TtsMode TTS 应用模式
type TtsMode string

const (
	TtsModeFinal TtsMode = "final"
	TtsModeAll   TtsMode = "all"
)

// TtsAutoMode TTS 自动模式
type TtsAutoMode string

const (
	TtsAutoOff     TtsAutoMode = "off"
	TtsAutoAlways  TtsAutoMode = "always"
	TtsAutoInbound TtsAutoMode = "inbound"
	TtsAutoTagged  TtsAutoMode = "tagged"
)

// TtsModelOverrideConfig 模型覆盖 TTS 参数的权限配置
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

// TtsElevenLabsVoiceSettings ElevenLabs 语音参数
type TtsElevenLabsVoiceSettings struct {
	Stability       *float64 `json:"stability,omitempty"`
	SimilarityBoost *float64 `json:"similarityBoost,omitempty"`
	Style           *float64 `json:"style,omitempty"`
	UseSpeakerBoost *bool    `json:"useSpeakerBoost,omitempty"`
	Speed           *float64 `json:"speed,omitempty"`
}

// TtsElevenLabsConfig ElevenLabs TTS 配置
type TtsElevenLabsConfig struct {
	APIKey                 string                      `json:"apiKey,omitempty"`
	BaseURL                string                      `json:"baseUrl,omitempty"`
	VoiceID                string                      `json:"voiceId,omitempty"`
	ModelID                string                      `json:"modelId,omitempty"`
	Seed                   *int                        `json:"seed,omitempty"`
	ApplyTextNormalization string                      `json:"applyTextNormalization,omitempty"` // "auto"|"on"|"off"
	LanguageCode           string                      `json:"languageCode,omitempty"`
	VoiceSettings          *TtsElevenLabsVoiceSettings `json:"voiceSettings,omitempty"`
}

// TtsOpenAIConfig OpenAI TTS 配置
type TtsOpenAIConfig struct {
	APIKey string `json:"apiKey,omitempty"`
	Model  string `json:"model,omitempty"`
	Voice  string `json:"voice,omitempty"`
}

// TtsEdgeConfig Microsoft Edge TTS 配置
type TtsEdgeConfig struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	Voice         string `json:"voice,omitempty"`
	Lang          string `json:"lang,omitempty"`
	OutputFormat  string `json:"outputFormat,omitempty"`
	Pitch         string `json:"pitch,omitempty"`
	Rate          string `json:"rate,omitempty"`
	Volume        string `json:"volume,omitempty"`
	SaveSubtitles *bool  `json:"saveSubtitles,omitempty"`
	Proxy         string `json:"proxy,omitempty"`
	TimeoutMs     *int   `json:"timeoutMs,omitempty"`
}

// TtsConfig TTS 总配置
// 原版: export type TtsConfig
type TtsConfig struct {
	Auto           TtsAutoMode             `json:"auto,omitempty"`
	Enabled        *bool                   `json:"enabled,omitempty"` // 旧版兼容
	Mode           TtsMode                 `json:"mode,omitempty"`
	Provider       TtsProvider             `json:"provider,omitempty"`
	SummaryModel   string                  `json:"summaryModel,omitempty"`
	ModelOverrides *TtsModelOverrideConfig `json:"modelOverrides,omitempty"`
	ElevenLabs     *TtsElevenLabsConfig    `json:"elevenlabs,omitempty"`
	OpenAI         *TtsOpenAIConfig        `json:"openai,omitempty"`
	Edge           *TtsEdgeConfig          `json:"edge,omitempty"`
	PrefsPath      string                  `json:"prefsPath,omitempty"`
	MaxTextLength  *int                    `json:"maxTextLength,omitempty"`
	TimeoutMs      *int                    `json:"timeoutMs,omitempty"`
}
