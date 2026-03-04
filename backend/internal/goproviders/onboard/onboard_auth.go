// onboard/onboard_auth.go — 入口文件 / 包文档
// 对应 TS 文件: src/commands/onboard-auth.ts
//
// onboard 包提供 OpenClaw 用户引导式认证配置的核心逻辑。
//
// 主要功能：
//   - Provider 配置函数（Apply*Config / Apply*ProviderConfig）
//   - 凭证管理函数（Set*ApiKey / WriteOAuthCredentials）
//   - OAuth 环境检测与 VPS 感知处理器
//   - TLS 证书预检
//
// 可用的 Provider 配置函数列表（按 Provider 名称排序）：
//
//	Cloudflare AI Gateway:
//	  ApplyCloudflareAiGatewayConfig, ApplyCloudflareAiGatewayProviderConfig
//	Hugging Face:
//	  ApplyHuggingfaceConfig, ApplyHuggingfaceProviderConfig
//	Kilocode:
//	  ApplyKilocodeConfig, ApplyKilocodeProviderConfig
//	Kimi Code:
//	  ApplyKimiCodeConfig, ApplyKimiCodeProviderConfig
//	LiteLLM:
//	  ApplyLitellmConfig, ApplyLitellmProviderConfig
//	MiniMax:
//	  ApplyMinimaxConfig, ApplyMinimaxProviderConfig
//	  ApplyMinimaxHostedConfig, ApplyMinimaxHostedProviderConfig
//	  ApplyMinimaxApiConfig, ApplyMinimaxApiProviderConfig
//	  ApplyMinimaxApiConfigCn, ApplyMinimaxApiProviderConfigCn
//	Mistral:
//	  ApplyMistralConfig, ApplyMistralProviderConfig
//	Moonshot:
//	  ApplyMoonshotConfig, ApplyMoonshotProviderConfig
//	  ApplyMoonshotConfigCn, ApplyMoonshotProviderConfigCn
//	OpenCode Zen:
//	  ApplyOpencodeZenConfig, ApplyOpencodeZenProviderConfig
//	OpenRouter:
//	  ApplyOpenrouterConfig, ApplyOpenrouterProviderConfig
//	Qianfan:
//	  ApplyQianfanConfig, ApplyQianfanProviderConfig
//	Synthetic:
//	  ApplySyntheticConfig, ApplySyntheticProviderConfig
//	Together:
//	  ApplyTogetherConfig, ApplyTogetherProviderConfig
//	Venice:
//	  ApplyVeniceConfig, ApplyVeniceProviderConfig
//	Vercel AI Gateway:
//	  ApplyVercelAiGatewayConfig, ApplyVercelAiGatewayProviderConfig
//	xAI:
//	  ApplyXaiConfig, ApplyXaiProviderConfig
//	Xiaomi:
//	  ApplyXiaomiConfig, ApplyXiaomiProviderConfig
//	Z.AI:
//	  ApplyZaiConfig, ApplyZaiProviderConfig
//
// 凭证管理函数列表：
//
//	SetAnthropicApiKey, SetOpenaiApiKey, SetGeminiApiKey, SetMinimaxApiKey,
//	SetMoonshotApiKey, SetKimiCodingApiKey, SetVolcengineApiKey, SetByteplusApiKey,
//	SetSyntheticApiKey, SetVeniceApiKey, SetZaiApiKey, SetXiaomiApiKey,
//	SetOpenrouterApiKey, SetCloudflareAiGatewayConfig, SetLitellmApiKey,
//	SetVercelAiGatewayApiKey, SetOpencodeZenApiKey, SetTogetherApiKey,
//	SetHuggingfaceApiKey, SetQianfanApiKey, SetXaiApiKey, SetMistralApiKey,
//	SetKilocodeApiKey
//
// OAuth 相关：
//
//	IsRemoteEnvironment, CreateVpsAwareOAuthHandlers
//	RunOpenAIOAuthTlsPreflight, FormatOpenAIOAuthTlsPreflightFix
package onboard
