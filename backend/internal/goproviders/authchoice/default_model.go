// authchoice/default_model.go — 默认模型 apply 逻辑
// 对应 TS 文件: src/commands/auth-choice.default-model.ts
package authchoice

// DefaultModelChoiceParams 默认模型选择参数。
// 对应 TS: applyDefaultModelChoice 的参数类型
type DefaultModelChoiceParams struct {
	Config              OpenClawConfig
	SetDefaultModel     bool
	DefaultModel        string
	ApplyDefaultConfig  func(OpenClawConfig) OpenClawConfig
	ApplyProviderConfig func(OpenClawConfig) OpenClawConfig
	NoteDefault         string
	NoteAgentModel      func(model string) error
	Prompter            WizardPrompter
}

// DefaultModelChoiceResult 默认模型选择结果。
type DefaultModelChoiceResult struct {
	Config             OpenClawConfig
	AgentModelOverride string
}

// ApplyDefaultModelChoice 应用默认模型选择。
// 对应 TS: applyDefaultModelChoice()
func ApplyDefaultModelChoice(params DefaultModelChoiceParams) (DefaultModelChoiceResult, error) {
	if params.SetDefaultModel {
		next := params.ApplyDefaultConfig(params.Config)
		if params.NoteDefault != "" && params.Prompter != nil {
			_ = params.Prompter.Note("Default model set to "+params.NoteDefault, "Model configured")
		}
		return DefaultModelChoiceResult{Config: next}, nil
	}

	next := params.ApplyProviderConfig(params.Config)
	nextWithModel := EnsureModelAllowlistEntry(next, params.DefaultModel)
	if params.NoteAgentModel != nil {
		_ = params.NoteAgentModel(params.DefaultModel)
	}
	return DefaultModelChoiceResult{
		Config:             nextWithModel,
		AgentModelOverride: params.DefaultModel,
	}, nil
}
