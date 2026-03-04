// authchoice/apply_byteplus.go — BytePlus 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.byteplus.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// ByteplusDefaultModel BytePlus 默认模型。
const ByteplusDefaultModel = "byteplus-plan/ark-code-latest"

// ApplyAuthChoiceBytePlus BytePlus 认证 apply。
// 标准 API Key 流程 + 默认模型应用（与火山引擎同构）。
// 对应 TS: applyAuthChoiceBytePlus()
func ApplyAuthChoiceBytePlus(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceBytePlusAPIKey {
		return nil, nil
	}

	requestedSecretInputMode := NormalizeSecretInputModeInput(getSecretInputMode(params.Opts))
	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             getOptToken(params.Opts, "byteplusApiKey"),
		TokenProvider:     strPtr("byteplus"),
		SecretInputMode:   requestedSecretInputMode,
		Config:            params.Config,
		ExpectedProviders: []string{"byteplus"},
		Provider:          "byteplus",
		EnvLabel:          "BYTEPLUS_API_KEY",
		PromptMessage:     "Enter BytePlus API key",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetByteplusApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	if err != nil {
		return nil, err
	}

	configWithAuth := ApplyAuthProfileConfig(params.Config, ApplyProfileConfigParams{
		ProfileID: "byteplus:default",
		Provider:  "byteplus",
		Mode:      "api_key",
	})
	configWithModel := ApplyPrimaryModel(configWithAuth, ByteplusDefaultModel)

	return &ApplyAuthChoiceResult{
		Config:             configWithModel,
		AgentModelOverride: ByteplusDefaultModel,
	}, nil
}
