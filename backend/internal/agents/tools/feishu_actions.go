// tools/feishu_actions.go — 飞书频道动作工具包装器。
// 继承自 telegram_actions.go 相同模式。
// 实际逻辑在 internal/channels/bridge/feishu_actions.go 中实现。
package tools

import (
	"context"

	"github.com/Acosmi/ClawAcosmi/internal/channels/bridge"
)

// FeishuActionsToolDeps 飞书工具所需的外部依赖。
type FeishuActionsToolDeps struct {
	BridgeDeps bridge.FeishuActionDeps
	// Actions 配置值（来自 config.channels.feishu.actions）。
	Actions map[string]any
}

// FeishuActionsTool 创建 feishu_action AgentTool 定义。
func FeishuActionsTool(ctx context.Context, deps *FeishuActionsToolDeps) *AgentTool {
	if deps == nil {
		return nil
	}
	return &AgentTool{
		Label:       "Feishu",
		Name:        "feishu_action",
		Description: "Perform Feishu (Lark) actions such as sending messages to users or group chats.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "The Feishu action to perform.",
				},
			},
			"required":             []string{"action"},
			"additionalProperties": true,
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			actionGate := bridge.CreateActionGate(deps.Actions)
			result, err := bridge.HandleFeishuAction(ctx, args, actionGate, deps.BridgeDeps)
			if err != nil {
				return errorToolResult(err.Error()), nil
			}
			return bridgeResultToAgentResult(result), nil
		},
	}
}
