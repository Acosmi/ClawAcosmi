package plugins

import "strings"

// NormalizedPluginsConfig 规范化后的插件配置
// 对应 TS: config-state.ts NormalizedPluginsConfig
type NormalizedPluginsConfig struct {
	Enabled   bool
	Allow     []string
	Deny      []string
	LoadPaths []string
	Slots     PluginSlotsConfig
	Entries   map[string]PluginEntryConfig
}

// PluginSlotsConfig 插件槽位配置
type PluginSlotsConfig struct {
	Memory *string // nil = use default, pointer to empty = "none"
}

// PluginEntryConfig 单个插件配置条目
type PluginEntryConfig struct {
	Enabled *bool
	Config  interface{}
}

// BundledEnabledByDefault 默认启用的内置插件
var BundledEnabledByDefault = map[string]bool{
	"device-pair":   true,
	"phone-control": true,
	"talk-voice":    true,
}

// NormalizePluginsConfig 规范化插件配置
// 对应 TS: config-state.ts normalizePluginsConfig
func NormalizePluginsConfig(raw map[string]interface{}) NormalizedPluginsConfig {
	if raw == nil {
		memDefault := DefaultSlotIdForKey("memory")
		return NormalizedPluginsConfig{
			Enabled:   true,
			Allow:     nil,
			Deny:      nil,
			LoadPaths: nil,
			Slots:     PluginSlotsConfig{Memory: &memDefault},
			Entries:   make(map[string]PluginEntryConfig),
		}
	}

	enabled := true
	if v, ok := raw["enabled"].(bool); ok {
		enabled = v
	}

	allow := normalizeStringListFromInterface(raw["allow"])
	deny := normalizeStringListFromInterface(raw["deny"])

	var loadPaths []string
	if loadMap, ok := raw["load"].(map[string]interface{}); ok {
		loadPaths = normalizeStringListFromInterface(loadMap["paths"])
	}

	var memorySlot *string
	if slotsMap, ok := raw["slots"].(map[string]interface{}); ok {
		memorySlot = normalizeSlotValue(slotsMap["memory"])
	}
	if memorySlot == nil {
		def := DefaultSlotIdForKey("memory")
		memorySlot = &def
	}

	entries := normalizePluginEntriesFromInterface(raw["entries"])

	return NormalizedPluginsConfig{
		Enabled:   enabled,
		Allow:     allow,
		Deny:      deny,
		LoadPaths: loadPaths,
		Slots:     PluginSlotsConfig{Memory: memorySlot},
		Entries:   entries,
	}
}

// EnableStateResult enable 检查结果
type EnableStateResult struct {
	Enabled bool
	Reason  string
}

// ResolveEnableState 决定插件是否启用
// 对应 TS: config-state.ts resolveEnableState
func ResolveEnableState(id string, origin PluginOrigin, config NormalizedPluginsConfig) EnableStateResult {
	if !config.Enabled {
		return EnableStateResult{Enabled: false, Reason: "plugins disabled"}
	}
	for _, d := range config.Deny {
		if d == id {
			return EnableStateResult{Enabled: false, Reason: "blocked by denylist"}
		}
	}
	if len(config.Allow) > 0 {
		found := false
		for _, a := range config.Allow {
			if a == id {
				found = true
				break
			}
		}
		if !found {
			return EnableStateResult{Enabled: false, Reason: "not in allowlist"}
		}
	}
	if config.Slots.Memory != nil && *config.Slots.Memory == id {
		return EnableStateResult{Enabled: true}
	}
	if entry, ok := config.Entries[id]; ok {
		if entry.Enabled != nil {
			if *entry.Enabled {
				return EnableStateResult{Enabled: true}
			}
			return EnableStateResult{Enabled: false, Reason: "disabled in config"}
		}
	}
	if origin == PluginOriginBundled {
		if BundledEnabledByDefault[id] {
			return EnableStateResult{Enabled: true}
		}
		return EnableStateResult{Enabled: false, Reason: "bundled (disabled by default)"}
	}
	return EnableStateResult{Enabled: true}
}

// MemorySlotDecisionResult 内存槽位判定结果
type MemorySlotDecisionResult struct {
	Enabled  bool
	Reason   string
	Selected bool
}

// ResolveMemorySlotDecision 决定内存槽位插件的启用状态
// 对应 TS: config-state.ts resolveMemorySlotDecision
func ResolveMemorySlotDecision(id string, kind string, slot *string, selectedID string) MemorySlotDecisionResult {
	if kind != "memory" {
		return MemorySlotDecisionResult{Enabled: true}
	}
	// slot == nil → use default (allow)
	// slot == pointer to "" → disabled ("none")
	if slot != nil && *slot == "" {
		return MemorySlotDecisionResult{Enabled: false, Reason: "memory slot disabled"}
	}
	if slot != nil && *slot != "" {
		if *slot == id {
			return MemorySlotDecisionResult{Enabled: true, Selected: true}
		}
		return MemorySlotDecisionResult{
			Enabled: false,
			Reason:  "memory slot set to \"" + *slot + "\"",
		}
	}
	if selectedID != "" && selectedID != id {
		return MemorySlotDecisionResult{
			Enabled: false,
			Reason:  "memory slot already filled by \"" + selectedID + "\"",
		}
	}
	return MemorySlotDecisionResult{Enabled: true, Selected: true}
}

// --- helpers ---

func normalizeSlotValue(v interface{}) *string {
	s, ok := v.(string)
	if !ok {
		return nil
	}
	trimmed := trimStr(s)
	if trimmed == "" {
		return nil
	}
	if lower := lowerStr(trimmed); lower == "none" {
		empty := ""
		return &empty
	}
	return &trimmed
}

func normalizeStringListFromInterface(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			trimmed := trimStr(s)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizePluginEntriesFromInterface(v interface{}) map[string]PluginEntryConfig {
	entries := make(map[string]PluginEntryConfig)
	m, ok := v.(map[string]interface{})
	if !ok {
		return entries
	}
	for key, val := range m {
		trimmedKey := trimStr(key)
		if trimmedKey == "" {
			continue
		}
		em, ok := val.(map[string]interface{})
		if !ok {
			entries[trimmedKey] = PluginEntryConfig{}
			continue
		}
		entry := PluginEntryConfig{}
		if e, ok := em["enabled"].(bool); ok {
			entry.Enabled = &e
		}
		if c, exists := em["config"]; exists {
			entry.Config = c
		}
		entries[trimmedKey] = entry
	}
	return entries
}

func trimStr(s string) string {
	return strings.TrimSpace(s)
}

func lowerStr(s string) string {
	return strings.ToLower(s)
}
