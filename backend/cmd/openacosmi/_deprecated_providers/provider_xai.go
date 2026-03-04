package main

// provider_xai.go — xAI Grok provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	xaiBaseURL  = "https://api.x.ai/v1"
	xaiModelRef = "xai/grok-4"
	xaiModelID  = "grok-4"
)

// ---------- Provider 配置 ----------

// ApplyXaiProviderConfig 注册 xAI/Grok provider 及模型列表。
func ApplyXaiProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, xaiModelRef, "Grok")
	p := ensureProvider(cfg, "xai")
	p.BaseURL = xaiBaseURL
	p.API = "openai-completions"
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "grok-4",
		Name:          "Grok 4",
		ContextWindow: 256_000,
		MaxTokens:     16_384,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "grok-4-fast-reasoning",
		Name:          "Grok 4 Fast (Reasoning)",
		Reasoning:     true,
		ContextWindow: 2_000_000,
		MaxTokens:     32_768,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "grok-4-fast-non-reasoning",
		Name:          "Grok 4 Fast",
		ContextWindow: 2_000_000,
		MaxTokens:     32_768,
	})
	// ---------- Grok 4.1 系列 ----------
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "grok-4-1-fast-reasoning",
		Name:          "Grok 4.1 Fast (Reasoning)",
		Reasoning:     true,
		ContextWindow: 2_000_000,
		MaxTokens:     32_768,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "grok-4-1-fast-non-reasoning",
		Name:          "Grok 4.1 Fast",
		ContextWindow: 2_000_000,
		MaxTokens:     32_768,
	})
	// ---------- 编程专用 ----------
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "grok-code-fast-1",
		Name:          "Grok Code Fast",
		ContextWindow: 256_000,
		MaxTokens:     16_384,
	})
	// ---------- Grok 3 系列 ----------
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "grok-3",
		Name:          "Grok 3",
		ContextWindow: 131_072,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "grok-3-mini",
		Name:          "Grok 3 Mini",
		ContextWindow: 131_072,
		MaxTokens:     8_192,
	})
}

// ApplyXaiConfig 注册并设为默认。
func ApplyXaiConfig(cfg *types.OpenAcosmiConfig) {
	ApplyXaiProviderConfig(cfg)
	setDefaultModel(cfg, xaiModelRef)
}

// ---------- 凭据 ----------

// SetXaiApiKey 写入 xAI/Grok API key。
func SetXaiApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "xai", key)
}
