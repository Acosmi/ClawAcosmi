// types/models.go — 模型定义相关类型
// 对应 TS 文件: src/config/types.models.ts
// 包含 ModelApi、ModelCost、ModelCompatConfig、ModelDefinitionConfig 等核心类型。
package types

// ModelApi 模型 API 协议标识符。
// 对应 TS 联合类型: ModelApi
type ModelApi string

const (
	ModelApiOpenAICompletions     ModelApi = "openai-completions"
	ModelApiOpenAIResponses       ModelApi = "openai-responses"
	ModelApiOpenAICodexResponses  ModelApi = "openai-codex-responses"
	ModelApiAnthropicMessages     ModelApi = "anthropic-messages"
	ModelApiGoogleGenerativeAI    ModelApi = "google-generative-ai"
	ModelApiGitHubCopilot         ModelApi = "github-copilot"
	ModelApiBedrockConverseStream ModelApi = "bedrock-converse-stream"
	ModelApiOllama                ModelApi = "ollama"
)

// ModelCost 模型调用费用（单位：美元/百万 token）。
// 对应 TS: ModelDefinitionConfig["cost"]
type ModelCost struct {
	// Input 输入 token 费用
	Input float64 `json:"input"`
	// Output 输出 token 费用
	Output float64 `json:"output"`
	// CacheRead 缓存读取费用
	CacheRead float64 `json:"cacheRead"`
	// CacheWrite 缓存写入费用
	CacheWrite float64 `json:"cacheWrite"`
}

// ModelCompatConfig 模型兼容性配置。
// 对应 TS: ModelCompatConfig
type ModelCompatConfig struct {
	// SupportsStore 是否支持 Store
	SupportsStore *bool `json:"supportsStore,omitempty"`
	// SupportsDeveloperRole 是否支持 developer 角色
	SupportsDeveloperRole *bool `json:"supportsDeveloperRole,omitempty"`
	// SupportsReasoningEffort 是否支持推理努力程度
	SupportsReasoningEffort *bool `json:"supportsReasoningEffort,omitempty"`
	// SupportsUsageInStreaming 是否支持流式中的使用量统计
	SupportsUsageInStreaming *bool `json:"supportsUsageInStreaming,omitempty"`
	// SupportsStrictMode 是否支持严格模式
	SupportsStrictMode *bool `json:"supportsStrictMode,omitempty"`
	// MaxTokensField max_tokens 字段名，可选 "max_completion_tokens" 或 "max_tokens"
	MaxTokensField string `json:"maxTokensField,omitempty"`
	// ThinkingFormat 思考格式，可选 "openai"、"zai"、"qwen"
	ThinkingFormat string `json:"thinkingFormat,omitempty"`
	// RequiresToolResultName 是否要求工具结果包含名称
	RequiresToolResultName *bool `json:"requiresToolResultName,omitempty"`
	// RequiresAssistantAfterToolResult 是否要求工具结果后有 assistant 消息
	RequiresAssistantAfterToolResult *bool `json:"requiresAssistantAfterToolResult,omitempty"`
	// RequiresThinkingAsText 是否要求思考内容作为文本
	RequiresThinkingAsText *bool `json:"requiresThinkingAsText,omitempty"`
	// RequiresMistralToolIds 是否要求 Mistral 风格的工具 ID
	RequiresMistralToolIds *bool `json:"requiresMistralToolIds,omitempty"`
}

// ModelProviderAuthMode 模型提供者认证模式。
// 对应 TS: ModelProviderAuthMode
type ModelProviderAuthMode string

const (
	ModelProviderAuthModeAPIKey ModelProviderAuthMode = "api-key"
	ModelProviderAuthModeAWS    ModelProviderAuthMode = "aws-sdk"
	ModelProviderAuthModeOAuth  ModelProviderAuthMode = "oauth"
	ModelProviderAuthModeToken  ModelProviderAuthMode = "token"
)

// ModelDefinitionConfig 模型定义配置。
// 对应 TS: ModelDefinitionConfig
type ModelDefinitionConfig struct {
	// ID 模型唯一标识
	ID string `json:"id"`
	// Name 模型显示名称
	Name string `json:"name"`
	// Api 模型 API 协议（可选）
	Api ModelApi `json:"api,omitempty"`
	// Reasoning 是否支持推理
	Reasoning bool `json:"reasoning"`
	// Input 支持的输入类型列表
	Input []string `json:"input"`
	// Cost 调用费用
	Cost ModelCost `json:"cost"`
	// ContextWindow 上下文窗口大小
	ContextWindow int `json:"contextWindow"`
	// MaxTokens 最大输出 token 数
	MaxTokens int `json:"maxTokens"`
	// Headers 自定义请求头（可选）
	Headers map[string]string `json:"headers,omitempty"`
	// Compat 兼容性配置（可选）
	Compat *ModelCompatConfig `json:"compat,omitempty"`
}

// ModelProviderConfig 模型提供者配置。
// 对应 TS: ModelProviderConfig
type ModelProviderConfig struct {
	// BaseURL 提供者 API 基础 URL
	BaseURL string `json:"baseUrl"`
	// ApiKey API 密钥（明文或 SecretRef）
	ApiKey interface{} `json:"apiKey,omitempty"`
	// Auth 认证模式
	Auth ModelProviderAuthMode `json:"auth,omitempty"`
	// Api 默认 API 协议
	Api ModelApi `json:"api,omitempty"`
	// InjectNumCtxForOpenAICompat 是否注入 num_ctx 参数
	InjectNumCtxForOpenAICompat *bool `json:"injectNumCtxForOpenAICompat,omitempty"`
	// Headers 自定义请求头
	Headers map[string]string `json:"headers,omitempty"`
	// AuthHeader 是否使用认证头
	AuthHeader *bool `json:"authHeader,omitempty"`
	// Models 模型定义列表
	Models []ModelDefinitionConfig `json:"models"`
}
