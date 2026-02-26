package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"Argus-compound/go-sensory/internal/input"
)

// ──────────────────────────────────────────────────────────────
// macOS shortcut definitions
// ──────────────────────────────────────────────────────────────

// shortcutEntry maps a semantic action name to a key combo + risk metadata.
type shortcutEntry struct {
	Keys        []input.Key // key combo to press
	Risk        RiskLevel   // default risk for this shortcut
	Description string      // human-readable for approval UI
	ActionName  string      // action name sent to ApprovalGateway
}

// shortcutTable is the central registry of all macOS shortcuts.
// Adding a new shortcut = adding one line here.
var shortcutTable = map[string]shortcutEntry{
	// ── Clipboard ──
	"copy": {
		Keys:        []input.Key{input.KeyCommand, input.KeyC},
		Risk:        RiskLow,
		Description: "复制 (⌘C)",
		ActionName:  "macos_copy",
	},
	"paste": {
		Keys:        []input.Key{input.KeyCommand, input.KeyV},
		Risk:        RiskMedium,
		Description: "粘贴 (⌘V)",
		ActionName:  "macos_paste",
	},
	"cut": {
		Keys:        []input.Key{input.KeyCommand, input.KeyX},
		Risk:        RiskMedium,
		Description: "剪切 (⌘X)",
		ActionName:  "macos_cut",
	},
	"select_all": {
		Keys:        []input.Key{input.KeyCommand, input.KeyA},
		Risk:        RiskLow,
		Description: "全选 (⌘A)",
		ActionName:  "macos_select_all",
	},

	// ── Edit operations ──
	"undo": {
		Keys:        []input.Key{input.KeyCommand, input.KeyZ},
		Risk:        RiskMedium,
		Description: "撤销 (⌘Z)",
		ActionName:  "macos_undo",
	},
	"redo": {
		Keys:        []input.Key{input.KeyCommand, input.KeyShift, input.KeyZ},
		Risk:        RiskMedium,
		Description: "重做 (⇧⌘Z)",
		ActionName:  "macos_redo",
	},
	"save": {
		Keys:        []input.Key{input.KeyCommand, input.KeyS},
		Risk:        RiskMedium,
		Description: "保存 (⌘S)",
		ActionName:  "macos_save",
	},
	"find": {
		Keys:        []input.Key{input.KeyCommand, input.KeyF},
		Risk:        RiskLow,
		Description: "查找 (⌘F)",
		ActionName:  "macos_find",
	},

	// ── Window management ──
	"close_window": {
		Keys:        []input.Key{input.KeyCommand, input.KeyW},
		Risk:        RiskMedium,
		Description: "关闭窗口 (⌘W)",
		ActionName:  "macos_close_window",
	},
	"minimize": {
		Keys:        []input.Key{input.KeyCommand, input.KeyM},
		Risk:        RiskLow,
		Description: "最小化 (⌘M)",
		ActionName:  "macos_minimize",
	},
	"hide": {
		Keys:        []input.Key{input.KeyCommand, input.KeyH},
		Risk:        RiskLow,
		Description: "隐藏 (⌘H)",
		ActionName:  "macos_hide",
	},
	"fullscreen": {
		Keys:        []input.Key{input.KeyControl, input.KeyCommand, input.KeyF},
		Risk:        RiskLow,
		Description: "全屏 (⌃⌘F)",
		ActionName:  "macos_fullscreen",
	},

	// ── App switching ──
	"switch_app": {
		Keys:        []input.Key{input.KeyCommand, input.KeyTab},
		Risk:        RiskMedium,
		Description: "切换应用 (⌘Tab)",
		ActionName:  "macos_switch_app",
	},
	"spotlight": {
		Keys:        []input.Key{input.KeyCommand, input.KeySpace},
		Risk:        RiskMedium,
		Description: "Spotlight 搜索 (⌘Space)",
		ActionName:  "macos_spotlight",
	},

	// ── Screenshot ──
	"screenshot": {
		Keys:        []input.Key{input.KeyCommand, input.KeyShift, input.Key(0x15)}, // 0x15 = key 3 area
		Risk:        RiskLow,
		Description: "截屏 (⇧⌘3)",
		ActionName:  "macos_screenshot",
	},
	"screenshot_region": {
		Keys:        []input.Key{input.KeyCommand, input.KeyShift, input.Key(0x14)}, // 0x14 = key 4 area
		Risk:        RiskLow,
		Description: "区域截屏 (⇧⌘4)",
		ActionName:  "macos_screenshot_region",
	},

	// ── Tab management (browser/terminal) ──
	"new_tab": {
		Keys:        []input.Key{input.KeyCommand, input.KeyT},
		Risk:        RiskMedium,
		Description: "新建标签 (⌘T)",
		ActionName:  "macos_new_tab",
	},
	"close_tab": {
		Keys:        []input.Key{input.KeyCommand, input.KeyW},
		Risk:        RiskMedium,
		Description: "关闭标签 (⌘W)",
		ActionName:  "macos_close_tab",
	},

	// ── Navigation ──
	"back": {
		Keys:        []input.Key{input.KeyCommand, input.KeyArrowLeft},
		Risk:        RiskLow,
		Description: "后退 (⌘←)",
		ActionName:  "macos_back",
	},
	"forward": {
		Keys:        []input.Key{input.KeyCommand, input.KeyArrowRight},
		Risk:        RiskLow,
		Description: "前进 (⌘→)",
		ActionName:  "macos_forward",
	},
}

