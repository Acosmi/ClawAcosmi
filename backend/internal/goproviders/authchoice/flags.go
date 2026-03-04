// authchoice/flags.go — Provider 认证 CLI 标志定义
// 对应 TS 文件: src/commands/onboard-provider-auth-flags.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// OnboardProviderAuthFlag Provider API 密钥的 CLI 标志信息。
// 对应 TS: OnboardProviderAuthFlag
type OnboardProviderAuthFlag struct {
	// OptionKey 对应 OnboardOptions 中的字段名
	OptionKey string
	// AuthChoice 关联的认证选择
	AuthChoice types.AuthChoice
	// CLIFlag CLI 标志名称（如 "--anthropic-api-key"）
	CLIFlag string
	// CLIOption CLI 选项完整形式（如 "--anthropic-api-key <key>"）
	CLIOption string
	// Description 人类可读描述
	Description string
}

// OnboardProviderAuthFlags 所有 Provider API 密钥的 CLI 标志定义。
// 对应 TS: ONBOARD_PROVIDER_AUTH_FLAGS
// 用于 CLI 注册和非交互式推理。
var OnboardProviderAuthFlags = []OnboardProviderAuthFlag{
	{
		OptionKey:   "anthropicApiKey",
		AuthChoice:  types.AuthChoiceAPIKey,
		CLIFlag:     "--anthropic-api-key",
		CLIOption:   "--anthropic-api-key <key>",
		Description: "Anthropic API key",
	},
	{
		OptionKey:   "openaiApiKey",
		AuthChoice:  types.AuthChoiceOpenAIAPIKey,
		CLIFlag:     "--openai-api-key",
		CLIOption:   "--openai-api-key <key>",
		Description: "OpenAI API key",
	},
	{
		OptionKey:   "mistralApiKey",
		AuthChoice:  types.AuthChoiceMistralAPIKey,
		CLIFlag:     "--mistral-api-key",
		CLIOption:   "--mistral-api-key <key>",
		Description: "Mistral API key",
	},
	{
		OptionKey:   "openrouterApiKey",
		AuthChoice:  types.AuthChoiceOpenRouterAPIKey,
		CLIFlag:     "--openrouter-api-key",
		CLIOption:   "--openrouter-api-key <key>",
		Description: "OpenRouter API key",
	},
	{
		OptionKey:   "kilocodeApiKey",
		AuthChoice:  types.AuthChoiceKilocodeAPIKey,
		CLIFlag:     "--kilocode-api-key",
		CLIOption:   "--kilocode-api-key <key>",
		Description: "Kilo Gateway API key",
	},
	{
		OptionKey:   "aiGatewayApiKey",
		AuthChoice:  types.AuthChoiceAIGatewayAPIKey,
		CLIFlag:     "--ai-gateway-api-key",
		CLIOption:   "--ai-gateway-api-key <key>",
		Description: "Vercel AI Gateway API key",
	},
	{
		OptionKey:   "cloudflareAiGatewayApiKey",
		AuthChoice:  types.AuthChoiceCloudflareAIGatewayKey,
		CLIFlag:     "--cloudflare-ai-gateway-api-key",
		CLIOption:   "--cloudflare-ai-gateway-api-key <key>",
		Description: "Cloudflare AI Gateway API key",
	},
	{
		OptionKey:   "moonshotApiKey",
		AuthChoice:  types.AuthChoiceMoonshotAPIKey,
		CLIFlag:     "--moonshot-api-key",
		CLIOption:   "--moonshot-api-key <key>",
		Description: "Moonshot API key",
	},
	{
		OptionKey:   "kimiCodeApiKey",
		AuthChoice:  types.AuthChoiceKimiCodeAPIKey,
		CLIFlag:     "--kimi-code-api-key",
		CLIOption:   "--kimi-code-api-key <key>",
		Description: "Kimi Coding API key",
	},
	{
		OptionKey:   "geminiApiKey",
		AuthChoice:  types.AuthChoiceGeminiAPIKey,
		CLIFlag:     "--gemini-api-key",
		CLIOption:   "--gemini-api-key <key>",
		Description: "Gemini API key",
	},
	{
		OptionKey:   "zaiApiKey",
		AuthChoice:  types.AuthChoiceZAIAPIKey,
		CLIFlag:     "--zai-api-key",
		CLIOption:   "--zai-api-key <key>",
		Description: "Z.AI API key",
	},
	{
		OptionKey:   "xiaomiApiKey",
		AuthChoice:  types.AuthChoiceXiaomiAPIKey,
		CLIFlag:     "--xiaomi-api-key",
		CLIOption:   "--xiaomi-api-key <key>",
		Description: "Xiaomi API key",
	},
	{
		OptionKey:   "minimaxApiKey",
		AuthChoice:  types.AuthChoiceMinimaxAPI,
		CLIFlag:     "--minimax-api-key",
		CLIOption:   "--minimax-api-key <key>",
		Description: "MiniMax API key",
	},
	{
		OptionKey:   "syntheticApiKey",
		AuthChoice:  types.AuthChoiceSyntheticAPIKey,
		CLIFlag:     "--synthetic-api-key",
		CLIOption:   "--synthetic-api-key <key>",
		Description: "Synthetic API key",
	},
	{
		OptionKey:   "veniceApiKey",
		AuthChoice:  types.AuthChoiceVeniceAPIKey,
		CLIFlag:     "--venice-api-key",
		CLIOption:   "--venice-api-key <key>",
		Description: "Venice API key",
	},
	{
		OptionKey:   "togetherApiKey",
		AuthChoice:  types.AuthChoiceTogetherAPIKey,
		CLIFlag:     "--together-api-key",
		CLIOption:   "--together-api-key <key>",
		Description: "Together AI API key",
	},
	{
		OptionKey:   "huggingfaceApiKey",
		AuthChoice:  types.AuthChoiceHuggingFaceAPIKey,
		CLIFlag:     "--huggingface-api-key",
		CLIOption:   "--huggingface-api-key <key>",
		Description: "Hugging Face API key (HF token)",
	},
	{
		OptionKey:   "opencodeZenApiKey",
		AuthChoice:  types.AuthChoiceOpenCodeZen,
		CLIFlag:     "--opencode-zen-api-key",
		CLIOption:   "--opencode-zen-api-key <key>",
		Description: "OpenCode Zen API key",
	},
	{
		OptionKey:   "xaiApiKey",
		AuthChoice:  types.AuthChoiceXAIAPIKey,
		CLIFlag:     "--xai-api-key",
		CLIOption:   "--xai-api-key <key>",
		Description: "xAI API key",
	},
	{
		OptionKey:   "litellmApiKey",
		AuthChoice:  types.AuthChoiceLiteLLMAPIKey,
		CLIFlag:     "--litellm-api-key",
		CLIOption:   "--litellm-api-key <key>",
		Description: "LiteLLM API key",
	},
	{
		OptionKey:   "qianfanApiKey",
		AuthChoice:  types.AuthChoiceQianfanAPIKey,
		CLIFlag:     "--qianfan-api-key",
		CLIOption:   "--qianfan-api-key <key>",
		Description: "QIANFAN API key",
	},
	{
		OptionKey:   "volcengineApiKey",
		AuthChoice:  types.AuthChoiceVolcengineAPIKey,
		CLIFlag:     "--volcengine-api-key",
		CLIOption:   "--volcengine-api-key <key>",
		Description: "Volcano Engine API key",
	},
	{
		OptionKey:   "byteplusApiKey",
		AuthChoice:  types.AuthChoiceBytePlusAPIKey,
		CLIFlag:     "--byteplus-api-key",
		CLIOption:   "--byteplus-api-key <key>",
		Description: "BytePlus API key",
	},
}
