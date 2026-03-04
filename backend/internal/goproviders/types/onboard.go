// types/onboard.go — Onboard 相关类型和枚举常量
// 对应 TS 文件: src/commands/onboard-types.ts
// 拆分为两个文件：本文件定义枚举常量，onboard_options.go 定义 OnboardOptions 结构体
package types

// AuthChoice 认证选择标识符。
// 对应 TS 联合类型: AuthChoice（47个可选值）
type AuthChoice string

const (
	AuthChoiceOAuth                  AuthChoice = "oauth"
	AuthChoiceSetupToken             AuthChoice = "setup-token"
	AuthChoiceClaudeCLI              AuthChoice = "claude-cli"
	AuthChoiceToken                  AuthChoice = "token"
	AuthChoiceChutes                 AuthChoice = "chutes"
	AuthChoiceVLLM                   AuthChoice = "vllm"
	AuthChoiceOpenAICodex            AuthChoice = "openai-codex"
	AuthChoiceOpenAIAPIKey           AuthChoice = "openai-api-key"
	AuthChoiceOpenRouterAPIKey       AuthChoice = "openrouter-api-key"
	AuthChoiceKilocodeAPIKey         AuthChoice = "kilocode-api-key"
	AuthChoiceLiteLLMAPIKey          AuthChoice = "litellm-api-key"
	AuthChoiceAIGatewayAPIKey        AuthChoice = "ai-gateway-api-key"
	AuthChoiceCloudflareAIGatewayKey AuthChoice = "cloudflare-ai-gateway-api-key"
	AuthChoiceMoonshotAPIKey         AuthChoice = "moonshot-api-key"
	AuthChoiceMoonshotAPIKeyCN       AuthChoice = "moonshot-api-key-cn"
	AuthChoiceKimiCodeAPIKey         AuthChoice = "kimi-code-api-key"
	AuthChoiceSyntheticAPIKey        AuthChoice = "synthetic-api-key"
	AuthChoiceVeniceAPIKey           AuthChoice = "venice-api-key"
	AuthChoiceTogetherAPIKey         AuthChoice = "together-api-key"
	AuthChoiceHuggingFaceAPIKey      AuthChoice = "huggingface-api-key"
	AuthChoiceCodexCLI               AuthChoice = "codex-cli"
	AuthChoiceAPIKey                 AuthChoice = "apiKey"
	AuthChoiceGeminiAPIKey           AuthChoice = "gemini-api-key"
	AuthChoiceGoogleGeminiCLI        AuthChoice = "google-gemini-cli"
	AuthChoiceZAIAPIKey              AuthChoice = "zai-api-key"
	AuthChoiceZAICodingGlobal        AuthChoice = "zai-coding-global"
	AuthChoiceZAICodingCN            AuthChoice = "zai-coding-cn"
	AuthChoiceZAIGlobal              AuthChoice = "zai-global"
	AuthChoiceZAICN                  AuthChoice = "zai-cn"
	AuthChoiceXiaomiAPIKey           AuthChoice = "xiaomi-api-key"
	AuthChoiceMinimaxCloud           AuthChoice = "minimax-cloud"
	AuthChoiceMinimax                AuthChoice = "minimax"
	AuthChoiceMinimaxAPI             AuthChoice = "minimax-api"
	AuthChoiceMinimaxAPIKeyCN        AuthChoice = "minimax-api-key-cn"
	AuthChoiceMinimaxAPILightning    AuthChoice = "minimax-api-lightning"
	AuthChoiceMinimaxPortal          AuthChoice = "minimax-portal"
	AuthChoiceOpenCodeZen            AuthChoice = "opencode-zen"
	AuthChoiceGitHubCopilot          AuthChoice = "github-copilot"
	AuthChoiceCopilotProxy           AuthChoice = "copilot-proxy"
	AuthChoiceQwenPortal             AuthChoice = "qwen-portal"
	AuthChoiceXAIAPIKey              AuthChoice = "xai-api-key"
	AuthChoiceMistralAPIKey          AuthChoice = "mistral-api-key"
	AuthChoiceVolcengineAPIKey       AuthChoice = "volcengine-api-key"
	AuthChoiceBytePlusAPIKey         AuthChoice = "byteplus-api-key"
	AuthChoiceQianfanAPIKey          AuthChoice = "qianfan-api-key"
	AuthChoiceCustomAPIKey           AuthChoice = "custom-api-key"
	AuthChoiceSkip                   AuthChoice = "skip"
)

// AuthChoiceGroupID 认证选择分组标识符。
// 对应 TS 联合类型: AuthChoiceGroupId（27个可选值）
type AuthChoiceGroupID string

