// tools/telegram_actions.go — Telegram 频道动作工具包装器。
// TS 参考：src/agents/tools/telegram-actions.ts (325L)
// 实际逻辑在 internal/channels/bridge/telegram_actions.go 中实现。
package tools

import (
	"context"

	"github.com/Acosmi/ClawAcosmi/internal/channels/bridge"
)

// TelegramActionsToolDeps Telegram 工具所需的外部依赖。
type TelegramActionsToolDeps struct {
	BridgeDeps bridge.TelegramActionDeps
	// Actions 配置值（来自 config.channels.telegram.actions）。
	Actions map[string]any
}

// TelegramActionsTool 创建 telegram_action AgentTool 定义。
func TelegramActionsTool(ctx context.Context, deps *TelegramActionsToolDeps) *AgentTool {
	if deps == nil {
		return nil
	}
	return &AgentTool{
		Label:       "Telegram",
		Name:        "telegram_action",
		Description: "Perform Telegram actions such as sending messages, editing, deleting, reactions, stickers, and admin operations.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "The Telegram action to perform.",
				},
			},
			"required":             []string{"action"},
			"additionalProperties": true,
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			actionGate := bridge.CreateActionGate(deps.Actions)
			result, err := bridge.HandleTelegramAction(ctx, args, actionGate, deps.BridgeDeps)
			if err != nil {
				return errorToolResult(err.Error()), nil
			}
			return bridgeResultToAgentResult(result), nil
		},
	}
}
