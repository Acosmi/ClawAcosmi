// onboard/config_litellm.go — LiteLLM 配置
// 对应 TS 文件: src/commands/onboard-auth.config-litellm.ts
package onboard

import (
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// LitellmBaseURL LiteLLM 默认基础 URL。
const LitellmBaseURL = "http://localhost:4000"

// LitellmDefaultModelID LiteLLM 默认模型 ID。
const LitellmDefaultModelID = "claude-opus-4-6"

const litellmDefaultContextWindow = 128000
const litellmDefaultMaxTokens = 8192

var litellmDefaultCost = types.ModelCost{}

func buildLitellmModelDefinition() types.ModelDefinitionConfig {
	return types.ModelDefinitionConfig{
		ID:            LitellmDefaultModelID,
		Name:          "Claude Opus 4.6",
		Reasoning:     true,
		Input:         []string{"text", "image"},
		Cost:          litellmDefaultCost,
		ContextWindow: litellmDefaultContextWindow,
		MaxTokens:     litellmDefaultMaxTokens,
	}
}

// ApplyLitellmProviderConfig 应用 LiteLLM Provider 配置。
func ApplyLitellmProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, LitellmDefaultModelRef, "LiteLLM")

	defaultModel := buildLitellmModelDefinition()

	existingProvider := getNestedMap(cfg, "models", "providers", "litellm")
	resolvedBaseURL := ""
	if existingProvider != nil {
		if bu, ok := existingProvider["baseUrl"].(string); ok {
			resolvedBaseURL = bu
		}
	}
	if resolvedBaseURL == "" {
		resolvedBaseURL = LitellmBaseURL
	}

	return ApplyProviderConfigWithDefaultModel(cfg, models, "litellm",
		"openai-completions", resolvedBaseURL, defaultModel, LitellmDefaultModelID)
}

// ApplyLitellmConfig 应用 LiteLLM 配置并设置为默认模型。
func ApplyLitellmConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyLitellmProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, LitellmDefaultModelRef)
}
