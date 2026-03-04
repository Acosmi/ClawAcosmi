// authchoice/apply_xai.go — xAI 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.xai.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// XaiDefaultModelRef xAI 默认模型引用。
const XaiDefaultModelRef = "xai/grok-3-mini-fast-beta"

// ApplyAuthChoiceXAI xAI 认证 apply。
// 标准 API Key 流程 + 默认模型选择。
// 对应 TS: applyAuthChoiceXAI()
func ApplyAuthChoiceXAI(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceXAIAPIKey {
		return nil, nil
	}

	requestedSecretInputMode := NormalizeSecretInputModeInput(getSecretInputMode(params.Opts))
	noteAgentModel := CreateAuthChoiceAgentModelNoter(params)

	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             getOptToken(params.Opts, "xaiApiKey"),
		TokenProvider:     strPtr("xai"),
		SecretInputMode:   requestedSecretInputMode,
		Config:            params.Config,
		ExpectedProviders: []string{"xai"},
		Provider:          "xai",
		EnvLabel:          "XAI_API_KEY",
		PromptMessage:     "Enter xAI API key",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetXaiApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	if err != nil {
		return nil, err
	}

	nextConfig := ApplyAuthProfileConfig(params.Config, ApplyProfileConfigParams{
		ProfileID: "xai:default",
		Provider:  "xai",
		Mode:      "api_key",
	})

	applied, err := ApplyDefaultModelChoice(DefaultModelChoiceParams{
		Config:              nextConfig,
		SetDefaultModel:     params.SetDefaultModel,
		DefaultModel:        XaiDefaultModelRef,
		ApplyDefaultConfig:  ApplyXaiConfig,
		ApplyProviderConfig: ApplyXaiProviderConfig(nextConfig),
		NoteDefault:         XaiDefaultModelRef,
		NoteAgentModel:      noteAgentModel,
		Prompter:            params.Prompter,
	})
	if err != nil {
		return nil, err
	}

	return &ApplyAuthChoiceResult{
		Config:             applied.Config,
		AgentModelOverride: applied.AgentModelOverride,
	}, nil
}

// ApplyXaiProviderConfig 返回 xAI provider config 应用函数。
// 包装 stub 函数以匹配签名。
func ApplyXaiProviderConfig(base OpenClawConfig) func(OpenClawConfig) OpenClawConfig {
	return func(config OpenClawConfig) OpenClawConfig {
		return ApplyXaiConfig(config)
	}
}
