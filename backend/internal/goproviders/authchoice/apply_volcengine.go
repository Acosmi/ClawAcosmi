// authchoice/apply_volcengine.go — 火山引擎认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.volcengine.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// VolcengineDefaultModel 火山引擎默认模型。
const VolcengineDefaultModel = "volcengine-plan/ark-code-latest"

// ApplyAuthChoiceVolcengine 火山引擎认证 apply。
// 标准 API Key 流程 + 默认模型应用。
// 对应 TS: applyAuthChoiceVolcengine()
func ApplyAuthChoiceVolcengine(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceVolcengineAPIKey {
		return nil, nil
	}

	requestedSecretInputMode := NormalizeSecretInputModeInput(getSecretInputMode(params.Opts))
	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             getOptToken(params.Opts, "volcengineApiKey"),
		TokenProvider:     strPtr("volcengine"),
		SecretInputMode:   requestedSecretInputMode,
		Config:            params.Config,
		ExpectedProviders: []string{"volcengine"},
		Provider:          "volcengine",
		EnvLabel:          "VOLCANO_ENGINE_API_KEY",
		PromptMessage:     "Enter Volcano Engine API key",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetVolcengineApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	if err != nil {
		return nil, err
	}

	configWithAuth := ApplyAuthProfileConfig(params.Config, ApplyProfileConfigParams{
		ProfileID: "volcengine:default",
		Provider:  "volcengine",
		Mode:      "api_key",
	})
	configWithModel := ApplyPrimaryModel(configWithAuth, VolcengineDefaultModel)

	return &ApplyAuthChoiceResult{
		Config:             configWithModel,
		AgentModelOverride: VolcengineDefaultModel,
	}, nil
}
