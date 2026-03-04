// authchoice/apply_openai.go — OpenAI 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.openai.ts
// OpenAI 支持两种认证路径：openai-api-key 和 openai-codex。
package authchoice

import (
	"fmt"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// OpenaiDefaultModel OpenAI 默认模型。
const OpenaiDefaultModel = "openai/gpt-4.1"

// ApplyAuthChoiceOpenAI OpenAI 认证 apply。
// 支持 openai-api-key 和 openai-codex 两条路径。
// 对应 TS: applyAuthChoiceOpenAI()
func ApplyAuthChoiceOpenAI(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	requestedSecretInputMode := NormalizeSecretInputModeInput(getSecretInputMode(params.Opts))
	noteAgentModel := CreateAuthChoiceAgentModelNoter(params)

	// 检查是否需要重映射 authChoice
	authChoice := params.AuthChoice
	if authChoice == types.AuthChoiceAPIKey && params.Opts != nil &&
		params.Opts.TokenProvider != nil && *params.Opts.TokenProvider == "openai" {
		authChoice = types.AuthChoiceOpenAIAPIKey
	}

	// 路径 1: openai-api-key
	if authChoice == types.AuthChoiceOpenAIAPIKey {
		return applyOpenAIApiKey(params, requestedSecretInputMode, noteAgentModel)
	}

	// 路径 2: openai-codex
	if params.AuthChoice == types.AuthChoiceOpenAICodex {
		return applyOpenAICodex(params, noteAgentModel)
	}

	return nil, nil
}

// applyOpenAIApiKey 处理 OpenAI API Key 认证路径。
func applyOpenAIApiKey(
	params ApplyAuthChoiceParams,
	requestedSecretInputMode types.SecretInputMode,
	noteAgentModel func(string) error,
) (*ApplyAuthChoiceResult, error) {
	nextConfig := params.Config

	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             getOptToken(params.Opts, "token"),
		TokenProvider:     getOptTokenProvider(params.Opts),
		SecretInputMode:   requestedSecretInputMode,
		Config:            nextConfig,
		ExpectedProviders: []string{"openai"},
		Provider:          "openai",
		EnvLabel:          "OPENAI_API_KEY",
		PromptMessage:     "Enter OpenAI API key",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetOpenaiApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	if err != nil {
		return nil, err
	}

	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
		ProfileID: "openai:default",
		Provider:  "openai",
		Mode:      "api_key",
	})

	// 应用默认模型
	applied, err := ApplyDefaultModelChoice(DefaultModelChoiceParams{
		Config:              nextConfig,
		SetDefaultModel:     params.SetDefaultModel,
		DefaultModel:        OpenaiDefaultModel,
		ApplyDefaultConfig:  ApplyOpenAIConfig,
		ApplyProviderConfig: ApplyOpenAIConfig,
		NoteDefault:         OpenaiDefaultModel,
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

// applyOpenAICodex 处理 OpenAI Codex OAuth 认证路径。
func applyOpenAICodex(
	params ApplyAuthChoiceParams,
	noteAgentModel func(string) error,
) (*ApplyAuthChoiceResult, error) {
	nextConfig := params.Config
	var agentModelOverride string

	// 执行 OAuth 登录（stub）
	creds, err := LoginOpenAICodexOAuth(LoginOpenAICodexOAuthParams{
		Prompter:            params.Prompter,
		Runtime:             params.Runtime,
		IsRemote:            IsRemoteEnvironment(),
		OpenURL:             OpenURL,
		LocalBrowserMessage: "Complete sign-in in browser…",
	})
	if err != nil {
		// 登录失败时保持流程不中断
		return &ApplyAuthChoiceResult{Config: nextConfig, AgentModelOverride: agentModelOverride}, nil
	}

	if creds != nil {
		profileID, wErr := WriteOAuthCredentials("openai-codex", creds, params.AgentDir)
		if wErr != nil {
			return nil, wErr
		}
		nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
			ProfileID: profileID,
			Provider:  "openai-codex",
			Mode:      "oauth",
		})
		if params.SetDefaultModel {
			applied := ApplyGoogleGeminiModelDefault(nextConfig)
			// 复用 stub 逻辑：设置 Codex 默认模型
			nextConfig = ApplyPrimaryModel(applied.Next, OpenaiCodexDefaultModel)
			_ = params.Prompter.Note(
				fmt.Sprintf("Default model set to %s", OpenaiCodexDefaultModel),
				"Model configured",
			)
		} else {
			agentModelOverride = OpenaiCodexDefaultModel
			_ = noteAgentModel(OpenaiCodexDefaultModel)
		}
	}

	return &ApplyAuthChoiceResult{Config: nextConfig, AgentModelOverride: agentModelOverride}, nil
}
