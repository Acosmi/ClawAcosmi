// tools/canvas_tool.go — 画布工具。
// TS 参考：src/agents/tools/canvas-tool.ts (180L)
package tools

import (
	"context"
	"fmt"
)

// CanvasProvider 画布操作接口。
type CanvasProvider interface {
	RenderHTML(ctx context.Context, html string) (string, error)
	Screenshot(ctx context.Context, url string) ([]byte, error)
}

// CreateCanvasTool 创建画布工具。
func CreateCanvasTool(provider CanvasProvider) *AgentTool {
	return &AgentTool{
		Name:        "canvas",
		Label:       "Canvas",
		Description: "Render HTML content or take screenshots of web pages.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string", "enum": []any{"render", "screenshot"},
					"description": "Canvas action",
				},
				"html": map[string]any{
					"type": "string", "description": "HTML content to render",
				},
				"url": map[string]any{
					"type": "string", "description": "URL to screenshot",
				},
			},
			"required": []any{"action"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			action, err := ReadStringParam(args, "action", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			if provider == nil {
				return nil, fmt.Errorf("canvas provider not configured")
			}

			switch action {
			case "render":
				html, err := ReadStringParam(args, "html", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				result, err := provider.RenderHTML(ctx, html)
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "rendered", "result": result}), nil
			case "screenshot":
				url, err := ReadStringParam(args, "url", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				data, err := provider.Screenshot(ctx, url)
				if err != nil {
					return nil, err
				}
				_ = data
				return JsonResult(map[string]any{"status": "captured", "url": url}), nil
			default:
				return nil, fmt.Errorf("unknown canvas action: %s", action)
			}
		},
	}
}
