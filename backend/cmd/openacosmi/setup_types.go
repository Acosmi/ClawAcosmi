package main

// setup_types.go — Setup/Onboarding 类型定义
// 对应 TS src/commands/onboard-types.ts (106L) + auth-choice-options.ts (272L)

import (
	"github.com/anthropic/open-acosmi/internal/agents/auth"
	"github.com/anthropic/open-acosmi/internal/tui"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- AuthChoice 常量 ----------

// AuthChoice 认证方式标识（对应 TS AuthChoice 联合类型）。
type AuthChoice = string

const (
	AuthChoiceOAuth                  AuthChoice = "oauth"
	AuthChoiceSetupToken             AuthChoice = "setup-token"
	AuthChoiceClaudeCli              AuthChoice = "claude-cli"
	AuthChoiceToken                  AuthChoice = "token"
	AuthChoiceChutes                 AuthChoice = "chutes"
	AuthChoiceOpenAICodex            AuthChoice = "openai-codex"
	AuthChoiceOpenAIApiKey           AuthChoice = "openai-api-key"
	AuthChoiceOpenRouterApiKey       AuthChoice = "openrouter-api-key"
	AuthChoiceAIGatewayApiKey        AuthChoice = "ai-gateway-api-key"
	AuthChoiceCloudflareAIGatewayKey AuthChoice = "cloudflare-ai-gateway-api-key"
	AuthChoiceMoonshotApiKey         AuthChoice = "moonshot-api-key"
	AuthChoiceMoonshotApiKeyCn       AuthChoice = "moonshot-api-key-cn"
	AuthChoiceKimiCodeApiKey         AuthChoice = "kimi-code-api-key"
	AuthChoiceSyntheticApiKey        AuthChoice = "synthetic-api-key"
	AuthChoiceVeniceApiKey           AuthChoice = "venice-api-key"
	AuthChoiceCodexCli               AuthChoice = "codex-cli"
	AuthChoiceApiKey                 AuthChoice = "apiKey"
	AuthChoiceGeminiApiKey           AuthChoice = "gemini-api-key"
	AuthChoiceGoogleAntigravity      AuthChoice = "google-antigravity"
	AuthChoiceGoogleGeminiCli        AuthChoice = "google-gemini-cli"
	AuthChoiceZaiApiKey              AuthChoice = "zai-api-key"
	AuthChoiceXiaomiApiKey           AuthChoice = "xiaomi-api-key"
	AuthChoiceMinimaxCloud           AuthChoice = "minimax-cloud"
	AuthChoiceMinimax                AuthChoice = "minimax"
	AuthChoiceMinimaxApi             AuthChoice = "minimax-api"
	AuthChoiceMinimaxApiLightning    AuthChoice = "minimax-api-lightning"
	AuthChoiceMinimaxPortal          AuthChoice = "minimax-portal"
	AuthChoiceAcosmiZen              AuthChoice = "openacosmi-zen"
	AuthChoiceGitHubCopilot          AuthChoice = "github-copilot"
	AuthChoiceCopilotProxy           AuthChoice = "copilot-proxy"
	AuthChoiceQwenPortal             AuthChoice = "qwen-portal"
	AuthChoiceXAIApiKey              AuthChoice = "xai-api-key"
	AuthChoiceQianfanApiKey          AuthChoice = "qianfan-api-key"
	AuthChoiceSkip                   AuthChoice = "skip"
)

// ---------- AuthChoice 分组 ----------

// AuthChoiceGroupID 提供商分组标识。
type AuthChoiceGroupID = string

const (
	GroupOpenAI              AuthChoiceGroupID = "openai"
	GroupAnthropic           AuthChoiceGroupID = "anthropic"
	GroupGoogle              AuthChoiceGroupID = "google"
	GroupCopilot             AuthChoiceGroupID = "copilot"
	GroupOpenRouter          AuthChoiceGroupID = "openrouter"
	GroupAIGateway           AuthChoiceGroupID = "ai-gateway"
	GroupCloudflareAIGateway AuthChoiceGroupID = "cloudflare-ai-gateway"
	GroupMoonshot            AuthChoiceGroupID = "moonshot"
	GroupZAI                 AuthChoiceGroupID = "zai"
	GroupXiaomi              AuthChoiceGroupID = "xiaomi"
	GroupAcosmiZen           AuthChoiceGroupID = "openacosmi-zen"
	GroupMinimax             AuthChoiceGroupID = "minimax"
	GroupSynthetic           AuthChoiceGroupID = "synthetic"
	GroupVenice              AuthChoiceGroupID = "venice"
	GroupQwen                AuthChoiceGroupID = "qwen"
	GroupQianfan             AuthChoiceGroupID = "qianfan"
	GroupXAI                 AuthChoiceGroupID = "xai"
)

// AuthChoiceOption 认证选项（对应 TS AuthChoiceOption）。
type AuthChoiceOption struct {
	Value AuthChoice
	Label string
	Hint  string
}

// AuthChoiceGroup 认证选项组（对应 TS AuthChoiceGroup）。
type AuthChoiceGroup struct {
	Value   AuthChoiceGroupID
	Label   string
	Hint    string
	Options []AuthChoiceOption
}

// ---------- Apply 参数/结果 ----------

// ApplyAuthChoiceParams 应用认证选择的参数。
type ApplyAuthChoiceParams struct {
	AuthChoice      AuthChoice
	Config          *types.OpenAcosmiConfig
	Prompter        tui.WizardPrompter
	AuthStore       *auth.AuthStore
	AgentDir        string
	SetDefaultModel bool
	AgentID         string
	Opts            *ApplyAuthChoiceOpts
}

// ApplyAuthChoiceOpts 可选参数。
type ApplyAuthChoiceOpts struct {
	TokenProvider                string
	Token                        string
	CloudflareAIGatewayAccountID string
	CloudflareAIGatewayGatewayID string
	CloudflareAIGatewayApiKey    string
	XAIApiKey                    string
}

// ApplyAuthChoiceResult 应用认证选择的结果。
type ApplyAuthChoiceResult struct {
	Config             *types.OpenAcosmiConfig
	AgentModelOverride string
}

// ---------- Onboard 类型别名 ----------

// OnboardMode 引导模式。
type OnboardMode = string

const (
	OnboardModeLocal  OnboardMode = "local"
	OnboardModeRemote OnboardMode = "remote"
)

// GatewayAuthChoice 网关认证选择。
type GatewayAuthChoice = string

const (
	GatewayAuthChoiceToken    GatewayAuthChoice = "token"
	GatewayAuthChoicePassword GatewayAuthChoice = "password"
)

// GatewayBind 网关绑定方式。
type GatewayBind = string

const (
	GatewayBindLoopback GatewayBind = "loopback"
	GatewayBindLan      GatewayBind = "lan"
	GatewayBindAuto     GatewayBind = "auto"
	GatewayBindCustom   GatewayBind = "custom"
	GatewayBindTailnet  GatewayBind = "tailnet"
)

// TailscaleMode Tailscale 模式。
type TailscaleMode = string

const (
	TailscaleModeOff    TailscaleMode = "off"
	TailscaleModeServe  TailscaleMode = "serve"
	TailscaleModeFunnel TailscaleMode = "funnel"
)

// NodeManagerChoice 节点管理器。
type NodeManagerChoice = string

const (
	NodeManagerNpm  NodeManagerChoice = "npm"
	NodeManagerPnpm NodeManagerChoice = "pnpm"
	NodeManagerBun  NodeManagerChoice = "bun"
)

// ---------- Setup/Onboard 选项 ----------

// OnboardOptions 引导命令完整选项（对应 TS OnboardOptions）。
type OnboardOptions struct {
	// General
	Mode           OnboardMode `json:"mode,omitempty"`
	Flow           string      `json:"flow,omitempty"` // "quickstart"|"advanced"|"manual"
	Workspace      string      `json:"workspace,omitempty"`
	NonInteractive bool        `json:"nonInteractive,omitempty"`
	AcceptRisk     bool        `json:"acceptRisk,omitempty"`
	Reset          bool        `json:"reset,omitempty"`
	AuthChoice     AuthChoice  `json:"authChoice,omitempty"`

	// Token auth
	TokenProvider  string `json:"tokenProvider,omitempty"`
	Token          string `json:"token,omitempty"`
	TokenProfileID string `json:"tokenProfileId,omitempty"`
	TokenExpiresIn string `json:"tokenExpiresIn,omitempty"`

	// Provider API keys
	AnthropicApiKey              string `json:"anthropicApiKey,omitempty"`
	OpenAIApiKey                 string `json:"openaiApiKey,omitempty"`
	OpenRouterApiKey             string `json:"openrouterApiKey,omitempty"`
	AIGatewayApiKey              string `json:"aiGatewayApiKey,omitempty"`
	CloudflareAIGatewayAccountID string `json:"cloudflareAiGatewayAccountId,omitempty"`
	CloudflareAIGatewayGatewayID string `json:"cloudflareAiGatewayGatewayId,omitempty"`
	CloudflareAIGatewayApiKey    string `json:"cloudflareAiGatewayApiKey,omitempty"`
	MoonshotApiKey               string `json:"moonshotApiKey,omitempty"`
	KimiCodeApiKey               string `json:"kimiCodeApiKey,omitempty"`
	GeminiApiKey                 string `json:"geminiApiKey,omitempty"`
	ZaiApiKey                    string `json:"zaiApiKey,omitempty"`
	XiaomiApiKey                 string `json:"xiaomiApiKey,omitempty"`
	MinimaxApiKey                string `json:"minimaxApiKey,omitempty"`
	SyntheticApiKey              string `json:"syntheticApiKey,omitempty"`
	VeniceApiKey                 string `json:"veniceApiKey,omitempty"`
	AcosmiZenApiKey              string `json:"openacosmiZenApiKey,omitempty"`
	XAIApiKey                    string `json:"xaiApiKey,omitempty"`
	QianfanApiKey                string `json:"qianfanApiKey,omitempty"`

	// Gateway
	GatewayPort     int               `json:"gatewayPort,omitempty"`
	GatewayBind     GatewayBind       `json:"gatewayBind,omitempty"`
	GatewayAuth     GatewayAuthChoice `json:"gatewayAuth,omitempty"`
	GatewayToken    string            `json:"gatewayToken,omitempty"`
	GatewayPassword string            `json:"gatewayPassword,omitempty"`

	// Tailscale
	Tailscale            TailscaleMode `json:"tailscale,omitempty"`
	TailscaleResetOnExit bool          `json:"tailscaleResetOnExit,omitempty"`

	// Misc
	InstallDaemon bool              `json:"installDaemon,omitempty"`
	SkipChannels  bool              `json:"skipChannels,omitempty"`
	SkipSkills    bool              `json:"skipSkills,omitempty"`
	SkipHealth    bool              `json:"skipHealth,omitempty"`
	SkipUI        bool              `json:"skipUi,omitempty"`
	NodeManager   NodeManagerChoice `json:"nodeManager,omitempty"`
	JSON          bool              `json:"json,omitempty"`

	// Remote
	RemoteURL   string `json:"remoteUrl,omitempty"`
	RemoteToken string `json:"remoteToken,omitempty"`
}

// SetupOptions setup 命令选项（对应 TS OnboardOptions 子集）。
// 向后兼容 — 使用 OnboardOptions 为首选。
type SetupOptions struct {
	Workspace      string
	NonInteractive bool
	Provider       string
	// 非交互模式下的 API Key
	AnthropicApiKey string
	OpenAIApiKey    string
	GeminiApiKey    string
}
