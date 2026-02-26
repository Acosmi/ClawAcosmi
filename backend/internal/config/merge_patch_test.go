package config

import (
	"encoding/json"
	"testing"
)

func jsonEqual(t *testing.T, got, want interface{}) bool {
	t.Helper()
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	return string(gotJSON) == string(wantJSON)
}

func TestApplyMergePatch_BasicMerge(t *testing.T) {
	base := map[string]interface{}{
		"a": "old",
		"b": float64(1),
	}
	patch := map[string]interface{}{
		"a": "new",
		"c": float64(2),
	}
	result := ApplyMergePatch(base, patch)
	m := result.(map[string]interface{})
	if m["a"] != "new" {
		t.Errorf("a = %v, want 'new'", m["a"])
	}
	if m["b"] != float64(1) {
		t.Errorf("b = %v, want 1", m["b"])
	}
	if m["c"] != float64(2) {
		t.Errorf("c = %v, want 2", m["c"])
	}
}

func TestApplyMergePatch_NullDeletes(t *testing.T) {
	base := map[string]interface{}{
		"a": "keep",
		"b": "delete-me",
	}
	patch := map[string]interface{}{
		"b": nil,
	}
	result := ApplyMergePatch(base, patch)
	m := result.(map[string]interface{})
	if m["a"] != "keep" {
		t.Errorf("a should be preserved")
	}
	if _, exists := m["b"]; exists {
		t.Errorf("b should be deleted by null patch")
	}
}

func TestApplyMergePatch_NestedRecursive(t *testing.T) {
	base := map[string]interface{}{
		"gateway": map[string]interface{}{
			"port": float64(18789),
			"mode": "local",
		},
	}
	patch := map[string]interface{}{
		"gateway": map[string]interface{}{
			"mode": "remote",
		},
	}
	result := ApplyMergePatch(base, patch)
	m := result.(map[string]interface{})
	gw := m["gateway"].(map[string]interface{})
	if gw["port"] != float64(18789) {
		t.Errorf("gateway.port should be preserved, got %v", gw["port"])
	}
	if gw["mode"] != "remote" {
		t.Errorf("gateway.mode = %v, want 'remote'", gw["mode"])
	}
}

func TestApplyMergePatch_NonObjectPatch(t *testing.T) {
	base := map[string]interface{}{"a": "old"}
	// Non-object patch replaces entirely
	result := ApplyMergePatch(base, "replaced")
	if result != "replaced" {
		t.Errorf("non-object patch should replace base entirely, got %v", result)
	}
}

func TestApplyMergePatch_EmptyBase(t *testing.T) {
	patch := map[string]interface{}{
		"x": float64(42),
	}
	result := ApplyMergePatch(nil, patch)
	m := result.(map[string]interface{})
	if m["x"] != float64(42) {
		t.Errorf("x = %v, want 42", m["x"])
	}
}

func TestApplyMergePatch_NestedNullDelete(t *testing.T) {
	base := map[string]interface{}{
		"session": map[string]interface{}{
			"scope":   "per-sender",
			"mainKey": "main",
		},
	}
	patch := map[string]interface{}{
		"session": map[string]interface{}{
			"mainKey": nil,
		},
	}
	result := ApplyMergePatch(base, patch)
	m := result.(map[string]interface{})
	session := m["session"].(map[string]interface{})
	if session["scope"] != "per-sender" {
		t.Errorf("session.scope should be preserved")
	}
	if _, exists := session["mainKey"]; exists {
		t.Errorf("session.mainKey should be deleted by null patch")
	}
}

func TestApplyMergePatch_DoesNotMutateBase(t *testing.T) {
	base := map[string]interface{}{
		"a": "old",
		"b": "stay",
	}
	patch := map[string]interface{}{
		"a": "new",
	}
	_ = ApplyMergePatch(base, patch)
	if base["a"] != "old" {
		t.Errorf("ApplyMergePatch mutated base: a = %v", base["a"])
	}
}
