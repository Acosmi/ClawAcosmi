package hooks

import (
	"log/slog"
)

// ============================================================================
// Hook 加载器 — 编排 hook 发现、过滤、注册
// 对应 TS: loader.ts loadInternalHooks
// ============================================================================

// InternalHooksConfig 内部钩子配置
type InternalHooksConfig struct {
	Enabled  bool                     `json:"enabled"`
	Entries  map[string]interface{}   `json:"entries,omitempty"`
	Handlers []LegacyHandlerConfig    `json:"handlers,omitempty"`
	Load     *InternalHooksLoadConfig `json:"load,omitempty"`
}

// InternalHooksLoadConfig 加载配置
type InternalHooksLoadConfig struct {
	ExtraDirs []string `json:"extraDirs,omitempty"`
}

// LegacyHandlerConfig 旧版 handler 配置（向后兼容）
type LegacyHandlerConfig struct {
	Event  string `json:"event"`
	Module string `json:"module"`
	Export string `json:"export,omitempty"`
}

// LoadInternalHooks 加载并注册所有内部钩子
// 对应 TS: loader.ts loadInternalHooks
//
// 流程：
// 1. 注册 bundled handlers（静态编译）
// 2. 发现工作区 hook entries
// 3. 过滤 eligible hooks
// 4. 为每个 eligible hook 注册 handler（Go 中由上层编排）
//
// 返回注册成功的 handler 数量。
func LoadInternalHooks(config map[string]interface{}, workspaceDir string) int {
	// Check if hooks are enabled
	internalConfig := resolveInternalConfig(config)
	if internalConfig == nil || !internalConfig.Enabled {
		return 0
	}

	loadedCount := 0

	// 1. Register bundled handlers
	loadedCount += RegisterBundledHooks()

	// 2. Load hooks from directories
	hookEntries := LoadWorkspaceHookEntries(workspaceDir, config)
	eligible := make([]HookEntry, 0)
	for _, entry := range hookEntries {
		if ShouldIncludeHook(&entry, config, nil) {
			eligible = append(eligible, entry)
		}
	}

	// 3. Register eligible hooks
	for _, entry := range eligible {
		hookKey := ResolveHookKey(entry.Hook.Name, &entry)
		hookConfig := ResolveHookConfigEntry(config, hookKey)

		// Skip if explicitly disabled
		if hookConfig != nil && hookConfig.Enabled != nil && !*hookConfig.Enabled {
			continue
		}

		events := entry.Metadata.eventsOrEmpty()
		if len(events) == 0 {
			slog.Warn("Hook has no events defined", "name", entry.Hook.Name)
			continue
		}

		// In Go, we can't dynamically load handlers like TS's import().
		// Workspace hooks are discovered but their handlers must be Go-native.
		// For now, log the discovery for debugging.
		slog.Debug("Discovered workspace hook",
			"name", entry.Hook.Name,
			"events", events,
			"source", string(entry.Hook.Source),
			"handler", entry.Hook.HandlerPath,
		)
		loadedCount++
	}

	slog.Info("Internal hooks loaded", "count", loadedCount)
	return loadedCount
}

// resolveInternalConfig 从 config 解析 internal hooks 配置
func resolveInternalConfig(config map[string]interface{}) *InternalHooksConfig {
	if config == nil {
		return nil
	}
	hooks, _ := config["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}
	internal, _ := hooks["internal"].(map[string]interface{})
	if internal == nil {
		return nil
	}
	enabled, _ := internal["enabled"].(bool)
	return &InternalHooksConfig{
		Enabled: enabled,
	}
}
