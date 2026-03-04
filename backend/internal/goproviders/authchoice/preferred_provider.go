// authchoice/preferred_provider.go — AuthChoice 到首选 Provider 的映射
// 对应 TS 文件: src/commands/auth-choice.preferred-provider.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// preferredProviderByAuthChoice AuthChoice 到首选 Provider ID 的映射表。
// 对应 TS: PREFERRED_PROVIDER_BY_AUTH_CHOICE
var preferredProviderByAuthChoice = map[types.AuthChoice]string{
	types.AuthChoiceOAuth:                  "anthropic",
	types.AuthChoiceSetupToken:             "anthropic",
	types.AuthChoiceClaudeCLI:              "anthropic",
	types.AuthChoiceToken:                  "anthropic",
	types.AuthChoiceAPIKey:                 "anthropic",
	types.AuthChoiceVLLM:                   "vllm",
	types.AuthChoiceOpenAICodex:            "openai-codex",
	types.AuthChoiceCodexCLI:               "openai-codex",
	types.AuthChoiceChutes:                 "chutes",
	types.AuthChoiceOpenAIAPIKey:           "openai",
	types.AuthChoiceOpenRouterAPIKey:       "openrouter",
	types.AuthChoiceKilocodeAPIKey:         "kilocode",
	types.AuthChoiceAIGatewayAPIKey:        "vercel-ai-gateway",
	types.AuthChoiceCloudflareAIGatewayKey: "cloudflare-ai-gateway",
	types.AuthChoiceMoonshotAPIKey:         "moonshot",
	types.AuthChoiceMoonshotAPIKeyCN:       "moonshot",
	types.AuthChoiceKimiCodeAPIKey:         "kimi-coding",
	types.AuthChoiceGeminiAPIKey:           "google",
	types.AuthChoiceGoogleGeminiCLI:        "google-gemini-cli",
	types.AuthChoiceMistralAPIKey:          "mistral",
	types.AuthChoiceZAIAPIKey:              "zai",
	types.AuthChoiceZAICodingGlobal:        "zai",
	types.AuthChoiceZAICodingCN:            "zai",
	types.AuthChoiceZAIGlobal:              "zai",
	types.AuthChoiceZAICN:                  "zai",
	types.AuthChoiceXiaomiAPIKey:           "xiaomi",
	types.AuthChoiceSyntheticAPIKey:        "synthetic",
	types.AuthChoiceVeniceAPIKey:           "venice",
	types.AuthChoiceTogetherAPIKey:         "together",
	types.AuthChoiceHuggingFaceAPIKey:      "huggingface",
	types.AuthChoiceGitHubCopilot:          "github-copilot",
	types.AuthChoiceCopilotProxy:           "copilot-proxy",
	types.AuthChoiceMinimaxCloud:           "minimax",
	types.AuthChoiceMinimaxAPI:             "minimax",
	types.AuthChoiceMinimaxAPIKeyCN:        "minimax-cn",
	types.AuthChoiceMinimaxAPILightning:    "minimax",
	types.AuthChoiceMinimax:                "lmstudio",
	types.AuthChoiceOpenCodeZen:            "opencode",
	types.AuthChoiceXAIAPIKey:              "xai",
	types.AuthChoiceLiteLLMAPIKey:          "litellm",
	types.AuthChoiceQwenPortal:             "qwen-portal",
	types.AuthChoiceVolcengineAPIKey:       "volcengine",
	types.AuthChoiceBytePlusAPIKey:         "byteplus",
	types.AuthChoiceMinimaxPortal:          "minimax-portal",
	types.AuthChoiceQianfanAPIKey:          "qianfan",
	types.AuthChoiceCustomAPIKey:           "custom",
}

// ResolvePreferredProviderForAuthChoice 根据 AuthChoice 解析首选 Provider ID。
// 对应 TS: resolvePreferredProviderForAuthChoice()
func ResolvePreferredProviderForAuthChoice(choice types.AuthChoice) string {
	if provider, ok := preferredProviderByAuthChoice[choice]; ok {
		return provider
	}
	return ""
}
