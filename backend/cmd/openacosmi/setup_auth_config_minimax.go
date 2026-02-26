package main

// setup_auth_config_minimax.go — Minimax 三模式 Provider 配置
// TS 对照: src/commands/onboard-auth.config-minimax.ts (216L)
//
// 提供 Minimax 的三种部署模式配置：
//   1. LM Studio 本地部署 (applyMinimaxProviderConfig)
//   2. Minimax 官方 OpenAI 兼容 API (applyMinimaxHostedProviderConfig)
//   3. Minimax Anthropic 兼容 API (applyMinimaxApiProviderConfig)

import "github.com/anthropic/open-acosmi/pkg/types"

// ---------- Mode 1: LM Studio 本地部署 ----------

// ApplyMinimaxProviderConfig 注册 LM Studio + Minimax 本地部署模式。
// 对应 TS: applyMinimaxProviderConfig (config-minimax.ts L15-59)。
func ApplyMinimaxProviderConfig(cfg *types.OpenAcosmiConfig) {
	setModelAlias(cfg, "anthropic/claude-opus-4-6", "Opus")
	setModelAlias(cfg, minimaxLMStudioModelRef, "Minimax")

	p := ensureProvider(cfg, "lmstudio")
	if p.BaseURL == "" {
		p.BaseURL = "http://127.0.0.1:1234/v1"
		p.APIKey = "lmstudio"
		p.API = "openai-responses"
	}
	mergeModelIfMissing(p, buildMinimaxModelDef(
		minimaxLMStudioModelID, "MiniMax M2.1 GS32",
		minimaxLMStudioCost, 196608, 8192,
	))
}

// ApplyMinimaxConfig Mode 1 + 设为默认模型。
// 对应 TS: applyMinimaxConfig (config-minimax.ts L106-126)。
func ApplyMinimaxConfig(cfg *types.OpenAcosmiConfig) {
	ApplyMinimaxProviderConfig(cfg)
	setDefaultModel(cfg, minimaxLMStudioModelRef)
}

// ---------- Mode 2: Minimax 官方 OpenAI 兼容 API ----------

// ApplyMinimaxHostedProviderConfig 注册 Minimax 官方 OpenAI 兼容 API。
// 对应 TS: applyMinimaxHostedProviderConfig (config-minimax.ts L61-104)。
func ApplyMinimaxHostedProviderConfig(cfg *types.OpenAcosmiConfig, baseURL string) {
	setModelAlias(cfg, minimaxHostedModelRef, "Minimax")

	hostedModel := buildMinimaxModelDef(
		minimaxHostedModelID, "",
		minimaxHostedCost,
		minimaxDefaultContextWindow,
		minimaxDefaultMaxTokens,
	)

	p := ensureProvider(cfg, "minimax")
	mergeModelIfMissing(p, hostedModel)

	resolvedURL := baseURL
	if resolvedURL == "" {
		resolvedURL = minimaxDefaultBaseURL
	}
	p.BaseURL = resolvedURL
	p.APIKey = "minimax"
	p.API = "openai-completions"
}

// ApplyMinimaxHostedConfig Mode 2 + 设为默认模型。
// 对应 TS: applyMinimaxHostedConfig (config-minimax.ts L128-146)。
func ApplyMinimaxHostedConfig(cfg *types.OpenAcosmiConfig, baseURL string) {
	ApplyMinimaxHostedProviderConfig(cfg, baseURL)
	setDefaultModel(cfg, minimaxHostedModelRef)
}

// ---------- Mode 3: Minimax Anthropic 兼容 API ----------

// ApplyMinimaxApiProviderConfig 注册 Minimax Anthropic 兼容 API。
// 对应 TS: applyMinimaxApiProviderConfig (config-minimax.ts L148-190)。
func ApplyMinimaxApiProviderConfig(cfg *types.OpenAcosmiConfig, modelID string) {
	if modelID == "" {
		modelID = minimaxHostedModelID
	}
	apiModel := buildMinimaxAPIModelDef(modelID)
	modelRef := "minimax/" + modelID

	p := ensureProvider(cfg, "minimax")
	mergeModelIfMissing(p, apiModel)

	p.BaseURL = minimaxAnthropicBaseURL
	p.API = "anthropic-messages"
	// 保留已有 apiKey，但清除默认 "minimax" 占位值
	if p.APIKey == "minimax" {
		p.APIKey = ""
	}

	setModelAlias(cfg, modelRef, "Minimax")
}

// ApplyMinimaxApiConfig Mode 3 + 设为默认模型。
// 对应 TS: applyMinimaxApiConfig (config-minimax.ts L192-215)。
func ApplyMinimaxApiConfig(cfg *types.OpenAcosmiConfig, modelID string) {
	if modelID == "" {
		modelID = minimaxHostedModelID
	}
	ApplyMinimaxApiProviderConfig(cfg, modelID)
	setDefaultModel(cfg, "minimax/"+modelID)
}
