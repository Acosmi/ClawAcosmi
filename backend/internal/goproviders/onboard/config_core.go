// onboard/config_core.go — 核心 Provider 配置函数（第一批：骨架 + ZAI/OpenRouter/Moonshot）
// 对应 TS 文件: src/commands/onboard-auth.config-core.ts
// 包含各 Provider 的 apply*Config/apply*ProviderConfig 配置函数。
package onboard

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ──────────────────────────────────────────────
// 辅助：获取/设置 agent models
// ──────────────────────────────────────────────

func getAgentModels(cfg OpenClawConfig) map[string]interface{} {
	models := getNestedMap(cfg, "agents", "defaults", "models")
	return copyMap(models)
}

func getProviders(cfg OpenClawConfig) map[string]interface{} {
	providers := getNestedMap(cfg, "models", "providers")
	return copyMap(providers)
}

func setModelAlias(models map[string]interface{}, modelRef, defaultAlias string) {
	existing, _ := models[modelRef].(map[string]interface{})
	entry := copyMap(existing)
	if _, hasAlias := entry["alias"]; !hasAlias {
		entry["alias"] = defaultAlias
	}
	models[modelRef] = entry
}

// ──────────────────────────────────────────────
// Z.AI 配置
// ──────────────────────────────────────────────

// ZaiConfigParams Z.AI 配置参数。
type ZaiConfigParams struct {
	Endpoint string
	ModelID  string
}

// ApplyZaiProviderConfig 应用 Z.AI Provider 配置（不改变默认模型）。
func ApplyZaiProviderConfig(cfg OpenClawConfig, params *ZaiConfigParams) OpenClawConfig {
	modelID := ZaiDefaultModelID
	if params != nil && strings.TrimSpace(params.ModelID) != "" {
		modelID = strings.TrimSpace(params.ModelID)
	}
	modelRef := "zai/" + modelID

	models := getAgentModels(cfg)
	setModelAlias(models, modelRef, "GLM")

	providers := getProviders(cfg)
	var existingProvider map[string]interface{}
	if raw, ok := providers["zai"]; ok && raw != nil {
		if ep, ok := raw.(map[string]interface{}); ok {
			existingProvider = ep
		}
	}
	existingModels := getModelsList(existingProvider)

	defaultModels := []types.ModelDefinitionConfig{
		BuildZaiModelDefinition(BuildZaiModelDefinitionParams{ID: "glm-5"}),
		BuildZaiModelDefinition(BuildZaiModelDefinitionParams{ID: "glm-4.7"}),
		BuildZaiModelDefinition(BuildZaiModelDefinitionParams{ID: "glm-4.7-flash"}),
		BuildZaiModelDefinition(BuildZaiModelDefinitionParams{ID: "glm-4.7-flashx"}),
	}

	mergedModels := make([]types.ModelDefinitionConfig, 0, len(existingModels)+len(defaultModels))
	mergedModels = append(mergedModels, existingModels...)
	seen := make(map[string]bool, len(existingModels))
	for _, m := range existingModels {
		seen[m.ID] = true
	}
	for _, m := range defaultModels {
		if !seen[m.ID] {
			mergedModels = append(mergedModels, m)
			seen[m.ID] = true
		}
	}

	// 提取已有 apiKey
	newProvider := copyMap(existingProvider)
	var normalizedApiKey string
	if apiKeyRaw, ok := newProvider["apiKey"]; ok {
		if s, ok := apiKeyRaw.(string); ok && strings.TrimSpace(s) != "" {
			normalizedApiKey = strings.TrimSpace(s)
		}
		delete(newProvider, "apiKey")
	}

	baseURL := ""
	if params != nil && params.Endpoint != "" {
		baseURL = ResolveZaiBaseURL(params.Endpoint)
	}
	if baseURL == "" {
		if existing, ok := newProvider["baseUrl"].(string); ok && existing != "" {
			baseURL = existing
		} else {
			baseURL = ResolveZaiBaseURL("")
		}
	}

	newProvider["baseUrl"] = baseURL
	newProvider["api"] = "openai-completions"
	if normalizedApiKey != "" {
		newProvider["apiKey"] = normalizedApiKey
	}
	if len(mergedModels) > 0 {
		newProvider["models"] = mergedModels
	} else {
		newProvider["models"] = defaultModels
	}
	providers["zai"] = newProvider

	return ApplyOnboardAuthAgentModelsAndProviders(cfg, models, providers)
}

