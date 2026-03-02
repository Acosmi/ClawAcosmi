// tools/agents_list_tool.go — Agent 列表工具。
// TS 参考：src/agents/tools/agents-list-tool.ts (97L)
package tools

import (
	"context"

	"github.com/openacosmi/claw-acismi/internal/agents/scope"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// CreateAgentsListTool 创建 agent 列表工具。
func CreateAgentsListTool(cfg *types.OpenAcosmiConfig) *AgentTool {
	return &AgentTool{
		Name:        "agents_list",
		Label:       "List Agents",
		Description: "List all configured agents and their settings.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			if cfg == nil {
				return JsonResult(map[string]any{"agents": []any{}, "count": 0}), nil
			}

			agents := scope.ListAgents(cfg)
			result := make([]map[string]any, 0, len(agents))
			for _, a := range agents {
				entry := map[string]any{
					"id": a.ID,
				}
				if a.Name != "" {
					entry["name"] = a.Name
				}
				if a.Model != nil && a.Model.Primary != "" {
					entry["model"] = a.Model.Primary
				}
				result = append(result, entry)
			}

			return JsonResult(map[string]any{
				"agents": result,
				"count":  len(result),
			}), nil
		},
	}
}