// ── Computed helpers ──

// shortcutNames returns all valid action names for schema enum.
func shortcutNames() []string {
	names := make([]string, 0, len(shortcutTable))
	for name := range shortcutTable {
		names = append(names, name)
	}
	return names
}

// ──────────────────────────────────────────────────────────────
// macOS dependencies
// ──────────────────────────────────────────────────────────────

// MacOSDeps bundles dependencies for macOS tools.
type MacOSDeps struct {
	Input   input.InputController
	Gateway *input.ApprovalGateway
}

// RegisterMacOSTools batch-registers macOS-specific MCP tools.
func RegisterMacOSTools(r *Registry, deps MacOSDeps) {
	r.Register(&MacOSShortcutTool{deps: deps})
	r.Register(&OpenURLTool{deps: deps})
}

// ══════════════════════════════════════════════════════════════
// Tool 1: macos_shortcut — unified semantic shortcut executor
// ══════════════════════════════════════════════════════════════

// MacOSShortcutTool executes named macOS keyboard shortcuts.
//
// Architecture: single tool with action enum (not 20+ separate tools).
// Rationale: O(1) extension (add entry to shortcutTable), cleaner
// MCP schema, consistent risk routing, avoids registry bloat.
type MacOSShortcutTool struct{ deps MacOSDeps }

func (t *MacOSShortcutTool) Name() string { return "macos_shortcut" }
func (t *MacOSShortcutTool) Description() string {
	return "执行 macOS 系统快捷键（复制/粘贴/保存/切换应用等）"
}
func (t *MacOSShortcutTool) Category() ToolCategory { return CategoryMacOS }
func (t *MacOSShortcutTool) Risk() RiskLevel        { return RiskMedium } // varies per action
func (t *MacOSShortcutTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"action": {
				Type:        "string",
				Description: "Shortcut to execute",
				Enum:        shortcutNames(),
			},
		},
		Required: []string{"action"},
	}
}

func (t *MacOSShortcutTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Action == "" {
		return &ToolResult{IsError: true, Error: "action is required"}, nil
	}

	entry, ok := shortcutTable[p.Action]
	if !ok {
		return &ToolResult{IsError: true, Error: fmt.Sprintf("unknown shortcut: %q (use one of %v)", p.Action, shortcutNames())}, nil
	}

	// Route through ApprovalGateway with the per-shortcut action name
	// so risk rules like macos_close_window → RiskMedium apply.
	if t.deps.Gateway != nil {
		approved, _, err := t.deps.Gateway.CheckAndApprove(ctx, entry.ActionName, params, "mcp", nil)
		if err != nil {
			return nil, err
		}
		if !approved {
			return &ToolResult{IsError: true, Error: "操作被人类审核员拒绝"}, nil
		}
	}

	if err := t.deps.Input.Hotkey(entry.Keys...); err != nil {
		return nil, fmt.Errorf("shortcut %q failed: %w", p.Action, err)
	}

	return &ToolResult{
		Content: map[string]any{
			"executed":    true,
			"action":      p.Action,
			"description": entry.Description,
		},
	}, nil
}

// ══════════════════════════════════════════════════════════════
// Tool 2: open_url — macOS `open` command
// ══════════════════════════════════════════════════════════════

// OpenURLTool opens a URL or file path in the default application.
// Uses macOS `open` command underneath (equivalent to double-clicking in Finder).
type OpenURLTool struct{ deps MacOSDeps }

func (t *OpenURLTool) Name() string           { return "open_url" }
func (t *OpenURLTool) Description() string    { return "用默认应用打开 URL 或文件路径" }
func (t *OpenURLTool) Category() ToolCategory { return CategoryMacOS }
func (t *OpenURLTool) Risk() RiskLevel        { return RiskMedium }
func (t *OpenURLTool) InputSchema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaField{
			"target": {Type: "string", Description: "URL (https://...) or file path to open"},
		},
		Required: []string{"target"},
	}
}

func (t *OpenURLTool) Execute(ctx context.Context, params json.RawMessage) (*ToolResult, error) {
	var p struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(params, &p); err != nil || p.Target == "" {
		return &ToolResult{IsError: true, Error: "target is required"}, nil
	}

	// Gate through approval
	if t.deps.Gateway != nil {
		approved, _, err := t.deps.Gateway.CheckAndApprove(ctx, "open_url", params, "mcp", nil)
		if err != nil {
			return nil, err
		}
		if !approved {
			return &ToolResult{IsError: true, Error: "操作被人类审核员拒绝"}, nil
		}
	}

	// Use Spotlight approach: ⌘Space → type URL → Return
	// This is safer than os/exec("open") — it goes through the GUI layer
	// and is visible to the user, maintaining the privacy-first principle.
	if err := t.deps.Input.Hotkey(input.KeyCommand, input.KeySpace); err != nil {
		return nil, fmt.Errorf("open spotlight: %w", err)
	}
	if err := t.deps.Input.Type(p.Target); err != nil {
		return nil, fmt.Errorf("type target: %w", err)
	}
	if err := t.deps.Input.KeyPress(input.KeyReturn); err != nil {
		return nil, fmt.Errorf("press return: %w", err)
	}

	return &ToolResult{
		Content: map[string]any{
			"opened": true,
			"target": p.Target,
			"method": "spotlight",
		},
	}, nil
}
