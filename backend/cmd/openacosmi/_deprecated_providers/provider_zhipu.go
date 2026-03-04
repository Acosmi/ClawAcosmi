package main

// provider_zhipu.go — 智谱 GLM (ZAI) provider 配置

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ---------- 模型常量 ----------

const (
	ZhipuBaseURL         = "https://open.bigmodel.cn/api/paas/v4"
	ZhipuDefaultModelID  = "glm-5" // GLM-5（744B MoE，2026-02-11 发布）
	ZhipuDefaultModelRef = "zai/glm-5"
)

// ---------- Provider 配置 ----------

// ApplyZhipuProviderConfig 注册智谱 GLM provider 及模型列表。
func ApplyZhipuProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, ZhipuDefaultModelRef, "GLM")

	p := ensureProvider(cfg, "zai")
	if p.BaseURL == "" {
		p.BaseURL = ZhipuBaseURL
	}
	p.API = "openai-completions"

	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "glm-5", // 2026-02-11 发布，744B MoE，205K ctx
		Name:          "GLM-5",
		ContextWindow: 204_800,
		MaxTokens:     128_000, // 官方 128K（docs.z.ai/guides/llm/glm-5）
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "glm-4.7",
		Name:          "GLM-4.7",
		ContextWindow: 204_800, // 官方 200K（docs.z.ai/guides/llm/glm-4-7，原 128K 错误）
		MaxTokens:     128_000, // 官方 128K（同上，原 4K 错误）
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "glm-4-plus",
		Name:          "GLM-4 Plus",
		ContextWindow: 128_000,
		MaxTokens:     4_096,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "glm-4-flash",
		Name:          "GLM-4 Flash",
		ContextWindow: 128_000,
		MaxTokens:     4_096,
	})
	mergeModelIfMissing(p, types.ModelDefinitionConfig{
		ID:            "glm-4-long",
		Name:          "GLM-4 Long",
		ContextWindow: 1_000_000,
		MaxTokens:     4_096,
	})
}

// ApplyZhipuConfig 注册智谱 GLM 并设为默认。
func ApplyZhipuConfig(cfg *types.OpenAcosmiConfig) {
	ApplyZhipuProviderConfig(cfg)
	setDefaultModel(cfg, ZhipuDefaultModelRef)
}

// ---------- 凭据 ----------

// SetZaiApiKey 写入 ZAI/GLM API key。
func SetZaiApiKey(store *auth.AuthStore, key string) error {
	return upsertApiKeyProfile(store, "zai", key)
}
