package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"Argus-compound/go-sensory/internal/input"
)

// ──────────────────────────────────────────────────────────────
// Shared dependencies for action tools
// ──────────────────────────────────────────────────────────────

// ActionDeps bundles the shared dependencies that all action tools need.
//
// Architecture note: unlike perception tools (which are auto-approved),
// action tools route through ApprovalGateway for human-in-the-loop
// confirmation.  This is critical for MCP — it's an external agent
// protocol, so every write operation must be approved.
type ActionDeps struct {
	Input   input.InputController
	Gateway *input.ApprovalGateway
}

// RegisterActionTools batch-registers all action MCP tools.
func RegisterActionTools(r *Registry, deps ActionDeps) {
	r.Register(&ClickTool{deps: deps})
	r.Register(&DoubleClickTool{deps: deps})
	r.Register(&TypeTextTool{deps: deps})
	r.Register(&PressKeyTool{deps: deps})
	r.Register(&HotkeyTool{deps: deps})
	r.Register(&ScrollTool{deps: deps})
	r.Register(&MousePositionTool{deps: deps})
}

// approveAction is the shared approval gate for all action tools.
// Returns (approved, modifiedParams, error).
func approveAction(ctx context.Context, gw *input.ApprovalGateway, action string, params json.RawMessage) (bool, json.RawMessage, error) {
	if gw == nil {
		// No gateway → allow (non-MCP path or disabled).
		return true, nil, nil
	}
	return gw.CheckAndApprove(ctx, action, params, "mcp", nil)
}

// ══════════════════════════════════════════════════════════════
// Tool 1: click
// ══════════════════════════════════════════════════════════════

type ClickTool struct{ deps ActionDeps }

func (t *ClickTool) Name() string           { return "click" }
func (t *ClickTool) Description() string    { return "在指定坐标执行鼠标点击" }
func (t *ClickTool) Category() ToolCategory { return CategoryAction }
func (t *ClickTool) Risk() RiskLevel        { return RiskMedium }
func (t *ClickTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"x":      {Type: "integer", Description: "X coordinate"},
			"y":      {Type: "integer", Description: "Y coordinate"},
			"button": {Type: "integer", Description: "Mouse button: 0=left, 1=right, 2=middle", Default: 0},
		},
		Required: []string{"x", "y"},
	}
}

