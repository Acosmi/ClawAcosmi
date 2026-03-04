// authchoice/apply_anthropic.go — Anthropic 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.anthropic.ts
// Anthropic 支持两种认证路径：setup-token 和 apiKey。
package authchoice

import (
	"fmt"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// AnthropicDefaultModel Anthropic 默认模型。
const AnthropicDefaultModel = "anthropic/claude-sonnet-4-6"

// ApplyAuthChoiceAnthropic Anthropic 认证 apply。
// 支持 setup-token/oauth/token 路径和 apiKey 路径。
// 对应 TS: applyAuthChoiceAnthropic()
func ApplyAuthChoiceAnthropic(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	requestedSecretInputMode := NormalizeSecretInputModeInput(getSecretInputMode(params.Opts))

	// 路径 1: setup-token / oauth / token
	if params.AuthChoice == types.AuthChoiceSetupToken ||
		params.AuthChoice == types.AuthChoiceOAuth ||
		params.AuthChoice == types.AuthChoiceToken {
		return applyAnthropicSetupToken(params, requestedSecretInputMode)
	}

	// 路径 2: apiKey
	if params.AuthChoice == types.AuthChoiceAPIKey {
		return applyAnthropicApiKey(params, requestedSecretInputMode)
	}

	return nil, nil
}

// applyAnthropicSetupToken 处理 Anthropic setup-token 认证路径。
func applyAnthropicSetupToken(
	params ApplyAuthChoiceParams,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	nextConfig := params.Config

	// 显示使用说明
	noteMsg := strings.Join([]string{
		"Run `claude setup-token` in your terminal.",
		"Then paste the generated token below.",
	}, "\n")
	_ = params.Prompter.Note(noteMsg, "Anthropic setup-token")

	// 选择密钥输入模式
	selectedMode, err := ResolveSecretInputModeForEnvSelection(
		params.Prompter,
		requestedSecretInputMode,
		&SecretInputModePromptCopy{
			ModeMessage:    "How do you want to provide this setup token?",
			PlaintextLabel: "Paste setup token now",
			PlaintextHint:  "Stores the token directly in the auth profile",
		},
	)
	if err != nil {
		return nil, err
	}

	var token string
	var tokenRef *TokenRef

	if selectedMode == types.SecretInputModeRef {
		// 引用模式
		ref, resolvedValue, pErr := PromptSecretRefForOnboarding(PromptSecretRefParams{
			Provider:        "anthropic-setup-token",
			Config:          params.Config,
			Prompter:        params.Prompter,
			PreferredEnvVar: "ANTHROPIC_SETUP_TOKEN",
			Copy: &SecretRefOnboardingPromptCopy{
				SourceMessage:     "Where is this Anthropic setup token stored?",
				EnvVarPlaceholder: "ANTHROPIC_SETUP_TOKEN",
			},
		})
		if pErr != nil {
			return nil, pErr
		}
		token = strings.TrimSpace(resolvedValue)
		tokenRef = &TokenRef{
			Source:   string(ref.Source),
			Provider: ref.Provider,
			ID:       ref.ID,
		}
	} else {
		// 直接粘贴
		tokenRaw, tErr := params.Prompter.Text(TextPromptOptions{
			Message:  "Paste Anthropic setup-token",
			Validate: ValidateAnthropicSetupToken,
		})
		if tErr != nil {
			return nil, tErr
		}
		token = strings.TrimSpace(tokenRaw)
	}

	// 验证 Token
	validationError := ValidateAnthropicSetupToken(token)
	if validationError != "" {
		return nil, fmt.Errorf("%s", validationError)
	}

	// 获取 Profile 名称
	profileNameRaw, err := params.Prompter.Text(TextPromptOptions{
		Message:     "Token name (blank = default)",
		Placeholder: "default",
	})
	if err != nil {
		return nil, err
	}

	provider := "anthropic"
	namedProfileId := BuildTokenProfileId(provider, profileNameRaw)

	// 构造凭据
	credential := map[string]interface{}{
		"type":     "token",
		"provider": provider,
		"token":    token,
	}
	if tokenRef != nil {
		credential["tokenRef"] = tokenRef
	}
	UpsertAuthProfile(namedProfileId, credential)

	// 更新配置
	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
		ProfileID: namedProfileId,
		Provider:  provider,
		Mode:      "token",
	})
	if params.SetDefaultModel {
		nextConfig = ApplyPrimaryModel(nextConfig, AnthropicDefaultModel)
	}
	return &ApplyAuthChoiceResult{Config: nextConfig}, nil
}

// applyAnthropicApiKey 处理 Anthropic API Key 认证路径。
func applyAnthropicApiKey(
	params ApplyAuthChoiceParams,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	// 检查 tokenProvider 是否匹配
	if params.Opts != nil && params.Opts.TokenProvider != nil &&
		*params.Opts.TokenProvider != "" && *params.Opts.TokenProvider != "anthropic" {
		return nil, nil
	}

	nextConfig := params.Config
	tokenProvider := strPtr("anthropic")

	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             getOptToken(params.Opts, "token"),
		TokenProvider:     tokenProvider,
		SecretInputMode:   requestedSecretInputMode,
		Config:            nextConfig,
		ExpectedProviders: []string{"anthropic"},
		Provider:          "anthropic",
		EnvLabel:          "ANTHROPIC_API_KEY",
		PromptMessage:     "Enter Anthropic API key",
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetAnthropicApiKey(apiKey, params.AgentDir, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	if err != nil {
		return nil, err
	}

	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
		ProfileID: "anthropic:default",
		Provider:  "anthropic",
		Mode:      "api_key",
	})
	if params.SetDefaultModel {
		nextConfig = ApplyPrimaryModel(nextConfig, AnthropicDefaultModel)
	}
	return &ApplyAuthChoiceResult{Config: nextConfig}, nil
}

// TokenRef Token 引用信息。
type TokenRef struct {
	Source   string `json:"source"`
	Provider string `json:"provider"`
	ID       string `json:"id"`
}
