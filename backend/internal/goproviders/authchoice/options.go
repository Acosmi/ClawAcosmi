// authchoice/options.go — 认证选项定义和构建函数（第一部分：类型 + 数据表）
// 对应 TS 文件: src/commands/auth-choice-options.ts
package authchoice

import (
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// AuthChoiceOption 单个认证选项。
// 对应 TS: AuthChoiceOption
type AuthChoiceOption struct {
	Value types.AuthChoice `json:"value"`
	Label string           `json:"label"`
	Hint  string           `json:"hint,omitempty"`
}

// AuthChoiceGroup 认证选项分组。
// 对应 TS: AuthChoiceGroup
type AuthChoiceGroup struct {
	Value   types.AuthChoiceGroupID `json:"value"`
	Label   string                  `json:"label"`
	Hint    string                  `json:"hint,omitempty"`
	Options []AuthChoiceOption      `json:"options"`
}

// authChoiceGroupDef 分组定义（内部使用）。
type authChoiceGroupDef struct {
	Value   types.AuthChoiceGroupID
	Label   string
	Hint    string
	Choices []types.AuthChoice
}

// authChoiceGroupDefs 所有认证选择分组定义。
// 对应 TS: AUTH_CHOICE_GROUP_DEFS
var authChoiceGroupDefs = []authChoiceGroupDef{
	{Value: types.AuthGroupOpenAI, Label: "OpenAI", Hint: "Codex OAuth + API key", Choices: []types.AuthChoice{types.AuthChoiceOpenAICodex, types.AuthChoiceOpenAIAPIKey}},
	{Value: types.AuthGroupAnthropic, Label: "Anthropic", Hint: "setup-token + API key", Choices: []types.AuthChoice{types.AuthChoiceToken, types.AuthChoiceAPIKey}},
	{Value: types.AuthGroupChutes, Label: "Chutes", Hint: "OAuth", Choices: []types.AuthChoice{types.AuthChoiceChutes}},
	{Value: types.AuthGroupVLLM, Label: "vLLM", Hint: "Local/self-hosted OpenAI-compatible", Choices: []types.AuthChoice{types.AuthChoiceVLLM}},
	{Value: types.AuthGroupMinimax, Label: "MiniMax", Hint: "M2.5 (recommended)", Choices: []types.AuthChoice{types.AuthChoiceMinimaxPortal, types.AuthChoiceMinimaxAPI, types.AuthChoiceMinimaxAPIKeyCN, types.AuthChoiceMinimaxAPILightning}},
	{Value: types.AuthGroupMoonshot, Label: "Moonshot AI (Kimi K2.5)", Hint: "Kimi K2.5 + Kimi Coding", Choices: []types.AuthChoice{types.AuthChoiceMoonshotAPIKey, types.AuthChoiceMoonshotAPIKeyCN, types.AuthChoiceKimiCodeAPIKey}},
	{Value: types.AuthGroupGoogle, Label: "Google", Hint: "Gemini API key + OAuth", Choices: []types.AuthChoice{types.AuthChoiceGeminiAPIKey, types.AuthChoiceGoogleGeminiCLI}},
	{Value: types.AuthGroupXAI, Label: "xAI (Grok)", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceXAIAPIKey}},
	{Value: types.AuthGroupMistral, Label: "Mistral AI", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceMistralAPIKey}},
	{Value: types.AuthGroupVolcengine, Label: "Volcano Engine", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceVolcengineAPIKey}},
	{Value: types.AuthGroupBytePlus, Label: "BytePlus", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceBytePlusAPIKey}},
	{Value: types.AuthGroupOpenRouter, Label: "OpenRouter", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceOpenRouterAPIKey}},
	{Value: types.AuthGroupKilocode, Label: "Kilo Gateway", Hint: "API key (OpenRouter-compatible)", Choices: []types.AuthChoice{types.AuthChoiceKilocodeAPIKey}},
	{Value: types.AuthGroupQwen, Label: "Qwen", Hint: "OAuth", Choices: []types.AuthChoice{types.AuthChoiceQwenPortal}},
	{Value: types.AuthGroupZAI, Label: "Z.AI", Hint: "GLM Coding Plan / Global / CN", Choices: []types.AuthChoice{types.AuthChoiceZAICodingGlobal, types.AuthChoiceZAICodingCN, types.AuthChoiceZAIGlobal, types.AuthChoiceZAICN}},
	{Value: types.AuthGroupQianfan, Label: "Qianfan", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceQianfanAPIKey}},
	{Value: types.AuthGroupCopilot, Label: "Copilot", Hint: "GitHub + local proxy", Choices: []types.AuthChoice{types.AuthChoiceGitHubCopilot, types.AuthChoiceCopilotProxy}},
	{Value: types.AuthGroupAIGateway, Label: "Vercel AI Gateway", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceAIGatewayAPIKey}},
	{Value: types.AuthGroupOpenCodeZen, Label: "OpenCode Zen", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceOpenCodeZen}},
	{Value: types.AuthGroupXiaomi, Label: "Xiaomi", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceXiaomiAPIKey}},
	{Value: types.AuthGroupSynthetic, Label: "Synthetic", Hint: "Anthropic-compatible (multi-model)", Choices: []types.AuthChoice{types.AuthChoiceSyntheticAPIKey}},
	{Value: types.AuthGroupTogether, Label: "Together AI", Hint: "API key", Choices: []types.AuthChoice{types.AuthChoiceTogetherAPIKey}},
	{Value: types.AuthGroupHuggingFace, Label: "Hugging Face", Hint: "Inference API (HF token)", Choices: []types.AuthChoice{types.AuthChoiceHuggingFaceAPIKey}},
	{Value: types.AuthGroupVenice, Label: "Venice AI", Hint: "Privacy-focused (uncensored models)", Choices: []types.AuthChoice{types.AuthChoiceVeniceAPIKey}},
	{Value: types.AuthGroupLiteLLM, Label: "LiteLLM", Hint: "Unified LLM gateway (100+ providers)", Choices: []types.AuthChoice{types.AuthChoiceLiteLLMAPIKey}},
	{Value: types.AuthGroupCloudflareAIGateway, Label: "Cloudflare AI Gateway", Hint: "Account ID + Gateway ID + API key", Choices: []types.AuthChoice{types.AuthChoiceCloudflareAIGatewayKey}},
	{Value: types.AuthGroupCustom, Label: "Custom Provider", Hint: "Any OpenAI or Anthropic compatible endpoint", Choices: []types.AuthChoice{types.AuthChoiceCustomAPIKey}},
}

