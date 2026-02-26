package config

import (
	"testing"
)

func TestSetAndGetConfigOverride(t *testing.T) {
	defer ResetConfigOverrides()

	ok, errMsg := SetConfigOverride("agents.defaults.contextTokens", 100000)
	if !ok || errMsg != "" {
		t.Fatalf("SetConfigOverride failed: %s", errMsg)
	}

	overrides := GetConfigOverrides()
	agents, _ := overrides["agents"].(map[string]interface{})
	defaults, _ := agents["defaults"].(map[string]interface{})
	if defaults["contextTokens"] != 100000 {
		t.Fatalf("got %v, want 100000", defaults["contextTokens"])
	}
}

func TestSetConfigOverrideInvalidPath(t *testing.T) {
	defer ResetConfigOverrides()

	ok, errMsg := SetConfigOverride("", "value")
	if ok {
		t.Fatal("expected failure for empty path")
	}
	if errMsg == "" {
		t.Fatal("expected error message")
	}
}

func TestUnsetConfigOverride(t *testing.T) {
	defer ResetConfigOverrides()

	SetConfigOverride("a.b.c", 42)

	ok, removed, errMsg := UnsetConfigOverride("a.b.c")
	if !ok || !removed || errMsg != "" {
		t.Fatalf("UnsetConfigOverride failed: ok=%v removed=%v err=%s", ok, removed, errMsg)
	}

	// 空父节点应被清理
	overrides := GetConfigOverrides()
	if len(overrides) != 0 {
		t.Fatalf("expected empty overrides after unset, got %v", overrides)
	}
}

func TestUnsetConfigOverrideMissing(t *testing.T) {
	defer ResetConfigOverrides()

	ok, removed, _ := UnsetConfigOverride("nonexistent.key")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if removed {
		t.Fatal("expected removed=false for missing key")
	}
}

func TestResetConfigOverrides(t *testing.T) {
	SetConfigOverride("foo.bar", "baz")
	ResetConfigOverrides()
	overrides := GetConfigOverrides()
	if len(overrides) != 0 {
		t.Fatalf("expected empty overrides after reset, got %v", overrides)
	}
}

func TestApplyConfigOverrides(t *testing.T) {
	defer ResetConfigOverrides()

	base := map[string]interface{}{
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"contextTokens": 200000,
				"timeout":       300,
			},
		},
		"logging": map[string]interface{}{
			"level": "info",
		},
	}

	SetConfigOverride("agents.defaults.contextTokens", 100000)
	SetConfigOverride("logging.level", "debug")

	result := ApplyConfigOverrides(base)

	// 覆盖值生效
	agents := result["agents"].(map[string]interface{})
	defaults := agents["defaults"].(map[string]interface{})
	if defaults["contextTokens"] != 100000 {
		t.Fatalf("contextTokens = %v, want 100000", defaults["contextTokens"])
	}
	if defaults["timeout"] != 300 {
		t.Fatalf("timeout should be preserved, got %v", defaults["timeout"])
	}

	logging := result["logging"].(map[string]interface{})
	if logging["level"] != "debug" {
		t.Fatalf("level = %v, want debug", logging["level"])
	}

	// 原始 base 未被修改
	originalAgents := base["agents"].(map[string]interface{})
	originalDefaults := originalAgents["defaults"].(map[string]interface{})
	if originalDefaults["contextTokens"] != 200000 {
		t.Fatal("original base should not be modified")
	}
}

func TestApplyConfigOverridesEmpty(t *testing.T) {
	defer ResetConfigOverrides()

	base := map[string]interface{}{"key": "value"}
	result := ApplyConfigOverrides(base)

	// 无覆盖时应返回原 base
	if result["key"] != "value" {
		t.Fatalf("got %v, want value", result["key"])
	}
}

// TestApplyConfigOverridesPrimitiveValue 验证非 map override 不被丢弃
// 回归测试 BUG-1: mergeOverrides 当 override 非 map 时返回 nil
func TestApplyConfigOverridesPrimitiveValue(t *testing.T) {
	defer ResetConfigOverrides()
	SetConfigOverride("logging.level", "debug")

	base := map[string]interface{}{
		"logging": map[string]interface{}{"level": "info"},
	}
	result := ApplyConfigOverrides(base)

	logging, ok := result["logging"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected logging to be map, got %T", result["logging"])
	}
	if logging["level"] != "debug" {
		t.Fatalf("expected 'debug', got %v", logging["level"])
	}
}

// TestMergeOverridesPrimitiveOverride 验证原始类型 override 的递归合并
// 回归测试 BUG-1: 深层嵌套中 override 为原始值时不返回 nil
func TestMergeOverridesPrimitiveOverride(t *testing.T) {
	base := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "old",
			},
		},
	}
	override := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "new",
			},
		},
	}
	result := mergeOverrides(base, override)
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	a := resultMap["a"].(map[string]interface{})
	b := a["b"].(map[string]interface{})
	if b["c"] != "new" {
		t.Fatalf("expected 'new', got %v", b["c"])
	}
}
