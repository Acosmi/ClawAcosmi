package main

// provider_qwen.go — Qwen 通义千问 provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	QwenBaseURLIntl   = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"
	QwenBaseURLCN     = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	QwenPortalBaseURL = "https://portal.qwen.ai/v1"

	QwenDefaultModelID  = "qwen3.5-plus" // Qwen3.5（1M ctx，2026-02-16 发布）
	QwenDefaultModelRef = "qwen/qwen3.5-plus"
)

// ---------- Provider 配置 ----------

// ApplyQwenProviderConfig 注册 Qwen DashScope provider（国际线路）。
func ApplyQwenProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, QwenDefaultModelRef, "Qwen")

	p := ensureProvider(cfg, "qwen")
	if p.BaseURL == "" {
		p.BaseURL = QwenBaseURLIntl
	}
	p.API = "openai-completions"

	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "qwen3.5-plus", // 2026-02-16 发布，1M ctx
		Name:          "Qwen3.5 Plus",
		ContextWindow: 1_000_000,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "qwen3-max",
		Name:          "Qwen3 Max",
		ContextWindow: 262_144, // 官方 262K（阿里云 Model Studio 2026）
		MaxTokens:     65_536,  // 官方 64K（alibabacloud.com Model Studio 文档）
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "qwen-max",
		Name:          "Qwen Max",
		ContextWindow: 131_072,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "qwen-turbo",
		Name:          "Qwen Turbo",
		ContextWindow: 1_000_000,
		MaxTokens:     8_192,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "qwen-flash",
		Name:          "Qwen Flash",
		ContextWindow: 1_000_000,
		MaxTokens:     8_192,
	})
}

// ApplyQwenConfig 注册 Qwen 并设为默认。
func ApplyQwenConfig(cfg *types.OpenAcosmiConfig) {
	ApplyQwenProviderConfig(cfg)
	setDefaultModel(cfg, QwenDefaultModelRef)
}

// ---------- 凭据 ----------

// SetQwenApiKey 写入 Qwen DashScope API key。
func SetQwenApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "qwen", key)
}
