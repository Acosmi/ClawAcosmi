package plugins

import "testing"

func TestNormalizePluginsConfig_Nil(t *testing.T) {
	cfg := NormalizePluginsConfig(nil)
	if !cfg.Enabled {
		t.Error("expected enabled=true for nil config")
	}
	if cfg.Slots.Memory == nil || *cfg.Slots.Memory != "memory-core" {
		t.Error("expected default memory slot 'memory-core'")
	}
}

func TestNormalizePluginsConfig_DisabledWithDenyList(t *testing.T) {
	raw := map[string]interface{}{
		"enabled": false,
		"deny":    []interface{}{"bad-plugin"},
	}
	cfg := NormalizePluginsConfig(raw)
	if cfg.Enabled {
		t.Error("expected enabled=false")
	}
	if len(cfg.Deny) != 1 || cfg.Deny[0] != "bad-plugin" {
		t.Errorf("expected deny=[bad-plugin], got %v", cfg.Deny)
	}
}

func TestNormalizePluginsConfig_MemorySlotNone(t *testing.T) {
	raw := map[string]interface{}{
		"slots": map[string]interface{}{
			"memory": "none",
		},
	}
	cfg := NormalizePluginsConfig(raw)
	if cfg.Slots.Memory == nil {
		t.Fatal("expected non-nil memory slot")
	}
	if *cfg.Slots.Memory != "" {
		t.Errorf("expected empty string for 'none' slot, got %q", *cfg.Slots.Memory)
	}
}

func TestResolveEnableState_PluginsDisabled(t *testing.T) {
	cfg := NormalizedPluginsConfig{Enabled: false}
	result := ResolveEnableState("any", PluginOriginGlobal, cfg)
	if result.Enabled {
		t.Error("expected disabled when plugins disabled")
	}
}

func TestResolveEnableState_Denylist(t *testing.T) {
	cfg := NormalizedPluginsConfig{
		Enabled: true,
		Deny:    []string{"blocked"},
	}
	result := ResolveEnableState("blocked", PluginOriginGlobal, cfg)
	if result.Enabled {
		t.Error("expected disabled for denied plugin")
	}
	if result.Reason != "blocked by denylist" {
		t.Errorf("unexpected reason: %s", result.Reason)
	}
}

func TestResolveEnableState_AllowlistMissing(t *testing.T) {
	cfg := NormalizedPluginsConfig{
		Enabled: true,
		Allow:   []string{"allowed-plugin"},
	}
	result := ResolveEnableState("other", PluginOriginGlobal, cfg)
	if result.Enabled {
		t.Error("expected disabled when not in allowlist")
	}
}

func TestResolveEnableState_BundledDefault(t *testing.T) {
	cfg := NormalizedPluginsConfig{
		Enabled: true,
		Entries: make(map[string]PluginEntryConfig),
	}
	// "device-pair" is in BundledEnabledByDefault
	result := ResolveEnableState("device-pair", PluginOriginBundled, cfg)
	if !result.Enabled {
		t.Error("expected enabled for bundled default plugin")
	}

	// "unknown-bundled" is not
	result2 := ResolveEnableState("unknown-bundled", PluginOriginBundled, cfg)
	if result2.Enabled {
		t.Error("expected disabled for non-default bundled plugin")
	}
}

func TestResolveMemorySlotDecision_NonMemory(t *testing.T) {
	result := ResolveMemorySlotDecision("any", "other", nil, "")
	if !result.Enabled {
		t.Error("expected enabled for non-memory kind")
	}
}

func TestResolveMemorySlotDecision_Disabled(t *testing.T) {
	empty := ""
	result := ResolveMemorySlotDecision("any", "memory", &empty, "")
	if result.Enabled {
		t.Error("expected disabled for empty slot (none)")
	}
}

func TestResolveMemorySlotDecision_Selected(t *testing.T) {
	slot := "my-memory"
	result := ResolveMemorySlotDecision("my-memory", "memory", &slot, "")
	if !result.Enabled || !result.Selected {
		t.Error("expected enabled+selected for matching slot")
	}
}

func TestResolveMemorySlotDecision_AlreadyFilled(t *testing.T) {
	result := ResolveMemorySlotDecision("plugin-b", "memory", nil, "plugin-a")
	if result.Enabled {
		t.Error("expected disabled when slot already filled")
	}
}
