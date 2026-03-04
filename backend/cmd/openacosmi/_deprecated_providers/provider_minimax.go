package main

// provider_minimax.go — MiniMax provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	MinimaxBaseURL         = "https://api.minimax.io/v1"
	MinimaxAnthropicURL    = "https://api.minimax.io/anthropic"
	MinimaxDefaultModelID  = "MiniMax-M2.5" // M2.5（2026-02-12 发布）
	MinimaxDefaultModelRef = "minimax/MiniMax-M2.5"
)

// ---------- Provider 配置 ----------

// ApplyMinimaxProviderConfig 注册 MiniMax API provider（OpenAI 兼容）。
func ApplyMinimaxProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, MinimaxDefaultModelRef, "MiniMax M2.5")

	p := ensureProvider(cfg, "minimax")
	if p.BaseURL == "" {
		p.BaseURL = MinimaxBaseURL
	}
	p.API = "openai-completions"

	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "MiniMax-M2.5", // 2026-02-12 发布
		Name:          "MiniMax M2.5",
		ContextWindow: 1_000_000,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "MiniMax-M2.1",
		Name:          "MiniMax M2.1",
		ContextWindow: 204_800,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "MiniMax-Text-01",
		Name:          "MiniMax Text-01",
		ContextWindow: 1_000_000,
		MaxTokens:     8_192,
	})
}

// ApplyMinimaxConfig 注册 MiniMax 并设为默认。
func ApplyMinimaxConfig(cfg *types.OpenAcosmiConfig) {
	ApplyMinimaxProviderConfig(cfg)
	setDefaultModel(cfg, MinimaxDefaultModelRef)
}

// ApplyMinimaxPortalProviderConfig 注册 MiniMax Portal provider（Anthropic 兼容，OAuth 授权）。
func ApplyMinimaxPortalProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, "minimax-portal/MiniMax-M2.5", "MiniMax Portal")

	p := ensureProvider(cfg, "minimax-portal")
	if p.BaseURL == "" {
		p.BaseURL = MinimaxAnthropicURL
	}
	p.API = "anthropic-messages"

	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "MiniMax-M2.5",
		Name:          "MiniMax M2.5 (Portal)",
		ContextWindow: 1_000_000,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "MiniMax-Text-01",
		Name:          "MiniMax Text-01 (Portal)",
		ContextWindow: 1_000_000,
		MaxTokens:     8_192,
	})
}

// ---------- 凭据 ----------

// SetMinimaxApiKey 写入 MiniMax API key。
func SetMinimaxApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "minimax", key)
}