// ApplyZaiConfig 应用 Z.AI 配置并设置为默认模型。
func ApplyZaiConfig(cfg OpenClawConfig, params *ZaiConfigParams) OpenClawConfig {
	modelID := ZaiDefaultModelID
	if params != nil && strings.TrimSpace(params.ModelID) != "" {
		modelID = strings.TrimSpace(params.ModelID)
	}
	modelRef := ZaiDefaultModelRef
	if modelID != ZaiDefaultModelID {
		modelRef = "zai/" + modelID
	}
	next := ApplyZaiProviderConfig(cfg, params)
	return ApplyAgentDefaultModelPrimary(next, modelRef)
}

// ──────────────────────────────────────────────
// OpenRouter 配置
// ──────────────────────────────────────────────

// ApplyOpenrouterProviderConfig 应用 OpenRouter Provider 配置。
func ApplyOpenrouterProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, OpenrouterDefaultModelRef, "OpenRouter")

	result := copyMap(cfg)
	agents := copyMap(getNestedMap(cfg, "agents"))
	defaults := copyMap(getNestedMap(cfg, "agents", "defaults"))
	defaults["models"] = models
	agents["defaults"] = defaults
	result["agents"] = agents
	return result
}

// ApplyOpenrouterConfig 应用 OpenRouter 配置并设置为默认模型。
func ApplyOpenrouterConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyOpenrouterProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, OpenrouterDefaultModelRef)
}

// ──────────────────────────────────────────────
// Moonshot 配置
// ──────────────────────────────────────────────

func applyMoonshotProviderConfigWithBaseURL(cfg OpenClawConfig, baseURL string) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, MoonshotDefaultModelRef, "Kimi")
	defaultModel := BuildMoonshotModelDefinition()
	return ApplyProviderConfigWithDefaultModel(cfg, models, "moonshot",
		"openai-completions", baseURL, defaultModel, MoonshotDefaultModelID)
}

// ApplyMoonshotProviderConfig 应用 Moonshot Provider 配置。
func ApplyMoonshotProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	return applyMoonshotProviderConfigWithBaseURL(cfg, MoonshotBaseURL)
}

// ApplyMoonshotProviderConfigCn 应用 Moonshot 中国区 Provider 配置。
func ApplyMoonshotProviderConfigCn(cfg OpenClawConfig) OpenClawConfig {
	return applyMoonshotProviderConfigWithBaseURL(cfg, MoonshotCnBaseURL)
}

// ApplyMoonshotConfig 应用 Moonshot 配置并设置为默认模型。
func ApplyMoonshotConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyMoonshotProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, MoonshotDefaultModelRef)
}

// ApplyMoonshotConfigCn 应用 Moonshot 中国区配置并设置为默认模型。
func ApplyMoonshotConfigCn(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyMoonshotProviderConfigCn(cfg)
	return ApplyAgentDefaultModelPrimary(next, MoonshotDefaultModelRef)
}

// ──────────────────────────────────────────────
// Kimi Code 配置
// ──────────────────────────────────────────────

// ApplyKimiCodeProviderConfig 应用 Kimi Code Provider 配置。
func ApplyKimiCodeProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, KimiCodingModelRef, "Kimi for Coding")
	defaultModel := types.ModelDefinitionConfig{
		ID:            KimiCodingModelID,
		Name:          "Kimi K2.5 for Coding",
		Reasoning:     true,
		Input:         []string{"text"},
		Cost:          types.ModelCost{},
		ContextWindow: 131072,
		MaxTokens:     65536,
	}
	return ApplyProviderConfigWithDefaultModel(cfg, models, "kimi-coding",
		"anthropic-messages", "https://api.kimi.com/coding/", defaultModel, KimiCodingModelID)
}

// ApplyKimiCodeConfig 应用 Kimi Code 配置并设置为默认模型。
func ApplyKimiCodeConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyKimiCodeProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, KimiCodingModelRef)
}

// ──────────────────────────────────────────────
// Synthetic 配置
// ──────────────────────────────────────────────

// SyntheticBaseURL Synthetic API 基础 URL。
const SyntheticBaseURL = "https://api.synthetic.dev/v1"

// SyntheticDefaultModelRef Synthetic 默认模型引用。
const SyntheticDefaultModelRef = "synthetic/MiniMax-M2.5"

// SyntheticDefaultModelID Synthetic 默认模型 ID。
const SyntheticDefaultModelID = "MiniMax-M2.5"

