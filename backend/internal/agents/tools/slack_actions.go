// tools/slack_actions.go — Slack 频道动作工具包装器。
// TS 参考：src/agents/tools/slack-actions.ts (313L)
// 实际逻辑在 internal/channels/bridge/slack_actions.go 中实现。
package tools

import (
	"context"

	"github.com/openacosmi/claw-acismi/internal/channels/bridge"
)

// SlackActionsToolDeps Slack 工具所需的外部依赖。
type SlackActionsToolDeps struct {
	BridgeDeps bridge.SlackActionDeps
	// Actions 配置值（来自 config.channels.slack.actions）。
	Actions map[string]any
	// SlackContext thread 上下文管理（可空）。
	SlackContext *bridge.SlackActionContext
}

// SlackActionsTool 创建 slack_action AgentTool 定义。
func SlackActionsTool(ctx context.Context, deps *SlackActionsToolDeps) *AgentTool {
	if deps == nil {
		return nil
	}
	return &AgentTool{
		Label:       "Slack",
		Name:        "slack_action",
		Description: "Perform Slack actions such as sending messages, managing threads, reactions, and pins.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "The Slack action to perform.",
				},
			},
			"required":             []string{"action"},
			"additionalProperties": true,
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			actionGate := bridge.CreateActionGate(deps.Actions)
			result, err := bridge.HandleSlackAction(ctx, args, actionGate, deps.SlackContext, deps.BridgeDeps)
			if err != nil {
				return errorToolResult(err.Error()), nil
			}
			return bridgeResultToAgentResult(result), nil
		},
	}
}
