package hooks

import (
	"testing"
)

// ============================================================================
// hook_config_test.go — 配置解析 + 资格检查测试
// ============================================================================

func TestResolveConfigPath(t *testing.T) {
	config := map[string]interface{}{
		"browser": map[string]interface{}{
			"enabled": true,
		},
		"workspace": map[string]interface{}{
			"dir": "/tmp/workspace",
		},
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
	}{
		{"nested bool", "browser.enabled", true},
		{"nested string", "workspace.dir", "/tmp/workspace"},
		{"missing path", "foo.bar", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveConfigPath(config, tt.path)
			if result != tt.expected {
				t.Errorf("ResolveConfigPath(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestIsConfigPathTruthy(t *testing.T) {
	config := map[string]interface{}{
		"browser": map[string]interface{}{
			"enabled": false,
		},
	}

	// Explicit false
	if IsConfigPathTruthy(config, "browser.enabled") {
		t.Error("browser.enabled should be false")
	}

	// Missing with default
	if !IsConfigPathTruthy(nil, "browser.enabled") {
		t.Error("browser.enabled should default to true")
	}

	// Missing without default
	if IsConfigPathTruthy(config, "nonexistent.path") {
		t.Error("nonexistent.path should be false")
	}
}

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"nil", nil, false},
		{"true", true, true},
		{"false", false, false},
		{"non-zero float", 1.0, true},
		{"zero float", 0.0, false},
		{"non-empty string", "hello", true},
		{"whitespace string", "   ", false},
		{"empty string", "", false},
		{"map (truthy)", map[string]interface{}{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTruthy(tt.value); got != tt.expected {
				t.Errorf("isTruthy(%v) = %v, want %v", tt.value, got, tt.expected)
			}
		})
	}
}

func TestShouldIncludeHook_Disabled(t *testing.T) {
	f := false
	entry := &HookEntry{
		Hook:     Hook{Name: "test-hook", Source: HookSourceBundled},
		Metadata: &HookMetadata{Events: []string{"command"}},
	}
	config := map[string]interface{}{
		"hooks": map[string]interface{}{
			"internal": map[string]interface{}{
				"entries": map[string]interface{}{
					"test-hook": map[string]interface{}{
						"enabled": false,
					},
				},
			},
		},
	}
	_ = f
	if ShouldIncludeHook(entry, config, nil) {
		t.Error("disabled hook should not be included")
	}
}

func TestShouldIncludeHook_Always(t *testing.T) {
	always := true
	entry := &HookEntry{
		Hook: Hook{Name: "always-hook", Source: HookSourceBundled},
		Metadata: &HookMetadata{
			Events: []string{"command"},
			Always: &always,
			Requires: &HookRequirements{
				Bins: []string{"nonexistent-binary-123"},
			},
		},
	}
	if !ShouldIncludeHook(entry, nil, nil) {
		t.Error("always hook should be included even with missing bins")
	}
}

func TestShouldIncludeHook_NilMetadata(t *testing.T) {
	entry := &HookEntry{
		Hook:     Hook{Name: "simple-hook", Source: HookSourceBundled},
		Metadata: nil,
	}
	if !ShouldIncludeHook(entry, nil, nil) {
		t.Error("hook with nil metadata should be included by default")
	}
}

func TestResolveHookConfigEntry(t *testing.T) {
	config := map[string]interface{}{
		"hooks": map[string]interface{}{
			"internal": map[string]interface{}{
				"entries": map[string]interface{}{
					"session-memory": map[string]interface{}{
						"enabled":  true,
						"messages": float64(20),
						"env": map[string]interface{}{
							"API_KEY": "test-key",
						},
					},
				},
			},
		},
	}

	hc := ResolveHookConfigEntry(config, "session-memory")
	if hc == nil {
		t.Fatal("expected non-nil HookConfig")
	}
	if hc.Enabled == nil || !*hc.Enabled {
		t.Error("expected enabled=true")
	}
	if hc.Messages == nil || *hc.Messages != 20 {
		t.Error("expected messages=20")
	}
	if hc.Env["API_KEY"] != "test-key" {
		t.Error("expected env API_KEY=test-key")
	}

	// Missing key
	if ResolveHookConfigEntry(config, "nonexistent") != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestParseFrontmatter(t *testing.T) {
	content := `---
name: test-hook
description: A test hook
emoji: 🎣
---

# Hook Content
Some markdown here.
`
	fm := ParseFrontmatter(content)
	if fm["name"] != "test-hook" {
		t.Errorf("expected name=test-hook, got %q", fm["name"])
	}
	if fm["description"] != "A test hook" {
		t.Errorf("expected description, got %q", fm["description"])
	}
	if fm["emoji"] != "🎣" {
		t.Errorf("expected emoji, got %q", fm["emoji"])
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "# Just Markdown\nNo frontmatter here."
	fm := ParseFrontmatter(content)
	if len(fm) != 0 {
		t.Errorf("expected empty map, got %v", fm)
	}
}

func TestResolveHookKey(t *testing.T) {
	// With metadata hookKey
	entry := &HookEntry{
		Metadata: &HookMetadata{HookKey: "custom-key"},
	}
	if got := ResolveHookKey("default-name", entry); got != "custom-key" {
		t.Errorf("expected custom-key, got %s", got)
	}

	// Without metadata hookKey
	entry2 := &HookEntry{Metadata: &HookMetadata{}}
	if got := ResolveHookKey("default-name", entry2); got != "default-name" {
		t.Errorf("expected default-name, got %s", got)
	}
}

func TestParseBooleanValue(t *testing.T) {
	tests := []struct {
		input    string
		fallback bool
		expected bool
	}{
		{"true", false, true},
		{"yes", false, true},
		{"1", false, true},
		{"false", true, false},
		{"no", true, false},
		{"0", true, false},
		{"maybe", true, true},
		{"maybe", false, false},
	}
	for _, tt := range tests {
		if got := parseBooleanValue(tt.input, tt.fallback); got != tt.expected {
			t.Errorf("parseBooleanValue(%q, %v) = %v, want %v", tt.input, tt.fallback, got, tt.expected)
		}
	}
}
