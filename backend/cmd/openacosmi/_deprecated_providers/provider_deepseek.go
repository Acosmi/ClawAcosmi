package main

// provider_deepseek.go — DeepSeek provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	DeepSeekBaseURL         = "https://api.deepseek.com/v1"
	DeepSeekDefaultModelID  = "deepseek-chat"
	DeepSeekDefaultModelRef = "deepseek/deepseek-chat"
)

// ---------- Provider 配置 ----------

// ApplyDeepSeekProviderConfig 注册 DeepSeek provider 及模型列表。
func ApplyDeepSeekProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, DeepSeekDefaultModelRef, "DeepSeek Chat")

	p := ensureProvider(cfg, "deepseek")
	if p.BaseURL == "" {
		p.BaseURL = DeepSeekBaseURL
	}
	p.API = "openai-completions"

	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "deepseek-chat",
		Name:          "DeepSeek Chat (V3.2)",
		ContextWindow: 131_072, // DeepSeek-V3.2 实际 128K
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "deepseek-reasoner",
		Name:          "DeepSeek Reasoner (V3.2)",
		Reasoning:     true,
		ContextWindow: 131_072, // DeepSeek-V3.2 实际 128K
		MaxTokens:     65_536,  // 官方最大 64K（含 reasoning token，来源: api-docs.deepseek.com）
	})
}

// ApplyDeepSeekConfig 注册 DeepSeek 并设为默认。
func ApplyDeepSeekConfig(cfg *types.OpenAcosmiConfig) {
	ApplyDeepSeekProviderConfig(cfg)
	setDefaultModel(cfg, DeepSeekDefaultModelRef)
}

// ---------- 凭据 ----------

// SetDeepSeekApiKey 写入 DeepSeek API key。
func SetDeepSeekApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "deepseek", key)
}
