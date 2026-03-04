// tools/discord_actions.go — Discord 频道动作工具包装器。
// TS 参考：src/agents/tools/discord-actions.ts (77L)
// 实际逻辑在 internal/channels/bridge/discord_actions*.go 中实现，
// 本文件仅将 bridge 层暴露为 AgentTool。
package tools

import (
	"context"
	"encoding/json"

	"github.com/Acosmi/ClawAcosmi/internal/channels/bridge"
)

// DiscordActionsToolDeps Discord 工具所需的外部依赖。
// 调用方在构造时注入 bridge.DiscordActionDeps 实现。
type DiscordActionsToolDeps struct {
	BridgeDeps bridge.DiscordActionDeps
	// Actions 配置值（来自 config.channels.discord.actions）。
	// nil 表示全部允许。
	Actions map[string]any
}

// DiscordActionsTool 创建 discord_action AgentTool 定义。
// TS 参考: src/agents/channel-tools.ts → Discord plugin
func DiscordActionsTool(ctx context.Context, deps *DiscordActionsToolDeps) *AgentTool {
	if deps == nil {
		return nil
	}
	return &AgentTool{
		Label:       "Discord",
		Name:        "discord_action",
		Description: "Perform Discord actions such as sending messages, managing channels, moderation, and setting presence.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "The Discord action to perform.",
				},
			},
			"required":             []string{"action"},
			"additionalProperties": true,
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			actionGate := bridge.CreateActionGate(deps.Actions)
			result, err := bridge.HandleDiscordAction(ctx, args, actionGate, deps.BridgeDeps)
			if err != nil {
				return errorToolResult(err.Error()), nil
			}
			return bridgeResultToAgentResult(result), nil
		},
	}
}

// ---------- bridge.ToolResult → AgentToolResult 变换 ----------

// bridgeResultToAgentResult 将 bridge.ToolResult 转换为 AgentToolResult。
func bridgeResultToAgentResult(br bridge.ToolResult) *AgentToolResult {
	if br.IsError {
		return errorToolResult(br.Error)
	}
	text := ""
	if br.Data != nil {
		switch v := br.Data.(type) {
		case string:
			text = v
		default:
			b, err := json.Marshal(v)
			if err != nil {
				text = "ok"
			} else {
				text = string(b)
			}
		}
	}
	return &AgentToolResult{
		Content: []ContentBlock{{Type: "text", Text: text}},
	}
}

// errorToolResult 构造错误结果。
func errorToolResult(msg string) *AgentToolResult {
	return &AgentToolResult{
		Content: []ContentBlock{{Type: "text", Text: "Error: " + msg}},
	}
}
