package main

// provider_doubao.go — 豆包 / 火山方舟 Ark provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	DoubaoBaseURL         = "https://ark.cn-beijing.volces.com/api/v3"
	DoubaoDefaultModelID  = "doubao-pro-32k"
	DoubaoDefaultModelRef = "doubao/doubao-pro-32k"
)

// ---------- Provider 配置 ----------

// ApplyDoubaoProviderConfig 注册豆包（火山方舟）provider 及模型列表。
func ApplyDoubaoProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, DoubaoDefaultModelRef, "豆包 Pro")
	p := ensureProvider(cfg, "doubao")
	if p.BaseURL == "" {
		p.BaseURL = DoubaoBaseURL
	}
	p.API = "openai-completions"
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "doubao-pro-32k",
		Name:          "豆包 Pro 32K",
		ContextWindow: 32_768,
		MaxTokens:     4_096,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "doubao-pro-128k",
		Name:          "豆包 Pro 128K",
		ContextWindow: 131_072,
		MaxTokens:     4_096,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "doubao-lite-32k",
		Name:          "豆包 Lite 32K",
		ContextWindow: 32_768,
		MaxTokens:     4_096,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "doubao-1-5-pro-32k",
		Name:          "豆包 1.5 Pro 32K",
		ContextWindow: 32_768,
		MaxTokens:     4_096,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "doubao-1-5-thinking-pro-32k",
		Name:          "豆包 1.5 Thinking Pro 32K",
		Reasoning:     true,
		ContextWindow: 32_768,
		MaxTokens:     4_096,
	})
}

// ApplyDoubaoConfig 注册豆包并设为默认。
func ApplyDoubaoConfig(cfg *types.OpenAcosmiConfig) {
	ApplyDoubaoProviderConfig(cfg)
	setDefaultModel(cfg, DoubaoDefaultModelRef)
}

// ---------- 凭据 ----------

// SetDoubaoApiKey 写入豆包/火山方舟 API key。
func SetDoubaoApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "doubao", key)
}