func (t *ClickTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		X      int `json:"x"`
		Y      int `json:"y"`
		Button int `json:"button"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{IsError: true, Error: "invalid params: " + err.Error()}, nil
	}

	approved, _, err := approveAction(ctx, t.deps.Gateway, "click", params)
	if err != nil {
		return nil, err
	}
	if !approved {
		return &ToolResult{IsError: true, Error: "操作被人类审核员拒绝"}, nil
	}

	if err := t.deps.Input.Click(p.X, p.Y, input.MouseButton(p.Button)); err != nil {
		return nil, fmt.Errorf("click failed: %w", err)
	}

	return &ToolResult{
		Content: map[string]any{"clicked": true, "x": p.X, "y": p.Y, "button": p.Button},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 2: double_click
// ══════════════════════════════════════════════════════════════

type DoubleClickTool struct{ deps ActionDeps }

func (t *DoubleClickTool) Name() string           { return "double_click" }
func (t *DoubleClickTool) Description() string    { return "在指定坐标执行鼠标双击" }
func (t *DoubleClickTool) Category() ToolCategory { return CategoryAction }
func (t *DoubleClickTool) Risk() RiskLevel        { return RiskMedium }
func (t *DoubleClickTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"x": {Type: "integer", Description: "X coordinate"},
			"y": {Type: "integer", Description: "Y coordinate"},
		},
		Required: []string{"x", "y"},
	}
}

func (t *DoubleClickTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{IsError: true, Error: "invalid params: " + err.Error()}, nil
	}

	approved, _, err := approveAction(ctx, t.deps.Gateway, "click", params)
	if err != nil {
		return nil, err
	}
	if !approved {
		return &ToolResult{IsError: true, Error: "操作被人类审核员拒绝"}, nil
	}

	if err := t.deps.Input.DoubleClick(p.X, p.Y); err != nil {
		return nil, fmt.Errorf("double_click failed: %w", err)
	}

	return &ToolResult{
		Content: map[string]any{"double_clicked": true, "x": p.X, "y": p.Y},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 3: type_text
// ══════════════════════════════════════════════════════════════

type TypeTextTool struct{ deps ActionDeps }

func (t *TypeTextTool) Name() string { return "type_text" }
func (t *TypeTextTool) Description() string {
	return "模拟键盘输入文本（可能动态升级风险到 High）"
}
func (t *TypeTextTool) Category() ToolCategory { return CategoryAction }
func (t *TypeTextTool) Risk() RiskLevel        { return RiskMedium }
func (t *TypeTextTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"text": {Type: "string", Description: "Text to type"},
		},
		Required: []string{"text"},
	}
}

func (t *TypeTextTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Text == "" {
		return &ToolResult{IsError: true, Error: "text is required"}, nil
	}

	// Gateway uses "type_text" action name for risk classification
	// (dynamic escalation for "sudo", "rm -rf", etc.)
	approved, modifiedParams, err := approveAction(ctx, t.deps.Gateway, "type_text", params)
	if err != nil {
		return nil, err
	}
	if !approved {
		return &ToolResult{IsError: true, Error: "操作被人类审核员拒绝"}, nil
	}

	// Human may have edited the text
	actual := p.Text
	if modifiedParams != nil {
		var mp struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(modifiedParams, &mp) == nil && mp.Text != "" {
			actual = mp.Text
		}
	}

	if err := t.deps.Input.Type(actual); err != nil {
		return nil, fmt.Errorf("type_text failed: %w", err)
	}

	return &ToolResult{
		Content: map[string]any{"typed": true, "length": len(actual)},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 4: press_key
// ══════════════════════════════════════════════════════════════

type PressKeyTool struct{ deps ActionDeps }

func (t *PressKeyTool) Name() string           { return "press_key" }
func (t *PressKeyTool) Description() string    { return "模拟按下并释放单个按键" }
func (t *PressKeyTool) Category() ToolCategory { return CategoryAction }
func (t *PressKeyTool) Risk() RiskLevel        { return RiskMedium }
func (t *PressKeyTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"key": {Type: "integer", Description: "macOS CGKeyCode value"},
		},
		Required: []string{"key"},
	}
}

func (t *PressKeyTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Key uint16 `json:"key"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{IsError: true, Error: "invalid params: " + err.Error()}, nil
	}

	// Use press_key action name for risk scoring
	approved, _, err := approveAction(ctx, t.deps.Gateway, "press_key", params)
	if err != nil {
		return nil, err
	}
	if !approved {
		return &ToolResult{IsError: true, Error: "操作被人类审核员拒绝"}, nil
	}

	if err := t.deps.Input.KeyPress(input.Key(p.Key)); err != nil {
		return nil, fmt.Errorf("press_key failed: %w", err)
	}

	return &ToolResult{
		Content: map[string]any{"pressed": true, "key": p.Key},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 5: hotkey
// ══════════════════════════════════════════════════════════════

type HotkeyTool struct{ deps ActionDeps }

func (t *HotkeyTool) Name() string           { return "hotkey" }
func (t *HotkeyTool) Description() string    { return "模拟组合键（如 ⌘C、Ctrl+Z）" }
func (t *HotkeyTool) Category() ToolCategory { return CategoryAction }
func (t *HotkeyTool) Risk() RiskLevel        { return RiskMedium }
func (t *HotkeyTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"keys": {Type: "array", Description: "Array of macOS CGKeyCode values to press simultaneously"},
		},
		Required: []string{"keys"},
	}
}

