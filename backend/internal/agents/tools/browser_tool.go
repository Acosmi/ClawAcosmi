// tools/browser_tool.go — 浏览器控制工具。
// TS 参考：src/agents/tools/browser-tool.ts (724L) + browser-tool.schema.ts (112L)
package tools

import (
	"context"
	"fmt"
)

// BrowserController 浏览器控制接口。
type BrowserController interface {
	Navigate(ctx context.Context, url string) error
	GetContent(ctx context.Context) (string, error)
	Click(ctx context.Context, selector string) error
	Type(ctx context.Context, selector, text string) error
	Screenshot(ctx context.Context) ([]byte, string, error)
	Evaluate(ctx context.Context, script string) (any, error)
	WaitForSelector(ctx context.Context, selector string) error
	GoBack(ctx context.Context) error
	GoForward(ctx context.Context) error
	GetURL(ctx context.Context) (string, error)

	// Phase 1: ARIA 快照 + ref 元素交互（接入 pw_role_snapshot / pw_ai_loop 基础设施）
	// SnapshotAI 返回 ARIA 无障碍树快照，包含 ref 标注的可交互元素。
	// 返回 map 包含 "snapshot" (string) 和 "refs" (RoleRefMap)。
	// 如底层不支持可返回 nil, nil。
	SnapshotAI(ctx context.Context) (map[string]any, error)
	// ClickRef 通过 ARIA ref 标识符（如 "e1"）点击元素，比 CSS selector 更健壮。
	ClickRef(ctx context.Context, ref string) error
	// FillRef 通过 ARIA ref 标识符填入文本。
	FillRef(ctx context.Context, ref, text string) error

	// Phase 4: Mariner AI 循环 — 意图级浏览任务。
	// AIBrowse 执行 observe→plan→act 循环，返回 JSON 结果。
	// 需要注入 AIPlanner（通过 SetAIPlanner），否则返回不支持错误。
	AIBrowse(ctx context.Context, goal string) (string, error)
}

// CreateBrowserTool 创建浏览器工具。
// TS 参考: browser-tool.ts
func CreateBrowserTool(controller BrowserController) *AgentTool {
	return &AgentTool{
		Name:        "browser",
		Label:       "Browser",
		Description: "Control a browser: navigate, click, type, screenshot, evaluate JavaScript, and more.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string",
					"enum": []any{
						"navigate", "get_content", "click", "type",
						"screenshot", "evaluate", "wait_for",
						"go_back", "go_forward", "get_url",
					},
					"description": "Browser action to perform",
				},
				"url":      map[string]any{"type": "string", "description": "URL to navigate to"},
				"selector": map[string]any{"type": "string", "description": "CSS selector for click/type/wait"},
				"text":     map[string]any{"type": "string", "description": "Text to type"},
				"script":   map[string]any{"type": "string", "description": "JavaScript to evaluate"},
			},
			"required": []any{"action"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			action, err := ReadStringParam(args, "action", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			if controller == nil {
				return nil, fmt.Errorf("browser controller not configured")
			}

			switch action {
			case "navigate":
				url, err := ReadStringParam(args, "url", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				if err := controller.Navigate(ctx, url); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "navigated", "url": url}), nil

			case "get_content":
				content, err := controller.GetContent(ctx)
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"content": truncateString(content, 50000)}), nil

			case "click":
				selector, err := ReadStringParam(args, "selector", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				if err := controller.Click(ctx, selector); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "clicked", "selector": selector}), nil

			case "type":
				selector, err := ReadStringParam(args, "selector", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				text, err := ReadStringParam(args, "text", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				if err := controller.Type(ctx, selector, text); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "typed", "selector": selector}), nil

			case "screenshot":
				data, mimeType, err := controller.Screenshot(ctx)
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "captured", "mimeType": mimeType, "size": len(data)}), nil

			case "evaluate":
				script, err := ReadStringParam(args, "script", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				result, err := controller.Evaluate(ctx, script)
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"result": result}), nil

			case "wait_for":
				selector, err := ReadStringParam(args, "selector", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				if err := controller.WaitForSelector(ctx, selector); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "found", "selector": selector}), nil

			case "go_back":
				if err := controller.GoBack(ctx); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "navigated_back"}), nil

			case "go_forward":
				if err := controller.GoForward(ctx); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "navigated_forward"}), nil

			case "get_url":
				url, err := controller.GetURL(ctx)
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"url": url}), nil

			default:
				return nil, fmt.Errorf("unknown browser action: %s", action)
			}
		},
	}
}
