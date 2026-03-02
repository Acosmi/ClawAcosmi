package models

// github_copilot_models.go — GitHub Copilot 模型定义
// 对应 TS src/providers/github-copilot-models.ts (42L)
//
// Copilot 模型 ID 因 plan/org 不同可能变化。列表故意宽泛；
// 如果某模型不可用，Copilot 会返回错误，用户可从配置中移除。

import "github.com/openacosmi/claw-acismi/pkg/types"

const (
	copilotDefaultContextWindow = 128_000
	copilotDefaultMaxTokens     = 8192
)

// defaultCopilotModelIDs 默认 Copilot 模型 ID 列表。
var defaultCopilotModelIDs = []string{
	"gpt-4o",
	"gpt-4.1",
	"gpt-4.1-mini",
	"gpt-4.1-nano",
	"o1",
	"o1-mini",
	"o3-mini",
}

// GetDefaultCopilotModelIDs 返回默认 Copilot 模型 ID 列表副本。
// 对应 TS: getDefaultCopilotModelIds()
func GetDefaultCopilotModelIDs() []string {
	result := make([]string, len(defaultCopilotModelIDs))
	copy(result, defaultCopilotModelIDs)
	return result
}

// BuildCopilotModelDefinition 构建 Copilot 模型定义。
// 使用 OpenAI Responses API 兼容格式，provider 保持 "github-copilot"
// 以便附加 Copilot 专属 header。
// 对应 TS: buildCopilotModelDefinition(modelId)
func BuildCopilotModelDefinition(modelID string) types.ModelDefinitionConfig {
	return types.ModelDefinitionConfig{
		ID:            modelID,
		Name:          modelID,
		API:           types.ModelAPIOpenAIResponses,
		Reasoning:     false,
		Input:         []types.ModelInputType{types.ModelInputText, types.ModelInputImage},
		Cost:          types.ModelCostConfig{Input: 0, Output: 0, CacheRead: 0, CacheWrite: 0},
		ContextWindow: copilotDefaultContextWindow,
		MaxTokens:     copilotDefaultMaxTokens,
	}
}

// BuildDefaultCopilotModels 构建全部默认 Copilot 模型定义。
func BuildDefaultCopilotModels() []types.ModelDefinitionConfig {
	ids := GetDefaultCopilotModelIDs()
	models := make([]types.ModelDefinitionConfig, 0, len(ids))
	for _, id := range ids {
		models = append(models, BuildCopilotModelDefinition(id))
	}
	return models
}
