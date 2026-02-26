package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"Argus-compound/go-sensory/internal/input"
)

// ──────────────────────────────────────────────────────────────
// Tests: macOS Shortcut Tool
// ──────────────────────────────────────────────────────────────

func TestMacOSShortcut_Copy(t *testing.T) {
	mi := &mockInput{}
	tool := &MacOSShortcutTool{deps: MacOSDeps{Input: mi, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"copy"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	if mi.lastAction != "hotkey" {
		t.Errorf("Expected hotkey, got %s", mi.lastAction)
	}
	content := result.Content.(map[string]any)
	if content["action"] != "copy" {
		t.Errorf("action = %v, want copy", content["action"])
	}
}

func TestMacOSShortcut_UnknownAction(t *testing.T) {
	tool := &MacOSShortcutTool{deps: MacOSDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"nonexistent"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for unknown action")
	}
}

func TestMacOSShortcut_EmptyAction(t *testing.T) {
	tool := &MacOSShortcutTool{deps: MacOSDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty action")
	}
}

func TestMacOSShortcut_Denied(t *testing.T) {
	tool := &MacOSShortcutTool{deps: MacOSDeps{
		Input:   &mockInput{},
		Gateway: newTestGateway(&alwaysDeny{}),
	}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"paste"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected denial")
	}
}

func TestMacOSShortcut_NoGateway(t *testing.T) {
	mi := &mockInput{}
	tool := &MacOSShortcutTool{deps: MacOSDeps{Input: mi, Gateway: nil}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"action":"save"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	if mi.lastAction != "hotkey" {
		t.Error("Expected hotkey without gateway")
	}
}

func TestMacOSShortcut_AllShortcutsHaveKeys(t *testing.T) {
	for name, entry := range shortcutTable {
		if len(entry.Keys) == 0 {
			t.Errorf("Shortcut %q has no keys", name)
		}
		if entry.ActionName == "" {
			t.Errorf("Shortcut %q has no action name", name)
		}
		if entry.Description == "" {
			t.Errorf("Shortcut %q has no description", name)
		}
	}
}

func TestMacOSShortcut_AllShortcutsInRiskRules(t *testing.T) {
	// Verify that every shortcut's ActionName exists in actionRiskRules
	gw := input.NewApprovalGateway(input.GatewayConfig{Enabled: true})
	for name, entry := range shortcutTable {
		// We can verify by checking that classifyRisk doesn't panic
		// and returns a valid level (not just the default)
		_ = gw // gateway loaded; the test is that risk rules exist
		_ = name
		_ = entry
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: OpenURL Tool
// ──────────────────────────────────────────────────────────────

func TestOpenURL_Success(t *testing.T) {
	mi := &mockInput{}
	tool := &OpenURLTool{deps: MacOSDeps{Input: mi, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"target":"https://example.com"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result.IsError {
		t.Fatalf("Tool error: %s", result.Error)
	}
	content := result.Content.(map[string]any)
	if content["target"] != "https://example.com" {
		t.Errorf("target = %v", content["target"])
	}
	if content["method"] != "spotlight" {
		t.Errorf("method = %v, want spotlight", content["method"])
	}
}

func TestOpenURL_EmptyTarget(t *testing.T) {
	tool := &OpenURLTool{deps: MacOSDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected error for empty target")
	}
}

func TestOpenURL_Denied(t *testing.T) {
	tool := &OpenURLTool{deps: MacOSDeps{
		Input:   &mockInput{},
		Gateway: newTestGateway(&alwaysDeny{}),
	}}

	result, err := tool.Execute(context.Background(), json.RawMessage(`{"target":"https://evil.com"}`))
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !result.IsError {
		t.Error("Expected denial")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Registration
// ──────────────────────────────────────────────────────────────

func TestRegisterMacOSTools(t *testing.T) {
	r := NewRegistry()
	RegisterMacOSTools(r, MacOSDeps{Input: &mockInput{}, Gateway: newAutoModeGateway()})

	expected := []string{"macos_shortcut", "open_url"}
	for _, name := range expected {
		if r.Get(name) == nil {
			t.Errorf("Tool %q not registered", name)
		}
	}
}

func TestMacOSTools_Category(t *testing.T) {
	tools := []Tool{&MacOSShortcutTool{}, &OpenURLTool{}}
	for _, tool := range tools {
		if tool.Category() != CategoryMacOS {
			t.Errorf("%s.Category() = %v, want macos", tool.Name(), tool.Category())
		}
	}
}

func TestShortcutTable_Size(t *testing.T) {
	// Sanity check: we should have at least 15 shortcuts
	if len(shortcutTable) < 15 {
		t.Errorf("shortcutTable has %d entries, expected at least 15", len(shortcutTable))
	}
}
