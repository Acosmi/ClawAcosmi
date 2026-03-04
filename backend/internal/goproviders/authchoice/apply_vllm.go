// authchoice/apply_vllm.go — vLLM 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.vllm.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// applyVllmDefaultModel 应用 vLLM 默认模型配置。
// 保留现有 fallbacks，设置 primary 模型。
// 对应 TS: applyVllmDefaultModel()
func applyVllmDefaultModel(cfg OpenClawConfig, modelRef string) OpenClawConfig {
	// stub：将模型引用写入配置
	return ApplyPrimaryModel(cfg, modelRef)
}

// ApplyAuthChoiceVllm vLLM 认证 apply。
// 交互式配置 vLLM 并设置默认模型。
// 对应 TS: applyAuthChoiceVllm()
func ApplyAuthChoiceVllm(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceVLLM {
		return nil, nil
	}

	// 调用 vLLM 配置（stub）
	nextConfig, modelRef, err := PromptAndConfigureVllm(PromptAndConfigureVllmParams{
		Cfg:      params.Config,
		Prompter: params.Prompter,
		AgentDir: params.AgentDir,
	})
	if err != nil {
		return nil, err
	}

	if !params.SetDefaultModel {
		return &ApplyAuthChoiceResult{
			Config:             nextConfig,
			AgentModelOverride: modelRef,
		}, nil
	}

	_ = params.Prompter.Note("Default model set to "+modelRef, "Model configured")
	return &ApplyAuthChoiceResult{
		Config: applyVllmDefaultModel(nextConfig, modelRef),
	}, nil
}
