// authchoice/apply_minimax.go — MiniMax 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.minimax.ts
// MiniMax 支持多种认证方式：Portal OAuth、Cloud/API/Lightning API Key、CN API Key、本地模式。
package authchoice

import (
	"fmt"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ApplyAuthChoiceMiniMax MiniMax 认证 apply。
// 支持 minimax-portal、minimax-cloud/api/lightning、minimax-api-key-cn、minimax 四条路径。
// 对应 TS: applyAuthChoiceMiniMax()
func ApplyAuthChoiceMiniMax(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	nextConfig := params.Config
	var agentModelOverride string

	// 创建模型状态桥接器
	state := &ApplyAuthChoiceModelState{
		Config:             nextConfig,
		AgentModelOverride: agentModelOverride,
	}
	applyProviderDefaultModel := CreateAuthChoiceDefaultModelApplier(params, state)
	requestedSecretInputMode := NormalizeSecretInputModeInput(getSecretInputMode(params.Opts))

	// 路径 1: minimax-portal（OAuth）
	if params.AuthChoice == types.AuthChoiceMinimaxPortal {
		return applyMinimaxPortal(params)
	}

	// 路径 2: minimax-cloud / minimax-api / minimax-api-lightning
	if params.AuthChoice == types.AuthChoiceMinimaxCloud ||
		params.AuthChoice == types.AuthChoiceMinimaxAPI ||
		params.AuthChoice == types.AuthChoiceMinimaxAPILightning {
		return applyMinimaxApiVariant(params, state, applyProviderDefaultModel, requestedSecretInputMode)
	}

	// 路径 3: minimax-api-key-cn
	if params.AuthChoice == types.AuthChoiceMinimaxAPIKeyCN {
		return applyMinimaxCN(params, state, applyProviderDefaultModel, requestedSecretInputMode)
	}

	// 路径 4: minimax 本地
	if params.AuthChoice == types.AuthChoiceMinimax {
		err := applyProviderDefaultModel(DefaultModelApplyOptions{
			DefaultModel:        "lmstudio/minimax-m2.5-gs32",
			ApplyDefaultConfig:  ApplyMinimaxConfig,
			ApplyProviderConfig: ApplyMinimaxConfig,
		})
		if err != nil {
			return nil, err
		}
		return &ApplyAuthChoiceResult{
			Config:             state.Config,
			AgentModelOverride: state.AgentModelOverride,
		}, nil
	}

	return nil, nil
}

// applyMinimaxPortal 处理 MiniMax Portal OAuth 认证路径。
func applyMinimaxPortal(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	// 让用户选择端点
	endpoint, err := params.Prompter.Select(SelectPromptOptions{
		Message: "Select MiniMax endpoint",
		Options: []SelectOption{
			{Value: "oauth", Label: "Global", Hint: "OAuth for international users"},
			{Value: "oauth-cn", Label: "CN", Hint: "OAuth for users in China"},
		},
	})
	if err != nil {
		return nil, err
	}

	// 委托 plugin provider
	return ApplyAuthChoicePluginProvider(params, PluginProviderAuthChoiceOptions{
		AuthChoice: "minimax-portal",
		PluginID:   "minimax-portal-auth",
		ProviderID: "minimax-portal",
		MethodID:   endpoint,
		Label:      "MiniMax",
	})
}

// ensureMinimaxApiKey 确保 MiniMax API Key 存在。
func ensureMinimaxApiKey(
	params ApplyAuthChoiceParams,
	requestedSecretInputMode types.SecretInputMode,
	profileId string,
	promptMessage string,
) error {
	_, err := EnsureApiKeyFromOptionEnvOrPrompt(EnsureApiKeyFromOptionEnvOrPromptParams{
		Token:             getOptToken(params.Opts, "token"),
		TokenProvider:     getOptTokenProvider(params.Opts),
		SecretInputMode:   requestedSecretInputMode,
		Config:            params.Config,
		ExpectedProviders: []string{"minimax", "minimax-cn"},
		Provider:          "minimax",
		EnvLabel:          "MINIMAX_API_KEY",
		PromptMessage:     promptMessage,
		Normalize:         NormalizeApiKeyInput,
		Validate:          ValidateApiKeyInput,
		Prompter:          params.Prompter,
		SetCredential: func(apiKey types.SecretInput, mode types.SecretInputMode) error {
			return SetMinimaxApiKey(apiKey, params.AgentDir, profileId, &ApiKeyStorageOptions{SecretInputMode: mode})
		},
	})
	return err
}

// applyMinimaxApiVariant 处理 MiniMax Cloud/API/Lightning 变体。
func applyMinimaxApiVariant(
	params ApplyAuthChoiceParams,
	state *ApplyAuthChoiceModelState,
	applyProviderDefaultModel CreateDefaultModelApplierFunc,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	profileId := "minimax:default"
	provider := "minimax"
	promptMessage := "Enter MiniMax API key"

	// 根据选择确定模型
	modelId := "MiniMax-M2.5"
	if params.AuthChoice == types.AuthChoiceMinimaxAPILightning {
		modelId = "MiniMax-M2.5-highspeed"
	}

	// 确保 API Key
	if err := ensureMinimaxApiKey(params, requestedSecretInputMode, profileId, promptMessage); err != nil {
		return nil, err
	}

	state.Config = ApplyAuthProfileConfig(state.Config, ApplyProfileConfigParams{
		ProfileID: profileId,
		Provider:  provider,
		Mode:      "api_key",
	})

	modelRef := fmt.Sprintf("minimax/%s", modelId)
	err := applyProviderDefaultModel(DefaultModelApplyOptions{
		DefaultModel: modelRef,
		ApplyDefaultConfig: func(config OpenClawConfig) OpenClawConfig {
			return ApplyMinimaxApiConfig(config, modelId)
		},
		ApplyProviderConfig: func(config OpenClawConfig) OpenClawConfig {
			return ApplyMinimaxApiConfig(config, modelId)
		},
	})
	if err != nil {
		return nil, err
	}

	return &ApplyAuthChoiceResult{
		Config:             state.Config,
		AgentModelOverride: state.AgentModelOverride,
	}, nil
}

// applyMinimaxCN 处理 MiniMax CN API Key。
func applyMinimaxCN(
	params ApplyAuthChoiceParams,
	state *ApplyAuthChoiceModelState,
	applyProviderDefaultModel CreateDefaultModelApplierFunc,
	requestedSecretInputMode types.SecretInputMode,
) (*ApplyAuthChoiceResult, error) {
	profileId := "minimax-cn:default"
	provider := "minimax-cn"
	promptMessage := "Enter MiniMax China API key"
	modelId := "MiniMax-M2.5"

	// 确保 API Key
	if err := ensureMinimaxApiKey(params, requestedSecretInputMode, profileId, promptMessage); err != nil {
		return nil, err
	}

	state.Config = ApplyAuthProfileConfig(state.Config, ApplyProfileConfigParams{
		ProfileID: profileId,
		Provider:  provider,
		Mode:      "api_key",
	})

	modelRef := fmt.Sprintf("minimax-cn/%s", modelId)
	err := applyProviderDefaultModel(DefaultModelApplyOptions{
		DefaultModel: modelRef,
		ApplyDefaultConfig: func(config OpenClawConfig) OpenClawConfig {
			return ApplyMinimaxApiConfigCn(config, modelId)
		},
		ApplyProviderConfig: func(config OpenClawConfig) OpenClawConfig {
			return ApplyMinimaxApiConfigCn(config, modelId)
		},
	})
	if err != nil {
		return nil, err
	}

	return &ApplyAuthChoiceResult{
		Config:             state.Config,
		AgentModelOverride: state.AgentModelOverride,
	}, nil
}