// ApplySyntheticProviderConfig 应用 Synthetic Provider 配置。
func ApplySyntheticProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, SyntheticDefaultModelRef, "MiniMax M2.5")

	providers := getProviders(cfg)
	var existingProvider map[string]interface{}
	if raw, ok := providers["synthetic"]; ok && raw != nil {
		if ep, ok := raw.(map[string]interface{}); ok {
			existingProvider = ep
		}
	}
	existingModels := getModelsList(existingProvider)

	// 构建 Synthetic 模型目录（此处简化为单模型）
	syntheticModels := []types.ModelDefinitionConfig{
		{
			ID: SyntheticDefaultModelID, Name: "MiniMax M2.5", Reasoning: true,
			Input: []string{"text"}, Cost: types.ModelCost{}, ContextWindow: 200000, MaxTokens: 8192,
		},
	}
	mergedModels := make([]types.ModelDefinitionConfig, 0, len(existingModels)+len(syntheticModels))
	mergedModels = append(mergedModels, existingModels...)
	existingIDs := make(map[string]bool, len(existingModels))
	for _, m := range existingModels {
		existingIDs[m.ID] = true
	}
	for _, m := range syntheticModels {
		if !existingIDs[m.ID] {
			mergedModels = append(mergedModels, m)
		}
	}

	newProvider := copyMap(existingProvider)
	var normalizedApiKey string
	if apiKeyRaw, ok := newProvider["apiKey"]; ok {
		if s, ok := apiKeyRaw.(string); ok && strings.TrimSpace(s) != "" {
			normalizedApiKey = strings.TrimSpace(s)
		}
		delete(newProvider, "apiKey")
	}
	newProvider["baseUrl"] = SyntheticBaseURL
	newProvider["api"] = "anthropic-messages"
	if normalizedApiKey != "" {
		newProvider["apiKey"] = normalizedApiKey
	}
	if len(mergedModels) > 0 {
		newProvider["models"] = mergedModels
	} else {
		newProvider["models"] = syntheticModels
	}
	providers["synthetic"] = newProvider

	return ApplyOnboardAuthAgentModelsAndProviders(cfg, models, providers)
}

// ApplySyntheticConfig 应用 Synthetic 配置并设置为默认模型。
func ApplySyntheticConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplySyntheticProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, SyntheticDefaultModelRef)
}

// ──────────────────────────────────────────────
// 小米 配置
// ──────────────────────────────────────────────

// XiaomiDefaultModelID 小米默认模型 ID。
const XiaomiDefaultModelID = "mimo-v2-flash"

// ApplyXiaomiProviderConfig 应用小米 Provider 配置。
func ApplyXiaomiProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, XiaomiDefaultModelRef, "Xiaomi")
	defaultModels := []types.ModelDefinitionConfig{
		{
			ID: XiaomiDefaultModelID, Name: "MiMo V2 Flash", Reasoning: true,
			Input: []string{"text"}, Cost: types.ModelCost{}, ContextWindow: 131072, MaxTokens: 16384,
		},
	}
	return ApplyProviderConfigWithDefaultModels(cfg, models, "xiaomi",
		"openai-completions", "https://api.xiaomi.com/v1", defaultModels, XiaomiDefaultModelID)
}

// ApplyXiaomiConfig 应用小米配置并设置为默认模型。
func ApplyXiaomiConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyXiaomiProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, XiaomiDefaultModelRef)
}

// ──────────────────────────────────────────────
// Venice 配置
// ──────────────────────────────────────────────

// VeniceBaseURL Venice API 基础 URL。
const VeniceBaseURL = "https://api.venice.ai/api/v1"

// VeniceDefaultModelRef Venice 默认模型引用。
const VeniceDefaultModelRef = "venice/llama-3.3-70b"

// ApplyVeniceProviderConfig 应用 Venice Provider 配置。
func ApplyVeniceProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, VeniceDefaultModelRef, "Llama 3.3 70B")
	veniceModels := []types.ModelDefinitionConfig{
		{
			ID: "llama-3.3-70b", Name: "Llama 3.3 70B", Reasoning: false,
			Input: []string{"text"}, Cost: types.ModelCost{}, ContextWindow: 131072, MaxTokens: 8192,
		},
	}
	return ApplyProviderConfigWithModelCatalog(cfg, models, "venice",
		"openai-completions", VeniceBaseURL, veniceModels)
}

// ApplyVeniceConfig 应用 Venice 配置并设置为默认模型。
func ApplyVeniceConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyVeniceProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, VeniceDefaultModelRef)
}

// ──────────────────────────────────────────────
// Together 配置
// ──────────────────────────────────────────────

// TogetherBaseURL Together API 基础 URL。
const TogetherBaseURL = "https://api.together.xyz/v1"