func (t *HotkeyTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Keys []uint16 `json:"keys"`
	}
	if err := json.Unmarshal(params, &p); err != nil || len(p.Keys) == 0 {
		return &ToolResult{IsError: true, Error: "keys array is required"}, nil
	}

	// Gateway handles dynamic escalation (Ctrl+C → High, Cmd+Q → blocked)
	approved, _, err := approveAction(ctx, t.deps.Gateway, "hotkey", params)
	if err != nil {
		return nil, err
	}
	if !approved {
		return &ToolResult{IsError: true, Error: "操作被人类审核员拒绝"}, nil
	}

	keys := make([]input.Key, len(p.Keys))
	for i, k := range p.Keys {
		keys[i] = input.Key(k)
	}

	if err := t.deps.Input.Hotkey(keys...); err != nil {
		return nil, fmt.Errorf("hotkey failed: %w", err)
	}

	return &ToolResult{
		Content: map[string]any{"pressed": true, "keys": p.Keys},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 6: scroll
// ══════════════════════════════════════════════════════════════

type ScrollTool struct{ deps ActionDeps }

func (t *ScrollTool) Name() string           { return "scroll" }
func (t *ScrollTool) Description() string    { return "在指定位置执行鼠标滚动" }
func (t *ScrollTool) Category() ToolCategory { return CategoryAction }
func (t *ScrollTool) Risk() RiskLevel        { return RiskLow } // scroll is read-ish, low risk
func (t *ScrollTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"x":       {Type: "integer", Description: "X coordinate for scroll position"},
			"y":       {Type: "integer", Description: "Y coordinate for scroll position"},
			"delta_x": {Type: "integer", Description: "Horizontal scroll amount (negative = left)", Default: 0},
			"delta_y": {Type: "integer", Description: "Vertical scroll amount (negative = up, positive = down)"},
		},
		Required: []string{"x", "y", "delta_y"},
	}
}

func (t *ScrollTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		X      int `json:"x"`
		Y      int `json:"y"`
		DeltaX int `json:"delta_x"`
		DeltaY int `json:"delta_y"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return &ToolResult{IsError: true, Error: "invalid params: " + err.Error()}, nil
	}

	// Scroll is RiskLow in actionRiskRules, auto-approved
	approved, _, err := approveAction(ctx, t.deps.Gateway, "scroll", params)
	if err != nil {
		return nil, err
	}
	if !approved {
		return &ToolResult{IsError: true, Error: "操作被人类审核员拒绝"}, nil
	}

	if err := t.deps.Input.Scroll(p.X, p.Y, p.DeltaX, p.DeltaY); err != nil {
		return nil, fmt.Errorf("scroll failed: %w", err)
	}

	return &ToolResult{
		Content: map[string]any{"scrolled": true, "x": p.X, "y": p.Y, "delta_x": p.DeltaX, "delta_y": p.DeltaY},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 7: mouse_position (read-only utility)
// ══════════════════════════════════════════════════════════════

type MousePositionTool struct{ deps ActionDeps }

func (t *MousePositionTool) Name() string           { return "mouse_position" }
func (t *MousePositionTool) Description() string    { return "获取当前鼠标光标位置" }
func (t *MousePositionTool) Category() ToolCategory { return CategoryAction }
func (t *MousePositionTool) Risk() RiskLevel        { return RiskLow }
func (t *MousePositionTool) InputSchema() ToolSchema {
	return ToolSchema{Type: "object"}
}

func (t *MousePositionTool) Execute(ctx context.Context, _ json.RawMessage) (*ToolResult, error) {
	x, y, err := t.deps.Input.GetMousePosition()
	if err != nil {
		return nil, fmt.Errorf("get mouse position: %w", err)
	}

	return &ToolResult{
		Content: map[string]any{"x": x, "y": y},
	}, nil
}
