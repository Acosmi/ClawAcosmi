// onboard/config_minimax.go — MiniMax 专用配置函数
// 对应 TS 文件: src/commands/onboard-auth.config-minimax.ts
package onboard

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ApplyMinimaxProviderConfig 应用 MiniMax Provider 配置（LM Studio 本地模式）。
func ApplyMinimaxProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, "anthropic/claude-opus-4-6", "Opus")
	setModelAlias(models, "lmstudio/minimax-m2.5-gs32", "Minimax")

	providers := getProviders(cfg)
	if _, ok := providers["lmstudio"]; !ok {
		boolFalse := false
		providers["lmstudio"] = map[string]interface{}{
			"baseUrl": "http://127.0.0.1:1234/v1",
			"apiKey":  "lmstudio",
			"api":     "openai-responses",
			"models": []types.ModelDefinitionConfig{
				BuildMinimaxModelDefinition(BuildMinimaxModelDefinitionParams{
					ID:            "minimax-m2.5-gs32",
					Name:          "MiniMax M2.5 GS32",
					Reasoning:     &boolFalse,
					Cost:          MinimaxLmStudioCost,
					ContextWindow: 196608,
					MaxTokens:     8192,
				}),
			},
		}
	}

	return ApplyOnboardAuthAgentModelsAndProviders(cfg, models, providers)
}

// ApplyMinimaxHostedProviderConfig 应用 MiniMax 托管版 Provider 配置。
func ApplyMinimaxHostedProviderConfig(cfg OpenClawConfig, baseURL string) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, MinimaxHostedModelRef, "Minimax")

	providers := getProviders(cfg)
	var existingProvider map[string]interface{}
	if raw, ok := providers["minimax"]; ok && raw != nil {
		if ep, ok := raw.(map[string]interface{}); ok {
			existingProvider = ep
		}
	}
	existingModels := getModelsList(existingProvider)

	hostedModel := BuildMinimaxModelDefinition(BuildMinimaxModelDefinitionParams{
		ID:            MinimaxHostedModelID,
		Cost:          MinimaxHostedCost,
		ContextWindow: DefaultMinimaxContextWindow,
		MaxTokens:     DefaultMinimaxMaxTokens,
	})

	hasHostedModel := false
	for _, m := range existingModels {
		if m.ID == MinimaxHostedModelID {
			hasHostedModel = true
			break
		}
	}
	mergedModels := existingModels
	if !hasHostedModel {
		mergedModels = append(append([]types.ModelDefinitionConfig{}, existingModels...), hostedModel)
	}

	newProvider := copyMap(existingProvider)
	resolvedBaseURL := strings.TrimSpace(baseURL)
	if resolvedBaseURL == "" {
		resolvedBaseURL = DefaultMinimaxBaseURL
	}
	newProvider["baseUrl"] = resolvedBaseURL
	newProvider["apiKey"] = "minimax"
	newProvider["api"] = "openai-completions"
	if len(mergedModels) > 0 {
		newProvider["models"] = mergedModels
	} else {
		newProvider["models"] = []types.ModelDefinitionConfig{hostedModel}
	}
	providers["minimax"] = newProvider

	return ApplyOnboardAuthAgentModelsAndProviders(cfg, models, providers)
}

// ApplyMinimaxConfig 应用 MiniMax 配置并设置为默认模型（LM Studio）。
func ApplyMinimaxConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyMinimaxProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, "lmstudio/minimax-m2.5-gs32")
}

// ApplyMinimaxHostedConfig 应用 MiniMax 托管版配置并设置为默认模型。
func ApplyMinimaxHostedConfig(cfg OpenClawConfig, baseURL string) OpenClawConfig {
	next := ApplyMinimaxHostedProviderConfig(cfg, baseURL)
	defaults := copyMap(getNestedMap(next, "agents", "defaults"))
	model := map[string]interface{}{"primary": MinimaxHostedModelRef}
	defaults["model"] = model
	agents := copyMap(getNestedMap(next, "agents"))
	agents["defaults"] = defaults
	result := copyMap(next)
	result["agents"] = agents
	return result
}

// ──────────────────────────────────────────────
// MiniMax Anthropic API 配置
// ──────────────────────────────────────────────

type minimaxApiProviderConfigParams struct {
	providerID string
	modelID    string
	baseURL    string
}