// ApplyTogetherProviderConfig 应用 Together Provider 配置。
func ApplyTogetherProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, TogetherDefaultModelRef, "Together AI")
	togetherModels := []types.ModelDefinitionConfig{
		{
			ID: "moonshotai/Kimi-K2.5", Name: "Kimi K2.5", Reasoning: false,
			Input: []string{"text"}, Cost: types.ModelCost{}, ContextWindow: 131072, MaxTokens: 8192,
		},
	}
	return ApplyProviderConfigWithModelCatalog(cfg, models, "together",
		"openai-completions", TogetherBaseURL, togetherModels)
}

// ApplyTogetherConfig 应用 Together 配置并设置为默认模型。
func ApplyTogetherConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyTogetherProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, TogetherDefaultModelRef)
}

// ──────────────────────────────────────────────
// Hugging Face 配置
// ──────────────────────────────────────────────

// HuggingfaceBaseURL Hugging Face API 基础 URL。
const HuggingfaceBaseURL = "https://router.huggingface.co/v1"

// ApplyHuggingfaceProviderConfig 应用 Hugging Face Provider 配置。
func ApplyHuggingfaceProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, HuggingfaceDefaultModelRef, "Hugging Face")
	hfModels := []types.ModelDefinitionConfig{
		{
			ID: "deepseek-ai/DeepSeek-R1", Name: "DeepSeek R1", Reasoning: true,
			Input: []string{"text"}, Cost: types.ModelCost{}, ContextWindow: 131072, MaxTokens: 8192,
		},
	}
	return ApplyProviderConfigWithModelCatalog(cfg, models, "huggingface",
		"openai-completions", HuggingfaceBaseURL, hfModels)
}

// ApplyHuggingfaceConfig 应用 Hugging Face 配置并设置为默认模型。
func ApplyHuggingfaceConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyHuggingfaceProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, HuggingfaceDefaultModelRef)
}

// ──────────────────────────────────────────────
// xAI 配置
// ──────────────────────────────────────────────

// ApplyXaiProviderConfig 应用 xAI Provider 配置。
func ApplyXaiProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, XaiDefaultModelRef, "Grok")
	defaultModel := BuildXaiModelDefinition()
	return ApplyProviderConfigWithDefaultModel(cfg, models, "xai",
		"openai-completions", XaiBaseURL, defaultModel, XaiDefaultModelID)
}

// ApplyXaiConfig 应用 xAI 配置并设置为默认模型。
func ApplyXaiConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyXaiProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, XaiDefaultModelRef)
}

// ──────────────────────────────────────────────
// Mistral 配置
// ──────────────────────────────────────────────

// ApplyMistralProviderConfig 应用 Mistral Provider 配置。
func ApplyMistralProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, MistralDefaultModelRef, "Mistral")
	defaultModel := BuildMistralModelDefinition()
	return ApplyProviderConfigWithDefaultModel(cfg, models, "mistral",
		"openai-completions", MistralBaseURL, defaultModel, MistralDefaultModelID)
}

// ApplyMistralConfig 应用 Mistral 配置并设置为默认模型。
func ApplyMistralConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyMistralProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, MistralDefaultModelRef)
}

// ──────────────────────────────────────────────
// Kilocode 配置
// ──────────────────────────────────────────────

// ApplyKilocodeProviderConfig 应用 Kilocode Provider 配置。
func ApplyKilocodeProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, KilocodeDefaultModelRef, "Kilo Gateway")
	kilocodeModels := []types.ModelDefinitionConfig{BuildKilocodeModelDefinition()}
	return ApplyProviderConfigWithModelCatalog(cfg, models, "kilocode",
		"openai-completions", KilocodeBaseURL, kilocodeModels)
}

// ApplyKilocodeConfig 应用 Kilocode 配置并设置为默认模型。
func ApplyKilocodeConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyKilocodeProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, KilocodeDefaultModelRef)
}

// ──────────────────────────────────────────────
// AuthProfile 配置
// ──────────────────────────────────────────────

// ApplyAuthProfileConfigParams AuthProfile 配置参数。
type ApplyAuthProfileConfigParams struct {
	ProfileID          string
	Provider           string
	Mode               string // "api_key" | "oauth" | "token"
	Email              string
	PreferProfileFirst *bool
}

