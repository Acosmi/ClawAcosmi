// types/onboard_options.go — OnboardOptions 结构体
// 对应 TS 文件: src/commands/onboard-types.ts 中的 OnboardOptions 类型
package types

// OnboardOptions 引导配置选项。
// 对应 TS: OnboardOptions
// 所有可选字段使用指针类型表示 undefined/omitted 语义。
type OnboardOptions struct {
	// Mode 引导模式
	Mode *OnboardMode `json:"mode,omitempty"`
	// Flow 引导流程（"manual" 是 "advanced" 的别名）
	Flow *OnboardFlow `json:"flow,omitempty"`
	// Workspace 工作区路径
	Workspace *string `json:"workspace,omitempty"`
	// NonInteractive 非交互模式
	NonInteractive *bool `json:"nonInteractive,omitempty"`
	// AcceptRisk 非交互引导时跳过风险提示（必须为 true）
	AcceptRisk *bool `json:"acceptRisk,omitempty"`
	// Reset 是否重置
	Reset *bool `json:"reset,omitempty"`
	// ResetScope 重置范围
	ResetScope *ResetScope `json:"resetScope,omitempty"`
	// AuthChoice 认证选择
	AuthChoice *AuthChoice `json:"authChoice,omitempty"`

	// --- 令牌相关（非交互模式下 authChoice=token 时使用） ---
	// TokenProvider 令牌提供者
	TokenProvider *string `json:"tokenProvider,omitempty"`
	// Token 令牌值
	Token *string `json:"token,omitempty"`
	// TokenProfileID 令牌 Profile ID
	TokenProfileID *string `json:"tokenProfileId,omitempty"`
	// TokenExpiresIn 令牌过期时间
	TokenExpiresIn *string `json:"tokenExpiresIn,omitempty"`

	// --- API 密钥持久化模式 ---
	// SecretInputMode API 密钥存储模式，默认 plaintext
	SecretInputMode *SecretInputMode `json:"secretInputMode,omitempty"`

	// --- 各提供者 API 密钥 ---
	AnthropicAPIKey              *string `json:"anthropicApiKey,omitempty"`
	OpenAIAPIKey                 *string `json:"openaiApiKey,omitempty"`
	MistralAPIKey                *string `json:"mistralApiKey,omitempty"`
	OpenRouterAPIKey             *string `json:"openrouterApiKey,omitempty"`
	KilocodeAPIKey               *string `json:"kilocodeApiKey,omitempty"`
	LiteLLMAPIKey                *string `json:"litellmApiKey,omitempty"`
	AIGatewayAPIKey              *string `json:"aiGatewayApiKey,omitempty"`
	CloudflareAIGatewayAccountID *string `json:"cloudflareAiGatewayAccountId,omitempty"`
	CloudflareAIGatewayGatewayID *string `json:"cloudflareAiGatewayGatewayId,omitempty"`
	CloudflareAIGatewayAPIKey    *string `json:"cloudflareAiGatewayApiKey,omitempty"`
	MoonshotAPIKey               *string `json:"moonshotApiKey,omitempty"`
	KimiCodeAPIKey               *string `json:"kimiCodeApiKey,omitempty"`
	GeminiAPIKey                 *string `json:"geminiApiKey,omitempty"`
	ZAIAPIKey                    *string `json:"zaiApiKey,omitempty"`
	XiaomiAPIKey                 *string `json:"xiaomiApiKey,omitempty"`
	MinimaxAPIKey                *string `json:"minimaxApiKey,omitempty"`
	SyntheticAPIKey              *string `json:"syntheticApiKey,omitempty"`
	VeniceAPIKey                 *string `json:"veniceApiKey,omitempty"`
	TogetherAPIKey               *string `json:"togetherApiKey,omitempty"`
	HuggingFaceAPIKey            *string `json:"huggingfaceApiKey,omitempty"`
	OpenCodeZenAPIKey            *string `json:"opencodeZenApiKey,omitempty"`
	XAIAPIKey                    *string `json:"xaiApiKey,omitempty"`
	VolcengineAPIKey             *string `json:"volcengineApiKey,omitempty"`
	BytePlusAPIKey               *string `json:"byteplusApiKey,omitempty"`
	QianfanAPIKey                *string `json:"qianfanApiKey,omitempty"`

	// --- 自定义提供者 ---
	CustomBaseURL       *string              `json:"customBaseUrl,omitempty"`
	CustomAPIKey        *string              `json:"customApiKey,omitempty"`
	CustomModelID       *string              `json:"customModelId,omitempty"`
	CustomProviderID    *string              `json:"customProviderId,omitempty"`
	CustomCompatibility *CustomCompatibility `json:"customCompatibility,omitempty"`

	// --- 网关配置 ---
	GatewayPort          *int                  `json:"gatewayPort,omitempty"`
	GatewayBind          *GatewayBind          `json:"gatewayBind,omitempty"`
	GatewayAuth          *GatewayAuthChoice    `json:"gatewayAuth,omitempty"`
	GatewayToken         *string               `json:"gatewayToken,omitempty"`
	GatewayPassword      *string               `json:"gatewayPassword,omitempty"`
	Tailscale            *TailscaleMode        `json:"tailscale,omitempty"`
	TailscaleResetOnExit *bool                 `json:"tailscaleResetOnExit,omitempty"`
	InstallDaemon        *bool                 `json:"installDaemon,omitempty"`
	DaemonRuntime        *GatewayDaemonRuntime `json:"daemonRuntime,omitempty"`

	// --- 跳过选项 ---
	SkipChannels  *bool `json:"skipChannels,omitempty"`
	SkipProviders *bool `json:"skipProviders,omitempty"` // 遗留别名
	SkipSkills    *bool `json:"skipSkills,omitempty"`
	SkipHealth    *bool `json:"skipHealth,omitempty"`
	SkipUI        *bool `json:"skipUI,omitempty"`

	// --- 其他 ---
	NodeManager *NodeManagerChoice `json:"nodeManager,omitempty"`
	RemoteURL   *string            `json:"remoteUrl,omitempty"`
	RemoteToken *string            `json:"remoteToken,omitempty"`
	JSON        *bool              `json:"json,omitempty"`
}
