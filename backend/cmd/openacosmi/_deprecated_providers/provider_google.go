package main

// provider_google.go — Google Gemini provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	GeminiBaseURL         = "https://generativelanguage.googleapis.com/v1beta"
	GeminiDefaultModelID  = "gemini-2.5-flash"
	GeminiDefaultModelRef = "google/gemini-2.5-flash"
	// OAuth ClientID 定义在 internal/agents/auth/google_oauth.go (GeminiOAuthClientID)
)

// ---------- Provider 配置 ----------

// ApplyGoogleProviderConfig 注册 Google Gemini provider 及模型列表。
func ApplyGoogleProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, GeminiDefaultModelRef, "Gemini Flash")
	setModelAlias(cfg, "google/gemini-2.5-pro", "Gemini Pro")

	p := ensureProvider(cfg, "google")
	p.API = "google-gemini"

	// ---------- Gemini 3 系列 ----------
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gemini-3.1-pro-preview", // 旗舰（2026-02-26 发布）
		Name:          "Gemini 3.1 Pro Preview",
		ContextWindow: 1_000_000,
		MaxTokens:     65_536, // 官方 64K（ai.google.dev）
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gemini-3-flash-preview", // Gemini 3 快速版（正式可用）
		Name:          "Gemini 3 Flash Preview",
		ContextWindow: 1_000_000,
		MaxTokens:     65_536, // 官方 64K（ai.google.dev）
	})
	// ---------- Stable 2.5 系列 ----------
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gemini-2.5-pro",
		Name:          "Gemini 2.5 Pro",
		ContextWindow: 1_000_000,
		MaxTokens:     65_536, // 官方 64K（ai.google.dev）
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gemini-2.5-flash",
		Name:          "Gemini 2.5 Flash",
		ContextWindow: 1_000_000,
		MaxTokens:     65_536, // 官方 64K（ai.google.dev）
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "gemini-2.5-flash-lite",
		Name:          "Gemini 2.5 Flash Lite",
		ContextWindow: 1_000_000,
		MaxTokens:     65_536, // 官方 64K（ai.google.dev）
	})
}

// ApplyGoogleConfig 注册 Google Gemini 并设为默认模型。
func ApplyGoogleConfig(cfg *types.OpenAcosmiConfig) {
	ApplyGoogleProviderConfig(cfg)
	setDefaultModel(cfg, GeminiDefaultModelRef)
}

// ---------- 凭据 ----------

// SetGeminiApiKey 写入 Google/Gemini API key。
func SetGeminiApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "google", key)
}
