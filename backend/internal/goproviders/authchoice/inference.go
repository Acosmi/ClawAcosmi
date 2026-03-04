// authchoice/inference.go — 非交互式认证选择推理
// 对应 TS 文件: src/commands/onboard-non-interactive/local/auth-choice-inference.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// AuthChoiceFlag 认证选择标志信息。
// 对应 TS: AuthChoiceFlag
type AuthChoiceFlag struct {
	OptionKey  string
	AuthChoice types.AuthChoice
	Label      string
}

// AuthChoiceInference 认证选择推理结果。
// 对应 TS: AuthChoiceInference
type AuthChoiceInference struct {
	Choice  types.AuthChoice
	Matches []AuthChoiceFlag
}

// hasStringValue 检查值是否为非空字符串。
func hasStringValue(value *string) bool {
	if value == nil {
		return false
	}
	return len(*value) > 0
}

// InferAuthChoiceFromFlags 从显式的 Provider API Key 标志推理 AuthChoice。
// 对应 TS: inferAuthChoiceFromFlags()
func InferAuthChoiceFromFlags(opts *types.OnboardOptions) AuthChoiceInference {
	if opts == nil {
		return AuthChoiceInference{}
	}

	// 构建选项字段提取映射
	optionExtractors := map[string]*string{
		"anthropicApiKey":           opts.AnthropicAPIKey,
		"openaiApiKey":              opts.OpenAIAPIKey,
		"mistralApiKey":             opts.MistralAPIKey,
		"openrouterApiKey":          opts.OpenRouterAPIKey,
		"kilocodeApiKey":            opts.KilocodeAPIKey,
		"aiGatewayApiKey":           opts.AIGatewayAPIKey,
		"cloudflareAiGatewayApiKey": opts.CloudflareAIGatewayAPIKey,
		"moonshotApiKey":            opts.MoonshotAPIKey,
		"kimiCodeApiKey":            opts.KimiCodeAPIKey,
		"geminiApiKey":              opts.GeminiAPIKey,
		"zaiApiKey":                 opts.ZAIAPIKey,
		"xiaomiApiKey":              opts.XiaomiAPIKey,
		"minimaxApiKey":             opts.MinimaxAPIKey,
		"syntheticApiKey":           opts.SyntheticAPIKey,
		"veniceApiKey":              opts.VeniceAPIKey,
		"togetherApiKey":            opts.TogetherAPIKey,
		"huggingfaceApiKey":         opts.HuggingFaceAPIKey,
		"opencodeZenApiKey":         opts.OpenCodeZenAPIKey,
		"xaiApiKey":                 opts.XAIAPIKey,
		"litellmApiKey":             opts.LiteLLMAPIKey,
		"qianfanApiKey":             opts.QianfanAPIKey,
		"volcengineApiKey":          opts.VolcengineAPIKey,
		"byteplusApiKey":            opts.BytePlusAPIKey,
	}

	var matches []AuthChoiceFlag

	// 遍历 OnboardProviderAuthFlags 匹配
	for _, flag := range OnboardProviderAuthFlags {
		val, ok := optionExtractors[flag.OptionKey]
		if ok && hasStringValue(val) {
			matches = append(matches, AuthChoiceFlag{
				OptionKey:  flag.OptionKey,
				AuthChoice: flag.AuthChoice,
				Label:      flag.CLIFlag,
			})
		}
	}

	// 检查自定义 Provider 标志
	if hasStringValue(opts.CustomBaseURL) ||
		hasStringValue(opts.CustomModelID) ||
		hasStringValue(opts.CustomAPIKey) {
		matches = append(matches, AuthChoiceFlag{
			OptionKey:  "customBaseUrl",
			AuthChoice: types.AuthChoiceCustomAPIKey,
			Label:      "--custom-base-url/--custom-model-id/--custom-api-key",
		})
	}

	var choice types.AuthChoice
	if len(matches) > 0 {
		choice = matches[0].AuthChoice
	}

	return AuthChoiceInference{
		Choice:  choice,
		Matches: matches,
	}
}
