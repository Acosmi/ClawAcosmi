// onboard/config_gateways.go — 网关配置（Cloudflare AI Gateway + Vercel AI Gateway）
// 对应 TS 文件: src/commands/onboard-auth.config-gateways.ts
package onboard

import (
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ApplyVercelAiGatewayProviderConfig 应用 Vercel AI Gateway Provider 配置。
func ApplyVercelAiGatewayProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, VercelAiGatewayDefaultModelRef, "Vercel AI Gateway")

	result := copyMap(cfg)
	agents := copyMap(getNestedMap(cfg, "agents"))
	defaults := copyMap(getNestedMap(cfg, "agents", "defaults"))
	defaults["models"] = models
	agents["defaults"] = defaults
	result["agents"] = agents
	return result
}

// ApplyCloudflareAiGatewayProviderConfig 应用 Cloudflare AI Gateway Provider 配置。
func ApplyCloudflareAiGatewayProviderConfig(cfg OpenClawConfig, accountID, gatewayID string) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, CloudflareAiGatewayDefaultModelRef, "Cloudflare AI Gateway")

	defaultModel := types.ModelDefinitionConfig{
		ID: "anthropic/claude-sonnet-4-20250514", Name: "Claude Sonnet 4",
		Reasoning: true, Input: []string{"text", "image"},
		Cost: types.ModelCost{}, ContextWindow: 200000, MaxTokens: 16384,
	}

	// 解析 baseUrl
	var baseURL string
	if accountID != "" && gatewayID != "" {
		baseURL = "https://gateway.ai.cloudflare.com/v1/" + accountID + "/" + gatewayID
	} else {
		existingProvider := getNestedMap(cfg, "models", "providers", "cloudflare-ai-gateway")
		if existingProvider != nil {
			if bu, ok := existingProvider["baseUrl"].(string); ok {
				baseURL = bu
			}
		}
	}

	if baseURL == "" {
		// 无 baseUrl，仅设置 models alias
		result := copyMap(cfg)
		agents := copyMap(getNestedMap(cfg, "agents"))
		defaults := copyMap(getNestedMap(cfg, "agents", "defaults"))
		defaults["models"] = models
		agents["defaults"] = defaults
		result["agents"] = agents
		return result
	}

	return ApplyProviderConfigWithDefaultModel(cfg, models, "cloudflare-ai-gateway",
		"anthropic-messages", baseURL, defaultModel, "")
}

// ApplyVercelAiGatewayConfig 应用 Vercel AI Gateway 配置并设置为默认模型。
func ApplyVercelAiGatewayConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyVercelAiGatewayProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, VercelAiGatewayDefaultModelRef)
}

// ApplyCloudflareAiGatewayConfig 应用 Cloudflare AI Gateway 配置并设置为默认模型。
func ApplyCloudflareAiGatewayConfig(cfg OpenClawConfig, accountID, gatewayID string) OpenClawConfig {
	next := ApplyCloudflareAiGatewayProviderConfig(cfg, accountID, gatewayID)
	return ApplyAgentDefaultModelPrimary(next, CloudflareAiGatewayDefaultModelRef)
}
