package main

// provider_openai_compat.go — 自定义 OpenAI 兼容 Provider 配置
//
// 支持任意 OpenAI 兼容端点注册为命名 provider，替代 OpenRouter 等聚合服务。
// 用户可自行配置多个命名端点，每个端点独立维护 BaseURL、APIKey 和模型列表。

import (
	"fmt"

	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 类型定义 ----------

// CustomOpenAIEndpointConfig 自定义 OpenAI 兼容端点配置。
type CustomOpenAIEndpointConfig struct {
	// Name 端点名称，用作 provider ID 后缀（e.g. "myservice" → provider "custom-myservice"）。
	Name string
	// BaseURL OpenAI 兼容 API 端点，e.g. "https://api.openrouter.ai/v1"。
	BaseURL string
	// Models 端点支持的模型列表（可选，为空时允许用户手动填写模型 ID）。
	Models []CustomOpenAIModelConfig
}

// CustomOpenAIModelConfig 自定义端点的模型定义。
type CustomOpenAIModelConfig struct {
	ID            string
	Name          string
	ContextWindow int
	MaxTokens     int
	Reasoning     bool
}

// ---------- 内部 helper ----------

// customProviderID 由端点名称生成 provider ID。
func customProviderID(name string) string {
	return "custom-" + name
}

// ---------- Provider 配置 ----------

// ApplyCustomOpenAICompatProviderConfig 注册自定义 OpenAI 兼容 provider。
func ApplyCustomOpenAICompatProviderConfig(cfg *types.OpenAcosmiConfig, endpoint CustomOpenAIEndpointConfig) {
	providerID := customProviderID(endpoint.Name)
	p := ensureProvider(cfg, providerID)
	if endpoint.BaseURL != "" && p.BaseURL == "" {
		p.BaseURL = endpoint.BaseURL
	}
	p.API = "openai-completions"
	for _, m := range endpoint.Models {
		mergeModelIfMissing(p, types.ModelDefinitionConfig{
			ID:            m.ID,
			Name:          m.Name,
			ContextWindow: m.ContextWindow,
			MaxTokens:     m.MaxTokens,
			Reasoning:     m.Reasoning,
		})
	}
}

// ApplyCustomOpenAICompatConfig 注册自定义端点并设为默认 provider。
func ApplyCustomOpenAICompatConfig(cfg *types.OpenAcosmiConfig, endpoint CustomOpenAIEndpointConfig, defaultModelID string) {
	ApplyCustomOpenAICompatProviderConfig(cfg, endpoint)
	if defaultModelID != "" {
		ref := fmt.Sprintf("custom-%s/%s", endpoint.Name, defaultModelID)
		setDefaultModel(cfg, ref)
	}
}

// ---------- 凭据 ----------

// SetCustomOpenAICompatApiKey 写入自定义端点 API key。
func SetCustomOpenAICompatApiKey(store *auth.AuthStore, name, key string) error {
	return upsertApiKeyProfile(store, customProviderID(name), key)
}
