// authchoice/apply_openrouter.go — OpenRouter 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.openrouter.ts
// OpenRouter 检查已有 profile，获取 API Key，应用默认模型。
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// OpenrouterDefaultModelRef OpenRouter 默认模型引用。
const OpenrouterDefaultModelRef = "openrouter/anthropic/claude-sonnet-4-20250514"

// ApplyAuthChoiceOpenRouter OpenRouter 认证 apply。
// 检查已有凭据 → 尝试从选项获取 → 交互获取 → 应用默认模型。
// 对应 TS: applyAuthChoiceOpenRouter()
func ApplyAuthChoiceOpenRouter(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceOpenRouterAPIKey {
		return nil, nil
	}

	nextConfig := params.Config
	var agentModelOverride string
	noteAgentModel := CreateAuthChoiceAgentModelNoter(params)
	requestedSecretInputMode := NormalizeSecretInputModeInput(getSecretInputMode(params.Opts))

	// 检查已有 profile
	store := EnsureAuthProfileStore(params.AgentDir)
	profileOrder := ResolveAuthProfileOrder(nextConfig, store, "openrouter")

	var existingProfileId string
	for _, pid := range profileOrder {
		if _, ok := store.Profiles[pid]; ok {
			existingProfileId = pid
			break
		}
	}

	profileId := "openrouter:default"
	mode := "api_key"
	hasCredential := false

	if existingProfileId != "" {
		profileId = existingProfileId
		// 检测凭据类型
		if profile, ok := store.Profiles[existingProfileId]; ok {
			if credType, ok := profile["type"]; ok {
				switch credType {
				case "oauth":
					mode = "oauth"
				case "token":
					mode = "token"
				default:
					mode = "api_key"
				}
			}
		}
		hasCredential = true
	}

	// 如果没有凭据，尝试从选项获取
	if !hasCredential && params.Opts != nil && params.Opts.Token != nil &&
		params.Opts.TokenProvider != nil && *params.Opts.TokenProvider == "openrouter" {
		_ = SetOpenrouterApiKey(
			NormalizeApiKeyInput(*params.Opts.Token),
			params.AgentDir,
			&ApiKeyStorageOptions{SecretInputMode: requestedSecretInputMode},
		)
		hasCredential = true
	}

	// 交互获取 API Key
	if !hasCredential {
		_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
			Token:             getOptToken(params.Opts, "token"),
			TokenProvider:     getOptTokenProvider(params.Opts),
			SecretInputMode:   requestedSecretInputMode,
			Config:            nextConfig,
			ExpectedProviders: []string{"openrouter"},
			Provider:          "openrouter",
			EnvLabel:          "OPENROUTER_API_KEY",
			PromptMessage:     "Enter OpenRouter API key",
			Normalize:         NormalizeApiKeyInput,
			Validate:          ValidateApiKeyInput,
			Prompter:          params.Prompter,
			SetCredential: func(apiKey types.SecretInput, m types.SecretInputMode) error {
				return SetOpenrouterApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: m})
			},
		})
		if err != nil {
			return nil, err
		}
		hasCredential = true
	}

	// 更新配置
	if hasCredential {
		nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
			ProfileID: profileId,
			Provider:  "openrouter",
			Mode:      mode,
		})
	}

	// 应用默认模型
	applied, err := ApplyDefaultModelChoice(DefaultModelChoiceParams{
		Config:              nextConfig,
		SetDefaultModel:     params.SetDefaultModel,
		DefaultModel:        OpenrouterDefaultModelRef,
		ApplyDefaultConfig:  ApplyOpenrouterConfig,
		ApplyProviderConfig: ApplyOpenrouterConfig,
		NoteDefault:         OpenrouterDefaultModelRef,
		NoteAgentModel:      noteAgentModel,
		Prompter:            params.Prompter,
	})
	if err != nil {
		return nil, err
	}

	nextConfig = applied.Config
	if applied.AgentModelOverride != "" {
		agentModelOverride = applied.AgentModelOverride
	}

	return &ApplyAuthChoiceResult{Config: nextConfig, AgentModelOverride: agentModelOverride}, nil
}
