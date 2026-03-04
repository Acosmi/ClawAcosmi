package main

// provider_kimi.go — Moonshot / Kimi provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	KimiBaseURL         = "https://api.moonshot.ai/v1"
	KimiCnBaseURL       = "https://api.moonshot.cn/v1"
	KimiDefaultModelID  = "moonshot-v1-kimi-k2" // K2.5 稳定别名，替代快照 kimi-k2-0905-preview
	KimiDefaultModelRef = "moonshot/moonshot-v1-kimi-k2"
)

// ---------- Provider 配置 ----------

// ApplyKimiProviderConfig 注册 Moonshot/Kimi provider（国际线路）。
func ApplyKimiProviderConfig(cfg *types.OpenAcosmiConfig) {
	applyKimiWithBaseURL(cfg, KimiBaseURL)
}

// ApplyKimiProviderConfigCn 注册 Moonshot/Kimi provider（中国区线路）。
func ApplyKimiProviderConfigCn(cfg *types.OpenAcosmiConfig) {
	applyKimiWithBaseURL(cfg, KimiCnBaseURL)
}

func applyKimiWithBaseURL(cfg *types.OpenAcosmiConfig, baseURL string) {
	setModelAlias(cfg, KimiDefaultModelRef, "Kimi")
	p := ensureProvider(cfg, "moonshot")
	if p.BaseURL == "" {
		p.BaseURL = baseURL
	}
	p.API = "openai-completions"
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "moonshot-v1-kimi-k2", // K2.5 正式生产别名（2026-01-27 发布）
		Name:          "Kimi K2.5",
		ContextWindow: 131_072,
		MaxTokens:     16_384,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "moonshot-v1-128k",
		Name:          "Moonshot V1 128K",
		ContextWindow: 128_000,
		MaxTokens:     8_192,
	})
}

// ApplyKimiConfig 注册 Kimi 并设为默认。
func ApplyKimiConfig(cfg *types.OpenAcosmiConfig) {
	ApplyKimiProviderConfig(cfg)
	setDefaultModel(cfg, KimiDefaultModelRef)
}

// ApplyKimiConfigCn 注册中国区 Kimi 并设为默认。
func ApplyKimiConfigCn(cfg *types.OpenAcosmiConfig) {
	ApplyKimiProviderConfigCn(cfg)
	setDefaultModel(cfg, KimiDefaultModelRef)
}

// ---------- 凭据 ----------

// SetMoonshotApiKey 写入 Moonshot API key。
func SetMoonshotApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "moonshot", key)
}
