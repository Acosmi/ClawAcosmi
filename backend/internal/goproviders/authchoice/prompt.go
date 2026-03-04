// authchoice/prompt.go — 交互式认证选择分组提示
// 对应 TS 文件: src/commands/auth-choice-prompt.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

const backValue = "__back"

// PromptAuthChoiceGroupedParams 分组认证选择提示参数。
type PromptAuthChoiceGroupedParams struct {
	Prompter    WizardPrompter
	Store       *types.AuthProfileStore
	IncludeSkip bool
}

// PromptAuthChoiceGrouped 交互式分组认证选择提示。
// 先选 Provider 分组，再选该组内具体的认证方式；
// 如果分组只有一个选项则直接选中。
// 对应 TS: promptAuthChoiceGrouped()
func PromptAuthChoiceGrouped(params PromptAuthChoiceGroupedParams) (types.AuthChoice, error) {
	result := BuildAuthChoiceGroups(params.IncludeSkip)
	availableGroups := make([]AuthChoiceGroup, 0, len(result.Groups))
	for _, g := range result.Groups {
		if len(g.Options) > 0 {
			availableGroups = append(availableGroups, g)
		}
	}

	for {
		// 构建 Provider 选择列表
		providerOptions := make([]SelectOption, 0, len(availableGroups)+1)
		for _, g := range availableGroups {
			providerOptions = append(providerOptions, SelectOption{
				Value: string(g.Value),
				Label: g.Label,
				Hint:  g.Hint,
			})
		}
		if result.SkipOption != nil {
			providerOptions = append(providerOptions, SelectOption{
				Value: string(result.SkipOption.Value),
				Label: result.SkipOption.Label,
			})
		}

		providerSelection, err := params.Prompter.Select(SelectPromptOptions{
			Message: "Model/auth provider",
			Options: providerOptions,
		})
		if err != nil {
			return "", err
		}

		if providerSelection == "skip" {
			return types.AuthChoiceSkip, nil
		}

		// 查找选中的分组
		var selectedGroup *AuthChoiceGroup
		for i := range availableGroups {
			if string(availableGroups[i].Value) == providerSelection {
				selectedGroup = &availableGroups[i]
				break
			}
		}

		if selectedGroup == nil || len(selectedGroup.Options) == 0 {
			_ = params.Prompter.Note(
				"No auth methods available for that provider.",
				"Model/auth choice",
			)
			continue
		}

		// 如果只有一个选项，直接选中
		if len(selectedGroup.Options) == 1 {
			return selectedGroup.Options[0].Value, nil
		}

		// 多个选项时让用户选具体方式
		methodOptions := make([]SelectOption, 0, len(selectedGroup.Options)+1)
		for _, opt := range selectedGroup.Options {
			methodOptions = append(methodOptions, SelectOption{
				Value: string(opt.Value),
				Label: opt.Label,
				Hint:  opt.Hint,
			})
		}
		methodOptions = append(methodOptions, SelectOption{
			Value: backValue,
			Label: "Back",
		})

		methodSelection, mErr := params.Prompter.Select(SelectPromptOptions{
			Message: selectedGroup.Label + " auth method",
			Options: methodOptions,
		})
		if mErr != nil {
			return "", mErr
		}

		if methodSelection == backValue {
			continue
		}

		return types.AuthChoice(methodSelection), nil
	}
}