// ApplyAuthProfileConfig 应用认证 Profile 配置到 OpenClawConfig。
// 对应 TS: applyAuthProfileConfig()
func ApplyAuthProfileConfig(cfg OpenClawConfig, params ApplyAuthProfileConfigParams) OpenClawConfig {
	normalizedProvider := strings.ToLower(params.Provider)

	// 获取现有 profiles
	auth := copyMap(getNestedMap(cfg, "auth"))
	profiles := copyMap(getNestedMap(auth, "profiles"))

	profile := map[string]interface{}{
		"provider": params.Provider,
		"mode":     params.Mode,
	}
	if params.Email != "" {
		profile["email"] = params.Email
	}
	profiles[params.ProfileID] = profile

	// 计算已配置的同 Provider 的 profiles
	var configuredProfileIDs []struct {
		id   string
		mode string
	}
	for id, raw := range profiles {
		if p, ok := raw.(map[string]interface{}); ok {
			if prov, _ := p["provider"].(string); strings.ToLower(prov) == normalizedProvider {
				mode, _ := p["mode"].(string)
				configuredProfileIDs = append(configuredProfileIDs, struct {
					id   string
					mode string
				}{id, mode})
			}
		}
	}

	preferFirst := true
	if params.PreferProfileFirst != nil {
		preferFirst = *params.PreferProfileFirst
	}

	// 处理 order
	existingOrder := getNestedMap(auth, "order")
	var existingProviderOrder []string
	hasExistingProviderOrder := false
	if existingOrder != nil {
		if raw, ok := existingOrder[params.Provider]; ok && raw != nil {
			if arr, ok := raw.([]interface{}); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						existingProviderOrder = append(existingProviderOrder, s)
					}
				}
				hasExistingProviderOrder = true
			} else if arr, ok := raw.([]string); ok {
				existingProviderOrder = arr
				hasExistingProviderOrder = true
			}
		}
	}

	var order map[string]interface{}
	if hasExistingProviderOrder {
		reordered := existingProviderOrder
		if preferFirst {
			newOrder := []string{params.ProfileID}
			for _, id := range existingProviderOrder {
				if id != params.ProfileID {
					newOrder = append(newOrder, id)
				}
			}
			reordered = newOrder
		}
		// 确保 profileID 存在
		found := false
		for _, id := range reordered {
			if id == params.ProfileID {
				found = true
				break
			}
		}
		if !found {
			reordered = append(reordered, params.ProfileID)
		}
		order = copyMap(existingOrder)
		order[params.Provider] = reordered
	} else {
		// 检查是否有混合模式
		hasMixed := false
		for _, cp := range configuredProfileIDs {
			if cp.id != params.ProfileID && cp.mode != params.Mode {
				hasMixed = true
				break
			}
		}
		if preferFirst && hasMixed {
			derived := []string{params.ProfileID}
			for _, cp := range configuredProfileIDs {
				if cp.id != params.ProfileID {
					derived = append(derived, cp.id)
				}
			}
			order = copyMap(existingOrder)
			order[params.Provider] = derived
		} else if existingOrder != nil {
			order = existingOrder
		}
	}

	auth["profiles"] = profiles
	if order != nil {
		auth["order"] = order
	}
	result := copyMap(cfg)
	result["auth"] = auth
	return result
}

// ──────────────────────────────────────────────
// 千帆 配置
// ──────────────────────────────────────────────

// ApplyQianfanProviderConfig 应用千帆 Provider 配置。
func ApplyQianfanProviderConfig(cfg OpenClawConfig) OpenClawConfig {
	models := getAgentModels(cfg)
	setModelAlias(models, QianfanDefaultModelRef, "QIANFAN")
	defaultModels := []types.ModelDefinitionConfig{
		{
			ID: QianfanDefaultModelID, Name: "ERNIE X1 Turbo 32K", Reasoning: false,
			Input: []string{"text"}, Cost: types.ModelCost{}, ContextWindow: 32768, MaxTokens: 8192,
		},
	}

	// 解析现有 baseUrl 和 api
	var resolvedBaseURL, resolvedApi string
	existingProvider := getNestedMap(cfg, "models", "providers", "qianfan")
	if existingProvider != nil {
		if bu, ok := existingProvider["baseUrl"].(string); ok && strings.TrimSpace(bu) != "" {
			resolvedBaseURL = strings.TrimSpace(bu)
		}
		if a, ok := existingProvider["api"].(string); ok && a != "" {
			resolvedApi = a
		}
	}
	if resolvedBaseURL == "" {
		resolvedBaseURL = QianfanBaseURL
	}
	if resolvedApi == "" {
		resolvedApi = "openai-completions"
	}

	return ApplyProviderConfigWithDefaultModels(cfg, models, "qianfan",
		resolvedApi, resolvedBaseURL, defaultModels, QianfanDefaultModelID)
}

// ApplyQianfanConfig 应用千帆配置并设置为默认模型。
func ApplyQianfanConfig(cfg OpenClawConfig) OpenClawConfig {
	next := ApplyQianfanProviderConfig(cfg)
	return ApplyAgentDefaultModelPrimary(next, QianfanDefaultModelRef)
}
