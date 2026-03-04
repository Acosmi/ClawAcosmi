package main

// provider_anthropic.go — Anthropic Claude provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	AnthropicBaseURL         = "https://api.anthropic.com/v1"
	AnthropicDefaultModelID  = "claude-opus-4-6"
	AnthropicDefaultModelRef = "anthropic/claude-opus-4-6"
)

// ---------- Provider 配置 ----------

// ApplyAnthropicProviderConfig 注册 Anthropic provider 及模型列表。
func ApplyAnthropicProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, AnthropicDefaultModelRef, "Claude Opus")
	setModelAlias(cfg, "anthropic/claude-sonnet-4-6", "Claude Sonnet")
	setModelAlias(cfg, "anthropic/claude-haiku-4-5", "Claude Haiku")

	p := ensureProvider(cfg, "anthropic")
	p.API = "anthropic-messages"

	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "claude-opus-4-6",
		Name:          "Claude Opus 4.6",
		ContextWindow: 200_000,
		MaxTokens:     128_000, // 官方 128K（2026-02-05 Anthropic 文档）
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "claude-sonnet-4-6",
		Name:          "Claude Sonnet 4.6",
		ContextWindow: 200_000,
		MaxTokens:     64_000, // 官方 64K（2026-02-05 Anthropic 文档）
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "claude-haiku-4-5",
		Name:          "Claude Haiku 4.5",
		ContextWindow: 200_000,
		MaxTokens:     64_000, // 官方 64K（2026 Anthropic 文档，原 8K 错误）
	})
}

// ApplyAnthropicConfig 注册并设为默认模型。
func ApplyAnthropicConfig(cfg *types.OpenAcosmiConfig) {
	ApplyAnthropicProviderConfig(cfg)
	setDefaultModel(cfg, AnthropicDefaultModelRef)
}

// ---------- 凭据 ----------

// SetAnthropicApiKey 写入 Anthropic API key。
func SetAnthropicApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "anthropic", key)
}
