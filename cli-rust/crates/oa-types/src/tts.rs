/// TTS (Text-to-Speech) configuration types.
///
/// Source: `src/config/types.tts.ts`

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum TtsProvider {
    Elevenlabs,
    Openai,
    Edge,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum TtsMode {
    Final,
    All,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum TtsAutoMode {
    Off,
    Always,
    Inbound,
    Tagged,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct TtsModelOverrideConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_text: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_provider: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_voice: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_model_id: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_voice_settings: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_normalization: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub allow_seed: Option<bool>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ElevenLabsTextNormalization {
    Auto,
    On,
    Off,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ElevenLabsVoiceSettings {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub stability: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub similarity_boost: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub style: Option<f64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub use_speaker_boost: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub speed: Option<f64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct TtsElevenLabsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub base_url: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub seed: Option<i64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub apply_text_normalization: Option<ElevenLabsTextNormalization>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub language_code: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice_settings: Option<ElevenLabsVoiceSettings>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct TtsOpenaiConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct TtsEdgeConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub voice: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub lang: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub output_format: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub pitch: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub rate: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub volume: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub save_subtitles: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub proxy: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
}

/// Top-level TTS configuration.
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct TtsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auto: Option<TtsAutoMode>,
    /// Legacy: enable auto-TTS when `auto` is not set.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<TtsMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provider: Option<TtsProvider>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub summary_model: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_overrides: Option<TtsModelOverrideConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub elevenlabs: Option<TtsElevenLabsConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub openai: Option<TtsOpenaiConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub edge: Option<TtsEdgeConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub prefs_path: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_text_length: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub timeout_ms: Option<u64>,
}
