package gateway

import (
	"testing"
)

func TestResolveHookMappings_Presets(t *testing.T) {
	raw := &HooksRawConfig{
		Presets: []string{"github", "slack"},
	}
	mappings := ResolveHookMappings(raw)
	if len(mappings) != 2 {
		t.Fatalf("expected 2 presets, got %d", len(mappings))
	}
	if mappings[0].ID != "preset-github" {
		t.Errorf("first mapping ID = %q", mappings[0].ID)
	}
	if mappings[1].ID != "preset-slack" {
		t.Errorf("second mapping ID = %q", mappings[1].ID)
	}
}

func TestResolveHookMappings_Custom(t *testing.T) {
	raw := &HooksRawConfig{
		Mappings: []HookMappingConfig{
			{
				ID:       "custom-1",
				Action:   "wake",
				WakeText: "hello",
			},
		},
	}
	mappings := ResolveHookMappings(raw)
	if len(mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(mappings))
	}
	if mappings[0].Action != "wake" {
		t.Errorf("action = %q", mappings[0].Action)
	}
}

func TestApplyHookMappings_PathMatch(t *testing.T) {
	mappings := []HookMappingResolved{
		{
			ID:        "m1",
			MatchPath: "/github",
			Action:    "wake",
			WakeText:  "triggered",
			WakeMode:  "now",
		},
	}
	ctx := &HookMappingContext{
		Path:    "/github",
		Headers: map[string]string{},
	}
	result, err := ApplyHookMappings(mappings, ctx)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result == nil {
		t.Fatal("expected match, got nil")
	}
	if result.Action != "wake" {
		t.Errorf("action = %q", result.Action)
	}
}

func TestApplyHookMappings_NoMatch(t *testing.T) {
	mappings := []HookMappingResolved{
		{
			ID:        "m1",
			MatchPath: "/github",
			Action:    "wake",
			WakeText:  "triggered",
			WakeMode:  "now",
		},
	}
	ctx := &HookMappingContext{
		Path:    "/slack",
		Headers: map[string]string{},
	}
	result, err := ApplyHookMappings(mappings, ctx)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for no match")
	}
}

func TestApplyHookMappings_SourceMatch(t *testing.T) {
	mappings := []HookMappingResolved{
		{
			ID:          "m1",
			MatchSource: "github",
			Action:      "agent",
			Message:     "github event",
			Name:        "GH",
			WakeMode:    "now",
		},
	}
	ctx := &HookMappingContext{
		Path:   "any",
		Source: "github",
		Headers: map[string]string{
			"x-github-event": "push",
		},
	}
	result, err := ApplyHookMappings(mappings, ctx)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result == nil || result.Action != "agent" {
		t.Error("expected agent match")
	}
}

func TestApplyHookMappings_WildcardPath(t *testing.T) {
	mappings := []HookMappingResolved{
		{
			ID:        "m1",
			MatchPath: "/api/*",
			Action:    "wake",
			WakeText:  "api call",
			WakeMode:  "now",
		},
	}
	ctx := &HookMappingContext{
		Path:    "/api/something",
		Headers: map[string]string{},
	}
	result, err := ApplyHookMappings(mappings, ctx)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result == nil {
		t.Fatal("expected match for wildcard")
	}
}

func TestRenderTemplate(t *testing.T) {
	ctx := &HookMappingContext{
		Path:   "/test",
		Method: "POST",
		Headers: map[string]string{
			"x-github-event": "push",
		},
		Body: map[string]interface{}{
			"action": "opened",
			"repository": map[string]interface{}{
				"full_name": "user/repo",
			},
		},
	}

	tests := []struct {
		template string
		expected string
	}{
		{"{{path}}", "/test"},
		{"{{method}}", "POST"},
		{"{{event}}", "push"},
		{"{{body.action}}", "opened"},
		{"{{body.repository.full_name}}", "user/repo"},
		{"Event: {{event}} on {{body.repository.full_name}}", "Event: push on user/repo"},
	}

	for _, tt := range tests {
		result := RenderTemplate(tt.template, ctx)
		if result != tt.expected {
			t.Errorf("RenderTemplate(%q) = %q, want %q", tt.template, result, tt.expected)
		}
	}
}

func TestGetByPath(t *testing.T) {
	obj := map[string]interface{}{
		"a": map[string]interface{}{
			"b": "value",
		},
		"items": []interface{}{"first", "second"},
	}

	tests := []struct {
		path     string
		expected interface{}
	}{
		{"a.b", "value"},
		{"items.0", "first"},
		{"items.1", "second"},
		{"missing", nil},
		{"a.missing", nil},
		{"items.99", nil},
	}

	for _, tt := range tests {
		result := GetByPath(obj, tt.path)
		if result != tt.expected {
			t.Errorf("GetByPath(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}
