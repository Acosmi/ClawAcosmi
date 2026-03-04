package main

// provider_openai.go — OpenAI provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	OpenAIBaseURL         = "https://api.openai.com/v1"
	OpenAIDefaultModelID  = "gpt-4.1"
	OpenAIDefaultModelRef = "openai/gpt-4.1"
)

// ---------- Provider 配置 ----------

// ApplyOpenAIProviderConfig 注册 OpenAI provider 及模型列表。
func ApplyOpenAIProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, OpenAIDefaultModelRef, "GPT-4.1")
	setModelAlias(cfg, "openai/o3", "o3")

	p := ensureProvider(cfg, "openai")
	p.API = "openai-completions"

	// ---------- GPT-5 系列 ----------
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gpt-5",
		Name:          "GPT-5",
		ContextWindow: 400_000,
		MaxTokens:     128_000,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gpt-5.2",
		Name:          "GPT-5.2",
		ContextWindow: 400_000,
		MaxTokens:     128_000,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gpt-5-mini",
		Name:          "GPT-5 Mini",
		ContextWindow: 400_000,
		MaxTokens:     32_768,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gpt-5-nano",
		Name:          "GPT-5 Nano",
		ContextWindow: 200_000,
		MaxTokens:     16_384,
	})
	// ---------- GPT-4.1 系列 ----------
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gpt-4.1",
		Name:          "GPT-4.1",
		ContextWindow: 1_047_576,
		MaxTokens:     32_768,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gpt-4.1-mini",
		Name:          "GPT-4.1 Mini",
		ContextWindow: 1_047_576,
		MaxTokens:     32_768,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gpt-4.1-nano",
		Name:          "GPT-4.1 Nano",
		ContextWindow: 1_047_576,
		MaxTokens:     32_768,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gpt-4o",
		Name:          "GPT-4o",
		ContextWindow: 128_000,
		MaxTokens:     16_384,
	})
	// ---------- 推理系列 ----------
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "o3",
		Name:          "o3",
		Reasoning:     true,
		ContextWindow: 200_000,
		MaxTokens:     100_000,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "o4-mini",
		Name:          "o4-mini",
		Reasoning:     true,
		ContextWindow: 200_000,
		MaxTokens:     100_000,
	})
}

// ApplyOpenAIConfig 注册 OpenAI 并设为默认模型。
func ApplyOpenAIConfig(cfg *types.OpenAcosmiConfig) {
	ApplyOpenAIProviderConfig(cfg)
	setDefaultModel(cfg, OpenAIDefaultModelRef)
}

// ---------- 凭据 ----------

// SetOpenAIApiKey 写入 OpenAI API key。
func SetOpenAIApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "openai", key)
}
