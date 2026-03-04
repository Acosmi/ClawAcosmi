// authchoice/apply.go — Apply 入口和核心类型定义
// 对应 TS 文件: src/commands/auth-choice.apply.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// ApplyAuthChoiceParams 应用认证选择的参数。
// 对应 TS: ApplyAuthChoiceParams
type ApplyAuthChoiceParams struct {
	AuthChoice      types.AuthChoice
	Config          OpenClawConfig
	Prompter        WizardPrompter
	Runtime         RuntimeEnv
	AgentDir        string
	SetDefaultModel bool
	AgentID         string
	Opts            *types.OnboardOptions
}

// ApplyAuthChoiceResult 认证选择应用结果。
// 对应 TS: ApplyAuthChoiceResult
type ApplyAuthChoiceResult struct {
	Config             OpenClawConfig
	AgentModelOverride string
}

// ApplyAuthChoiceHandler 认证选择处理器函数类型。
// 每个处理器尝试处理指定的 AuthChoice，成功返回结果，不匹配返回 nil。
type ApplyAuthChoiceHandler func(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error)

// ApplyAuthChoice 应用认证选择（主入口）。
// 按优先级依次尝试所有处理器，首个返回非 nil 结果的处理器获胜。
// 对应 TS: applyAuthChoice()
func ApplyAuthChoice(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	handlers := []ApplyAuthChoiceHandler{
		ApplyAuthChoiceAnthropic,
		ApplyAuthChoiceVllm,
		ApplyAuthChoiceOpenAI,
		ApplyAuthChoiceOAuth,
		ApplyAuthChoiceApiProviders,
		ApplyAuthChoiceMiniMax,
		ApplyAuthChoiceGitHubCopilot,
		ApplyAuthChoiceGoogleGeminiCli,
		ApplyAuthChoiceCopilotProxy,
		ApplyAuthChoiceQwenPortal,
		ApplyAuthChoiceXAI,
		ApplyAuthChoiceVolcengine,
		ApplyAuthChoiceBytePlus,
	}

	for _, handler := range handlers {
		result, err := handler(params)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	// 无匹配的处理器，返回原始配置
	return &ApplyAuthChoiceResult{Config: params.Config}, nil
}
