// onboard/config_shared.go — 共享配置工具函数
// 对应 TS 文件: src/commands/onboard-auth.config-shared.ts
// 提供 Provider 配置的公共合并和应用逻辑。
package onboard

import (
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// OpenClawConfig 配置类型（与 authchoice 包保持一致）。
type OpenClawConfig = map[string]interface{}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────

// getNestedMap 安全获取嵌套 map。
func getNestedMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	current := m
	for _, key := range keys {
		if current == nil {
			return nil
		}
		val, ok := current[key]
		if !ok || val == nil {
			return nil
		}
		next, ok := val.(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

// getStringValue 安全获取字符串值。
func getStringValue(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	val, ok := m[key]
	if !ok || val == nil {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return s
}

// copyMap 浅拷贝 map。
func copyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// getModelsList 获取 Provider 配置中的 models 列表。
func getModelsList(provider map[string]interface{}) []types.ModelDefinitionConfig {
	if provider == nil {
		return nil
	}
	modelsRaw, ok := provider["models"]
	if !ok || modelsRaw == nil {
		return nil
	}
	models, ok := modelsRaw.([]types.ModelDefinitionConfig)
	if !ok {
		return nil
	}
	return models
}

// extractAgentDefaultModelFallbacks 提取代理默认模型的 fallback 列表。
func extractAgentDefaultModelFallbacks(model interface{}) []string {
	if model == nil {
		return nil
	}
	m, ok := model.(map[string]interface{})
	if !ok {
		return nil
	}
	fallbacksRaw, ok := m["fallbacks"]
	if !ok || fallbacksRaw == nil {
		return nil
	}
	fallbacks, ok := fallbacksRaw.([]string)
	if ok {
		return fallbacks
	}
	// 尝试 []interface{} 转换
	ifaceSlice, ok := fallbacksRaw.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(ifaceSlice))
	for _, v := range ifaceSlice {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ──────────────────────────────────────────────
// 核心配置应用函数
// ──────────────────────────────────────────────

// ApplyOnboardAuthAgentModelsAndProviders 应用代理模型和 Provider 配置。
// 对应 TS: applyOnboardAuthAgentModelsAndProviders()
func ApplyOnboardAuthAgentModelsAndProviders(
	cfg OpenClawConfig,
	agentModels map[string]interface{},
	providers map[string]interface{},
) OpenClawConfig {
	result := copyMap(cfg)

	agents := copyMap(getNestedMap(cfg, "agents"))
	defaults := copyMap(getNestedMap(cfg, "agents", "defaults"))
	defaults["models"] = agentModels
	agents["defaults"] = defaults
	result["agents"] = agents

	models := copyMap(getNestedMap(cfg, "models"))
	if models["mode"] == nil {
		models["mode"] = "merge"
	}
	models["providers"] = providers
	result["models"] = models

	return result
}

// ApplyAgentDefaultModelPrimary 设置代理默认主模型。
// 对应 TS: applyAgentDefaultModelPrimary()
func ApplyAgentDefaultModelPrimary(cfg OpenClawConfig, primary string) OpenClawConfig {
	result := copyMap(cfg)
	agents := copyMap(getNestedMap(cfg, "agents"))
	defaults := copyMap(getNestedMap(cfg, "agents", "defaults"))

	existingFallbacks := extractAgentDefaultModelFallbacks(defaults["model"])
	model := map[string]interface{}{
		"primary": primary,
	}
	if existingFallbacks != nil {
		model["fallbacks"] = existingFallbacks
	}
	defaults["model"] = model
	agents["defaults"] = defaults
	result["agents"] = agents
	return result
}

// ──────────────────────────────────────────────
// Provider 配置合并状态
// ──────────────────────────────────────────────

type providerModelMergeState struct {
	providers        map[string]interface{}
	existingProvider map[string]interface{}
	existingModels   []types.ModelDefinitionConfig
}

func resolveProviderModelMergeState(cfg OpenClawConfig, providerID string) providerModelMergeState {
	providers := copyMap(getNestedMap(cfg, "models", "providers"))
	var existingProvider map[string]interface{}
	if raw, ok := providers[providerID]; ok && raw != nil {
		if ep, ok := raw.(map[string]interface{}); ok {
			existingProvider = ep
		}
	}
	existingModels := getModelsList(existingProvider)
	return providerModelMergeState{
		providers:        providers,
		existingProvider: existingProvider,
		existingModels:   existingModels,
	}
}

// buildProviderConfig 构建 Provider 配置 map（保留旧 key，排除 apiKey 重写）。
func buildProviderConfig(
	existingProvider map[string]interface{},
	api string,
	baseURL string,
	mergedModels []types.ModelDefinitionConfig,
	fallbackModels []types.ModelDefinitionConfig,
) map[string]interface{} {
	result := copyMap(existingProvider)
	// 提取并规范化 apiKey
	var normalizedApiKey string
	if apiKeyRaw, ok := result["apiKey"]; ok {
		if s, ok := apiKeyRaw.(string); ok {
			trimmed := s
			if len(trimmed) > 0 {
				normalizedApiKey = trimmed
			}
		}
		delete(result, "apiKey")
	}
	result["baseUrl"] = baseURL
	result["api"] = api
	if normalizedApiKey != "" {
		result["apiKey"] = normalizedApiKey
	}
	if len(mergedModels) > 0 {
		result["models"] = mergedModels
	} else {
		result["models"] = fallbackModels
	}
	return result
}

// ──────────────────────────────────────────────
// 顶层配置应用函数
// ──────────────────────────────────────────────

// ApplyProviderConfigWithDefaultModels 使用默认模型列表应用 Provider 配置。
// 对应 TS: applyProviderConfigWithDefaultModels()
func ApplyProviderConfigWithDefaultModels(
	cfg OpenClawConfig,
	agentModels map[string]interface{},
	providerID string,
	api string,
	baseURL string,
	defaultModels []types.ModelDefinitionConfig,
	defaultModelID string,
) OpenClawConfig {
	state := resolveProviderModelMergeState(cfg, providerID)

	resolvedDefaultModelID := defaultModelID
	if resolvedDefaultModelID == "" && len(defaultModels) > 0 {
		resolvedDefaultModelID = defaultModels[0].ID
	}
	hasDefaultModel := true
	if resolvedDefaultModelID != "" {
		hasDefaultModel = false
		for _, m := range state.existingModels {
			if m.ID == resolvedDefaultModelID {
				hasDefaultModel = true
				break
			}
		}
	}
	var mergedModels []types.ModelDefinitionConfig
	if len(state.existingModels) > 0 {
		if hasDefaultModel || len(defaultModels) == 0 {
			mergedModels = state.existingModels
		} else {
			mergedModels = append(append([]types.ModelDefinitionConfig{}, state.existingModels...), defaultModels...)
		}
	} else {
		mergedModels = defaultModels
	}

	providerCfg := buildProviderConfig(state.existingProvider, api, baseURL, mergedModels, defaultModels)
	state.providers[providerID] = providerCfg
	return ApplyOnboardAuthAgentModelsAndProviders(cfg, agentModels, state.providers)
}

// ApplyProviderConfigWithDefaultModel 使用单个默认模型应用 Provider 配置。
// 对应 TS: applyProviderConfigWithDefaultModel()
func ApplyProviderConfigWithDefaultModel(
	cfg OpenClawConfig,
	agentModels map[string]interface{},
	providerID string,
	api string,
	baseURL string,
	defaultModel types.ModelDefinitionConfig,
	defaultModelID string,
) OpenClawConfig {
	resolvedID := defaultModelID
	if resolvedID == "" {
		resolvedID = defaultModel.ID
	}
	return ApplyProviderConfigWithDefaultModels(
		cfg, agentModels, providerID, api, baseURL,
		[]types.ModelDefinitionConfig{defaultModel}, resolvedID,
	)
}

// ApplyProviderConfigWithModelCatalog 使用模型目录应用 Provider 配置。
// 对应 TS: applyProviderConfigWithModelCatalog()
func ApplyProviderConfigWithModelCatalog(
	cfg OpenClawConfig,
	agentModels map[string]interface{},
	providerID string,
	api string,
	baseURL string,
	catalogModels []types.ModelDefinitionConfig,
) OpenClawConfig {
	state := resolveProviderModelMergeState(cfg, providerID)
	var mergedModels []types.ModelDefinitionConfig
	if len(state.existingModels) > 0 {
		// 合并：先保留现有，再添加目录中不存在的
		mergedModels = append([]types.ModelDefinitionConfig{}, state.existingModels...)
		existingIDs := make(map[string]bool, len(state.existingModels))
		for _, m := range state.existingModels {
			existingIDs[m.ID] = true
		}
		for _, m := range catalogModels {
			if !existingIDs[m.ID] {
				mergedModels = append(mergedModels, m)
			}
		}
	} else {
		mergedModels = catalogModels
	}

	providerCfg := buildProviderConfig(state.existingProvider, api, baseURL, mergedModels, catalogModels)
	state.providers[providerID] = providerCfg
	return ApplyOnboardAuthAgentModelsAndProviders(cfg, agentModels, state.providers)
}
