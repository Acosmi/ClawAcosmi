/// Model configuration types.
///
/// Source: `src/config/types.models.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ModelApi {
    #[serde(rename = "openai-completions")]
    OpenaiCompletions,
    #[serde(rename = "openai-responses")]
    OpenaiResponses,
    #[serde(rename = "anthropic-messages")]
    AnthropicMessages,
    #[serde(rename = "google-generative-ai")]
    GoogleGenerativeAi,
    #[serde(rename = "github-copilot")]
    GithubCopilot,
    #[serde(rename = "bedrock-converse-stream")]
    BedrockConverseStream,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum MaxTokensField {
    #[serde(rename = "max_completion_tokens")]
    MaxCompletionTokens,
    #[serde(rename = "max_tokens")]
    MaxTokens,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ModelCompatConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub supports_store: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub supports_developer_role: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub supports_reasoning_effort: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub max_tokens_field: Option<MaxTokensField>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ModelProviderAuthMode {
    #[serde(rename = "api-key")]
    ApiKey,
    #[serde(rename = "aws-sdk")]
    AwsSdk,
    #[serde(rename = "oauth")]
    Oauth,
    #[serde(rename = "token")]
    Token,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ModelInputType {
    Text,
    Image,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ModelCostConfig {
    pub input: f64,
    pub output: f64,
    pub cache_read: f64,
    pub cache_write: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ModelDefinitionConfig {
    pub id: String,
    pub name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api: Option<ModelApi>,
    pub reasoning: bool,
    pub input: Vec<ModelInputType>,
    pub cost: ModelCostConfig,
    pub context_window: u64,
    pub max_tokens: u64,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub compat: Option<ModelCompatConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ModelProviderConfig {
    pub base_url: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_key: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth: Option<ModelProviderAuthMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api: Option<ModelApi>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub headers: Option<HashMap<String, String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth_header: Option<bool>,
    pub models: Vec<ModelDefinitionConfig>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct BedrockDiscoveryConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub enabled: Option<bool>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub region: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub provider_filter: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub refresh_interval: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default_context_window: Option<u64>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub default_max_tokens: Option<u64>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ModelsMode {
    Merge,
    Replace,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct ModelsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<ModelsMode>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub providers: Option<HashMap<String, ModelProviderConfig>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub bedrock_discovery: Option<BedrockDiscoveryConfig>,
}
