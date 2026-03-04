// authchoice/options_funcs.go — 认证选项构建函数
// 对应 TS 文件: src/commands/auth-choice-options.ts（BASE_AUTH_CHOICE_OPTIONS + 3个导出函数）
package authchoice

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// baseAuthChoiceOptions 基础认证选项列表（静态部分 + Provider 标志动态部分）。
// 对应 TS: BASE_AUTH_CHOICE_OPTIONS
func baseAuthChoiceOptions() []AuthChoiceOption {
	options := []AuthChoiceOption{
		{Value: types.AuthChoiceToken, Label: "Anthropic token (paste setup-token)", Hint: "run `claude setup-token` elsewhere, then paste the token here"},
		{Value: types.AuthChoiceOpenAICodex, Label: "OpenAI Codex (ChatGPT OAuth)"},
		{Value: types.AuthChoiceChutes, Label: "Chutes (OAuth)"},
		{Value: types.AuthChoiceVLLM, Label: "vLLM (custom URL + model)", Hint: "Local/self-hosted OpenAI-compatible server"},
	}
	// 动态插入 Provider 标志选项
	options = append(options, buildProviderAuthChoiceOptions()...)
	// 附加静态选项
	options = append(options,
		AuthChoiceOption{Value: types.AuthChoiceMoonshotAPIKeyCN, Label: "Kimi API key (.cn)"},
		AuthChoiceOption{Value: types.AuthChoiceGitHubCopilot, Label: "GitHub Copilot (GitHub device login)", Hint: "Uses GitHub device flow"},
		AuthChoiceOption{Value: types.AuthChoiceGeminiAPIKey, Label: "Google Gemini API key"},
		AuthChoiceOption{Value: types.AuthChoiceGoogleGeminiCLI, Label: "Google Gemini CLI OAuth", Hint: "Unofficial flow; review account-risk warning before use"},
		AuthChoiceOption{Value: types.AuthChoiceZAIAPIKey, Label: "Z.AI API key"},
		AuthChoiceOption{Value: types.AuthChoiceZAICodingGlobal, Label: "Coding-Plan-Global", Hint: "GLM Coding Plan Global (api.z.ai)"},
		AuthChoiceOption{Value: types.AuthChoiceZAICodingCN, Label: "Coding-Plan-CN", Hint: "GLM Coding Plan CN (open.bigmodel.cn)"},
		AuthChoiceOption{Value: types.AuthChoiceZAIGlobal, Label: "Global", Hint: "Z.AI Global (api.z.ai)"},
		AuthChoiceOption{Value: types.AuthChoiceZAICN, Label: "CN", Hint: "Z.AI CN (open.bigmodel.cn)"},
		AuthChoiceOption{Value: types.AuthChoiceXiaomiAPIKey, Label: "Xiaomi API key"},
		AuthChoiceOption{Value: types.AuthChoiceMinimaxPortal, Label: "MiniMax OAuth", Hint: "Oauth plugin for MiniMax"},
		AuthChoiceOption{Value: types.AuthChoiceQwenPortal, Label: "Qwen OAuth"},
		AuthChoiceOption{Value: types.AuthChoiceCopilotProxy, Label: "Copilot Proxy (local)", Hint: "Local proxy for VS Code Copilot models"},
		AuthChoiceOption{Value: types.AuthChoiceAPIKey, Label: "Anthropic API key"},
		AuthChoiceOption{Value: types.AuthChoiceOpenCodeZen, Label: "OpenCode Zen (multi-model proxy)", Hint: "Claude, GPT, Gemini via opencode.ai/zen"},
		AuthChoiceOption{Value: types.AuthChoiceMinimaxAPI, Label: "MiniMax M2.5"},
		AuthChoiceOption{Value: types.AuthChoiceMinimaxAPIKeyCN, Label: "MiniMax M2.5 (CN)", Hint: "China endpoint (api.minimaxi.com)"},
		AuthChoiceOption{Value: types.AuthChoiceMinimaxAPILightning, Label: "MiniMax M2.5 Highspeed", Hint: "Official fast tier (legacy: Lightning)"},
		AuthChoiceOption{Value: types.AuthChoiceCustomAPIKey, Label: "Custom Provider"},
	)
	return options
}

// FormatAuthChoiceChoicesForCLI 格式化认证选择列表为 CLI 可用的 "|" 分隔字符串。
// 对应 TS: formatAuthChoiceChoicesForCli()
func FormatAuthChoiceChoicesForCLI(includeSkip bool, includeLegacyAliases bool) string {
	options := baseAuthChoiceOptions()
	values := make([]string, 0, len(options)+2)
	for _, opt := range options {
		values = append(values, string(opt.Value))
	}
	if includeSkip {
		values = append(values, string(types.AuthChoiceSkip))
	}
	if includeLegacyAliases {
		for _, alias := range AuthChoiceLegacyAliasesForCLI {
			values = append(values, string(alias))
		}
	}
	return strings.Join(values, "|")
}

// BuildAuthChoiceOptions 构建认证选项列表。
// 对应 TS: buildAuthChoiceOptions()
func BuildAuthChoiceOptions(includeSkip bool) []AuthChoiceOption {
	options := make([]AuthChoiceOption, len(baseAuthChoiceOptions()))
	copy(options, baseAuthChoiceOptions())
	if includeSkip {
		options = append(options, AuthChoiceOption{Value: types.AuthChoiceSkip, Label: "Skip for now"})
	}
	return options
}

// BuildAuthChoiceGroupsResult 分组构建结果。
type BuildAuthChoiceGroupsResult struct {
	Groups     []AuthChoiceGroup
	SkipOption *AuthChoiceOption
}

// BuildAuthChoiceGroups 构建认证选项分组。
// 对应 TS: buildAuthChoiceGroups()
func BuildAuthChoiceGroups(includeSkip bool) BuildAuthChoiceGroupsResult {
	options := BuildAuthChoiceOptions(false)
	optionByValue := make(map[types.AuthChoice]AuthChoiceOption, len(options))
	for _, opt := range options {
		optionByValue[opt.Value] = opt
	}

	groups := make([]AuthChoiceGroup, 0, len(authChoiceGroupDefs))
	for _, def := range authChoiceGroupDefs {
		group := AuthChoiceGroup{
			Value:   def.Value,
			Label:   def.Label,
			Hint:    def.Hint,
			Options: make([]AuthChoiceOption, 0, len(def.Choices)),
		}
		for _, choice := range def.Choices {
			if opt, ok := optionByValue[choice]; ok {
				group.Options = append(group.Options, opt)
			}
		}
		groups = append(groups, group)
	}

	var skipOption *AuthChoiceOption
	if includeSkip {
		skip := AuthChoiceOption{Value: types.AuthChoiceSkip, Label: "Skip for now"}
		skipOption = &skip
	}

	return BuildAuthChoiceGroupsResult{
		Groups:     groups,
		SkipOption: skipOption,
	}
}