// providerAuthChoiceOptionHints 特定选项的自定义 hint。
// 对应 TS: PROVIDER_AUTH_CHOICE_OPTION_HINTS
var providerAuthChoiceOptionHints = map[types.AuthChoice]string{
	types.AuthChoiceLiteLLMAPIKey:          "Unified gateway for 100+ LLM providers",
	types.AuthChoiceCloudflareAIGatewayKey: "Account ID + Gateway ID + API key",
	types.AuthChoiceVeniceAPIKey:           "Privacy-focused inference (uncensored models)",
	types.AuthChoiceTogetherAPIKey:         "Access to Llama, DeepSeek, Qwen, and more open models",
	types.AuthChoiceHuggingFaceAPIKey:      "Inference Providers — OpenAI-compatible chat",
}

// providerAuthChoiceOptionLabels 特定选项的自定义 label。
// 对应 TS: PROVIDER_AUTH_CHOICE_OPTION_LABELS
var providerAuthChoiceOptionLabels = map[types.AuthChoice]string{
	types.AuthChoiceMoonshotAPIKey:         "Kimi API key (.ai)",
	types.AuthChoiceMoonshotAPIKeyCN:       "Kimi API key (.cn)",
	types.AuthChoiceKimiCodeAPIKey:         "Kimi Code API key (subscription)",
	types.AuthChoiceCloudflareAIGatewayKey: "Cloudflare AI Gateway",
}

// buildProviderAuthChoiceOptions 从 Provider 标志列表构建认证选项。
// 对应 TS: buildProviderAuthChoiceOptions()
func buildProviderAuthChoiceOptions() []AuthChoiceOption {
	options := make([]AuthChoiceOption, 0, len(OnboardProviderAuthFlags))
	for _, flag := range OnboardProviderAuthFlags {
		opt := AuthChoiceOption{
			Value: flag.AuthChoice,
			Label: flag.Description,
		}
		if label, ok := providerAuthChoiceOptionLabels[flag.AuthChoice]; ok {
			opt.Label = label
		}
		if hint, ok := providerAuthChoiceOptionHints[flag.AuthChoice]; ok {
			opt.Hint = hint
		}
		options = append(options, opt)
	}
	return options
}
