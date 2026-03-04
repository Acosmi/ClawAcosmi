// providers/copilot/models.go — GitHub Copilot 模型定义
// 对应 TS 文件: src/providers/github-copilot-models.ts
// 包含 Copilot 默认模型列表和模型定义构建函数。
package copilot

import (
	"fmt"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// 默认上下文窗口大小。
const defaultContextWindow = 128000

// 默认最大输出 token 数。
const defaultMaxTokens = 8192

// defaultModelIDs Copilot 默认模型 ID 列表。
// Copilot 的模型可用性因套餐/组织而异，此列表刻意保持宽泛。
// 若模型不可用，Copilot 会返回错误，用户可从配置中移除。
var defaultModelIDs = []string{
	"claude-sonnet-4.6",
	"claude-sonnet-4.5",
	"gpt-4o",
	"gpt-4.1",
	"gpt-4.1-mini",
	"gpt-4.1-nano",
	"o1",
	"o1-mini",
	"o3-mini",
}

// GetDefaultCopilotModelIDs 返回 Copilot 默认模型 ID 列表的副本。
// 对应 TS: getDefaultCopilotModelIds()
func GetDefaultCopilotModelIDs() []string {
	result := make([]string, len(defaultModelIDs))
	copy(result, defaultModelIDs)
	return result
}

// BuildCopilotModelDefinition 根据模型 ID 构建 Copilot 模型定义配置。
// pi-coding-agent 的注册表不感知 "github-copilot" API，
// 使用 OpenAI 兼容的 responses API，但 provider id 保持 "github-copilot"。
// 对应 TS: buildCopilotModelDefinition()
func BuildCopilotModelDefinition(modelID string) (types.ModelDefinitionConfig, error) {
	if len(modelID) == 0 {
		return types.ModelDefinitionConfig{}, fmt.Errorf("模型 ID 不能为空")
	}
	return types.ModelDefinitionConfig{
		ID:        modelID,
		Name:      modelID,
		Api:       types.ModelApiOpenAIResponses,
		Reasoning: false,
		Input:     []string{"text", "image"},
		Cost: types.ModelCost{
			Input:      0,
			Output:     0,
			CacheRead:  0,
			CacheWrite: 0,
		},
		ContextWindow: defaultContextWindow,
		MaxTokens:     defaultMaxTokens,
	}, nil
}
