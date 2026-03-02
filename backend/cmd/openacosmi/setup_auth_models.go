package main

// setup_auth_models.go — Minimax 模型常量和定义构建器
// TS 对照: src/commands/onboard-auth.models.ts (123L)
//
// 提供 Minimax 三模式配置所需的 URL/成本/模型定义构建函数。
// 注：Moonshot/xAI/Qianfan 常量已在 setup_auth_config.go 中定义（私有），
// 此文件仅补充 Minimax 特有的常量和构建器。

import "github.com/openacosmi/claw-acismi/pkg/types"

// ---------- Minimax 三模式常量 ----------

const (
	// minimaxDefaultBaseURL Minimax OpenAI 兼容 API 端点。
	minimaxDefaultBaseURL = "https://api.minimax.io/v1"
	// minimaxAnthropicBaseURL Minimax Anthropic 兼容 API 端点。
	minimaxAnthropicBaseURL = "https://api.minimax.io/anthropic"
	// minimaxHostedModelID Minimax 托管模型 ID。
	minimaxHostedModelID = "MiniMax-M2.1"
	// minimaxHostedModelRef Minimax 托管模型引用。
	minimaxHostedModelRef = "minimax/" + minimaxHostedModelID
	// minimaxDefaultContextWindow 默认上下文窗口。
	minimaxDefaultContextWindow = 200000
	// minimaxDefaultMaxTokens 默认最大输出 token。
	minimaxDefaultMaxTokens = 8192
	// minimaxLMStudioModelID LM Studio 本地部署模型 ID。
	minimaxLMStudioModelID = "minimax-m2.1-gs32"
	// minimaxLMStudioModelRef LM Studio 本地模型引用。
	minimaxLMStudioModelRef = "lmstudio/" + minimaxLMStudioModelID
)

var minimaxAPICost = types.ModelCostConfig{
	Input: 15, Output: 60, CacheRead: 2, CacheWrite: 10,
}

var minimaxHostedCost = types.ModelCostConfig{
	Input: 0, Output: 0, CacheRead: 0, CacheWrite: 0,
}

var minimaxLMStudioCost = types.ModelCostConfig{
	Input: 0, Output: 0, CacheRead: 0, CacheWrite: 0,
}

// ---------- OpenAcosmi Zen ----------

const (
	// acosmiZenDefaultModelRef OpenAcosmi Zen 默认模型引用。
	acosmiZenDefaultModelRef = "openacosmi-zen/claude-opus"
)

// ---------- 模型定义构建器 ----------

// minimaxModelCatalog Minimax 模型目录。
var minimaxModelCatalog = map[string]string{
	"MiniMax-M2.1":           "MiniMax M2.1",
	"MiniMax-M2.1-lightning": "MiniMax M2.1 Lightning",
}

// buildMinimaxModelDef 构建 Minimax 模型定义。
// 对应 TS: buildMinimaxModelDefinition (onboard-auth.models.ts L59-77)。
func buildMinimaxModelDef(id, name string, cost types.ModelCostConfig, contextWindow, maxTokens int) types.ModelDefinitionConfig {
	if name == "" {
		if n, ok := minimaxModelCatalog[id]; ok {
			name = n
		} else {
			name = "MiniMax " + id
		}
	}
	return types.ModelDefinitionConfig{
		ID:            id,
		Name:          name,
		Reasoning:     false,
		Input:         []types.ModelInputType{types.ModelInputText},
		Cost:          cost,
		ContextWindow: contextWindow,
		MaxTokens:     maxTokens,
	}
}

// buildMinimaxAPIModelDef 构建 Minimax API 模型定义。
// 对应 TS: buildMinimaxApiModelDefinition (onboard-auth.models.ts L79-86)。
func buildMinimaxAPIModelDef(modelID string) types.ModelDefinitionConfig {
	return buildMinimaxModelDef(
		modelID, "",
		minimaxAPICost,
		minimaxDefaultContextWindow,
		minimaxDefaultMaxTokens,
	)
}