func applyMinimaxApiProviderConfigWithBaseURL(cfg OpenClawConfig, params minimaxApiProviderConfigParams) OpenClawConfig {
	providers := getProviders(cfg)
	var existingProvider map[string]interface{}
	if raw, ok := providers[params.providerID]; ok && raw != nil {
		if ep, ok := raw.(map[string]interface{}); ok {
			existingProvider = ep
		}
	}
	existingModels := getModelsList(existingProvider)
	apiModel := BuildMinimaxApiModelDefinition(params.modelID)
	hasApiModel := false
	for _, m := range existingModels {
		if m.ID == params.modelID {
			hasApiModel = true
			break
		}
	}
	mergedModels := existingModels
	if !hasApiModel {
		mergedModels = append(append([]types.ModelDefinitionConfig{}, existingModels...), apiModel)
	}

	newProvider := copyMap(existingProvider)
	if len(newProvider) == 0 {
		newProvider["baseUrl"] = params.baseURL
		newProvider["models"] = []types.ModelDefinitionConfig{}
	}
	// 处理 apiKey
	var normalizedApiKey string
	if apiKeyRaw, ok := newProvider["apiKey"]; ok {
		if s, ok := apiKeyRaw.(string); ok {
			trimmed := strings.TrimSpace(s)
			if trimmed == "minimax" {
				trimmed = ""
			}
			if trimmed != "" {
				normalizedApiKey = trimmed
			}
		}
		delete(newProvider, "apiKey")
	}
	newProvider["baseUrl"] = params.baseURL
	newProvider["api"] = "anthropic-messages"
	boolTrue := true
	newProvider["authHeader"] = &boolTrue
	if normalizedApiKey != "" {
		newProvider["apiKey"] = normalizedApiKey
	}
	if len(mergedModels) > 0 {
		newProvider["models"] = mergedModels
	} else {
		newProvider["models"] = []types.ModelDefinitionConfig{apiModel}
	}
	providers[params.providerID] = newProvider

	models := getAgentModels(cfg)
	modelRef := params.providerID + "/" + params.modelID
	setModelAlias(models, modelRef, "Minimax")

	result := copyMap(cfg)
	agents := copyMap(getNestedMap(cfg, "agents"))
	defaults := copyMap(getNestedMap(cfg, "agents", "defaults"))
	defaults["models"] = models
	agents["defaults"] = defaults
	result["agents"] = agents
	result["models"] = map[string]interface{}{
		"mode":      getStringValue(getNestedMap(cfg, "models"), "mode"),
		"providers": providers,
	}
	if result["models"].(map[string]interface{})["mode"] == "" {
		result["models"].(map[string]interface{})["mode"] = "merge"
	}
	return result
}

// ApplyMinimaxApiProviderConfig 应用 MiniMax API Provider 配置（国际版）。
func ApplyMinimaxApiProviderConfig(cfg OpenClawConfig, modelID string) OpenClawConfig {
	if modelID == "" {
		modelID = "MiniMax-M2.5"
	}
	return applyMinimaxApiProviderConfigWithBaseURL(cfg, minimaxApiProviderConfigParams{
		providerID: "minimax", modelID: modelID, baseURL: MinimaxApiBaseURL,
	})
}

// ApplyMinimaxApiConfig 应用 MiniMax API 配置并设置为默认模型（国际版）。
func ApplyMinimaxApiConfig(cfg OpenClawConfig, modelID string) OpenClawConfig {
	if modelID == "" {
		modelID = "MiniMax-M2.5"
	}
	next := applyMinimaxApiProviderConfigWithBaseURL(cfg, minimaxApiProviderConfigParams{
		providerID: "minimax", modelID: modelID, baseURL: MinimaxApiBaseURL,
	})
	return ApplyAgentDefaultModelPrimary(next, "minimax/"+modelID)
}

// ApplyMinimaxApiProviderConfigCn 应用 MiniMax API Provider 配置（中国区）。
func ApplyMinimaxApiProviderConfigCn(cfg OpenClawConfig, modelID string) OpenClawConfig {
	if modelID == "" {
		modelID = "MiniMax-M2.5"
	}
	return applyMinimaxApiProviderConfigWithBaseURL(cfg, minimaxApiProviderConfigParams{
		providerID: "minimax-cn", modelID: modelID, baseURL: MinimaxCnApiBaseURL,
	})
}

// ApplyMinimaxApiConfigCn 应用 MiniMax API 配置并设置为默认模型（中国区）。
func ApplyMinimaxApiConfigCn(cfg OpenClawConfig, modelID string) OpenClawConfig {
	if modelID == "" {
		modelID = "MiniMax-M2.5"
	}
	next := applyMinimaxApiProviderConfigWithBaseURL(cfg, minimaxApiProviderConfigParams{
		providerID: "minimax-cn", modelID: modelID, baseURL: MinimaxCnApiBaseURL,
	})
	return ApplyAgentDefaultModelPrimary(next, "minimax-cn/"+modelID)
}