const (
	AuthGroupOpenAI              AuthChoiceGroupID = "openai"
	AuthGroupAnthropic           AuthChoiceGroupID = "anthropic"
	AuthGroupChutes              AuthChoiceGroupID = "chutes"
	AuthGroupVLLM                AuthChoiceGroupID = "vllm"
	AuthGroupGoogle              AuthChoiceGroupID = "google"
	AuthGroupCopilot             AuthChoiceGroupID = "copilot"
	AuthGroupOpenRouter          AuthChoiceGroupID = "openrouter"
	AuthGroupKilocode            AuthChoiceGroupID = "kilocode"
	AuthGroupLiteLLM             AuthChoiceGroupID = "litellm"
	AuthGroupAIGateway           AuthChoiceGroupID = "ai-gateway"
	AuthGroupCloudflareAIGateway AuthChoiceGroupID = "cloudflare-ai-gateway"
	AuthGroupMoonshot            AuthChoiceGroupID = "moonshot"
	AuthGroupZAI                 AuthChoiceGroupID = "zai"
	AuthGroupXiaomi              AuthChoiceGroupID = "xiaomi"
	AuthGroupOpenCodeZen         AuthChoiceGroupID = "opencode-zen"
	AuthGroupMinimax             AuthChoiceGroupID = "minimax"
	AuthGroupSynthetic           AuthChoiceGroupID = "synthetic"
	AuthGroupVenice              AuthChoiceGroupID = "venice"
	AuthGroupMistral             AuthChoiceGroupID = "mistral"
	AuthGroupQwen                AuthChoiceGroupID = "qwen"
	AuthGroupTogether            AuthChoiceGroupID = "together"
	AuthGroupHuggingFace         AuthChoiceGroupID = "huggingface"
	AuthGroupQianfan             AuthChoiceGroupID = "qianfan"
	AuthGroupXAI                 AuthChoiceGroupID = "xai"
	AuthGroupVolcengine          AuthChoiceGroupID = "volcengine"
	AuthGroupBytePlus            AuthChoiceGroupID = "byteplus"
	AuthGroupCustom              AuthChoiceGroupID = "custom"
)

// OnboardMode 引导模式。
type OnboardMode string

const (
	OnboardModeLocal  OnboardMode = "local"
	OnboardModeRemote OnboardMode = "remote"
)

// GatewayAuthChoice 网关认证选择。
type GatewayAuthChoice string

const (
	GatewayAuthToken    GatewayAuthChoice = "token"
	GatewayAuthPassword GatewayAuthChoice = "password"
)

// ResetScope 重置范围。
type ResetScope string

const (
	ResetScopeConfig      ResetScope = "config"
	ResetScopeConfigCreds ResetScope = "config+creds+sessions"
	ResetScopeFull        ResetScope = "full"
)

// GatewayBind 网关绑定模式。
type GatewayBind string

const (
	GatewayBindLoopback GatewayBind = "loopback"
	GatewayBindLAN      GatewayBind = "lan"
	GatewayBindAuto     GatewayBind = "auto"
	GatewayBindCustom   GatewayBind = "custom"
	GatewayBindTailnet  GatewayBind = "tailnet"
)

// TailscaleMode Tailscale 模式。
type TailscaleMode string

const (
	TailscaleModeOff    TailscaleMode = "off"
	TailscaleModeServe  TailscaleMode = "serve"
	TailscaleModeFunnel TailscaleMode = "funnel"
)

// NodeManagerChoice 包管理器选择。
type NodeManagerChoice string

const (
	NodeManagerNPM  NodeManagerChoice = "npm"
	NodeManagerPNPM NodeManagerChoice = "pnpm"
	NodeManagerBun  NodeManagerChoice = "bun"
)

// GatewayDaemonRuntime 网关守护进程运行时。
// 对应 TS 文件: src/commands/daemon-runtime.ts
type GatewayDaemonRuntime string

const (
	GatewayDaemonRuntimeNode GatewayDaemonRuntime = "node"
	GatewayDaemonRuntimeBun  GatewayDaemonRuntime = "bun"
)

// DefaultGatewayDaemonRuntime 默认网关守护进程运行时。
const DefaultGatewayDaemonRuntime = GatewayDaemonRuntimeNode

// IsGatewayDaemonRuntime 检查给定值是否是合法的网关守护进程运行时。
func IsGatewayDaemonRuntime(value string) bool {
	return value == string(GatewayDaemonRuntimeNode) || value == string(GatewayDaemonRuntimeBun)
}

// SecretInputMode 密钥输入模式。
type SecretInputMode string

const (
	SecretInputModePlaintext SecretInputMode = "plaintext"
	SecretInputModeRef       SecretInputMode = "ref"
)

// ChannelChoice 频道选择（等同于字符串类型，对应 TS: ChannelId）。
type ChannelChoice = string

// ProviderChoice 遗留别名（等同于 ChannelChoice）。
// 对应 TS: ProviderChoice = ChannelChoice
type ProviderChoice = ChannelChoice

// OnboardFlow 引导流程。
type OnboardFlow string

const (
	OnboardFlowQuickstart OnboardFlow = "quickstart"
	OnboardFlowAdvanced   OnboardFlow = "advanced"
	OnboardFlowManual     OnboardFlow = "manual" // "manual" 是 "advanced" 的别名
)

// CustomCompatibility 自定义提供者兼容性模式。
type CustomCompatibility string

const (
	CustomCompatibilityOpenAI    CustomCompatibility = "openai"
	CustomCompatibilityAnthropic CustomCompatibility = "anthropic"
)
