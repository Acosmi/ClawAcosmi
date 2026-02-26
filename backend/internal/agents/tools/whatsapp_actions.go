// tools/whatsapp_actions.go — WhatsApp 频道动作工具包装器。
// TS 参考：src/agents/tools/whatsapp-actions.ts (40L)
// 实际逻辑在 internal/channels/bridge/whatsapp_actions.go 中实现。
package tools

import (
	"context"

	"github.com/anthropic/open-acosmi/internal/channels/bridge"
)

// WhatsAppActionsToolDeps WhatsApp 工具所需的外部依赖。
type WhatsAppActionsToolDeps struct {
	// Actions 配置值（来自 config.channels.whatsapp.actions）。
	Actions map[string]any
}

// WhatsAppActionsTool 创建 whatsapp_action AgentTool 定义。
func WhatsAppActionsTool(deps *WhatsAppActionsToolDeps) *AgentTool {
	if deps == nil {
		return nil
	}
	return &AgentTool{
		Label:       "WhatsApp",
		Name:        "whatsapp_action",
		Description: "Perform WhatsApp actions such as sending reactions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "The WhatsApp action to perform.",
				},
			},
			"required":             []string{"action"},
			"additionalProperties": true,
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			actionGate := bridge.CreateActionGate(deps.Actions)
			result, err := bridge.HandleWhatsAppAction(args, actionGate)
			if err != nil {
				return errorToolResult(err.Error()), nil
			}
			return bridgeResultToAgentResult(result), nil
		},
	}
}
