package skills

// plugin_skills.go — 插件技能目录解析
// 对应 TS: agents/skills/plugin-skills.ts (74L)
//
// 解析插件 manifest 中声明的 skills 目录，
// 基于插件启用状态、memory slot 决策进行过滤。

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// PluginManifestRecord 插件 manifest 记录。
type PluginManifestRecord struct {
	ID      string   `json:"id"`
	Origin  string   `json:"origin"`
	Kind    string   `json:"kind"`
	RootDir string   `json:"rootDir"`
	Skills  []string `json:"skills,omitempty"`
}

// PluginManifestRegistry 插件 manifest 注册表。
type PluginManifestRegistry struct {
	Plugins []PluginManifestRecord `json:"plugins"`
}

// NormalizedPluginsConfig 规范化的插件配置。
type NormalizedPluginsConfig struct {
	Enabled  map[string]bool `json:"enabled"`
	Disabled map[string]bool `json:"disabled"`
	Slots    PluginSlots     `json:"slots"`
}

// PluginSlots 插件槽位。
type PluginSlots struct {
	Memory string `json:"memory"`
}

// PluginEnableState 插件启用状态。
type PluginEnableState struct {
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
}

// MemorySlotDecision memory 槽位决策。
type MemorySlotDecision struct {
	Enabled  bool `json:"enabled"`
	Selected bool `json:"selected"`
}

// ResolveEnableState 解析插件启用状态。
func ResolveEnableState(id, origin string, config NormalizedPluginsConfig) PluginEnableState {
	if config.Disabled[id] {
		return PluginEnableState{Enabled: false, Reason: "explicitly disabled"}
	}
	if config.Enabled[id] {
		return PluginEnableState{Enabled: true, Reason: "explicitly enabled"}
	}
	// 默认：workspace origin 启用，其他也启用
	return PluginEnableState{Enabled: true, Reason: "default"}
}

// ResolveMemorySlotDecision 解析 memory 槽位决策。
func ResolveMemorySlotDecision(id, kind, selectedID string, slot string) MemorySlotDecision {
	if kind != "memory" {
		return MemorySlotDecision{Enabled: true, Selected: false}
	}
	if selectedID != "" && selectedID != id {
		return MemorySlotDecision{Enabled: false, Selected: false}
	}
	if slot != "" && slot != id {
		return MemorySlotDecision{Enabled: false, Selected: false}
	}
	return MemorySlotDecision{Enabled: true, Selected: true}
}

// NormalizePluginsConfig 规范化插件配置。
func NormalizePluginsConfig(raw map[string]interface{}) NormalizedPluginsConfig {
	config := NormalizedPluginsConfig{
		Enabled:  make(map[string]bool),
		Disabled: make(map[string]bool),
	}
	if raw == nil {
		return config
	}
	for id, v := range raw {
		switch val := v.(type) {
		case bool:
			if val {
				config.Enabled[id] = true
			} else {
				config.Disabled[id] = true
			}
		case map[string]interface{}:
			if enabled, ok := val["enabled"].(bool); ok {
				if enabled {
					config.Enabled[id] = true
				} else {
					config.Disabled[id] = true
				}
			}
		}
	}
	return config
}

// LoadPluginManifestRegistry 加载插件 manifest 注册表。
func LoadPluginManifestRegistry(workspaceDir string, _ interface{}) PluginManifestRegistry {
	registry := PluginManifestRegistry{}
	pluginsDir := filepath.Join(workspaceDir, ".openacosmi", "plugins")
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return registry
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(pluginsDir, entry.Name(), "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}
		// 读取 manifest 并解析（简化实现）
		registry.Plugins = append(registry.Plugins, PluginManifestRecord{
			ID:      entry.Name(),
			Origin:  "workspace",
			RootDir: filepath.Join(pluginsDir, entry.Name()),
		})
	}
	return registry
}

// ResolvePluginSkillDirs 解析插件技能目录。
// 对应 TS: resolvePluginSkillDirs
func ResolvePluginSkillDirs(workspaceDir string, pluginsConfig map[string]interface{}) []string {
	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return nil
	}

	registry := LoadPluginManifestRegistry(workspaceDir, nil)
	if len(registry.Plugins) == 0 {
		return nil
	}

	normalizedPlugins := NormalizePluginsConfig(pluginsConfig)
	memorySlot := normalizedPlugins.Slots.Memory
	var selectedMemoryPluginID string
	seen := make(map[string]bool)
	var resolved []string

	for _, record := range registry.Plugins {
		if len(record.Skills) == 0 {
			continue
		}

		enableState := ResolveEnableState(record.ID, record.Origin, normalizedPlugins)
		if !enableState.Enabled {
			continue
		}

		memoryDecision := ResolveMemorySlotDecision(record.ID, record.Kind, selectedMemoryPluginID, memorySlot)
		if !memoryDecision.Enabled {
			continue
		}
		if memoryDecision.Selected && record.Kind == "memory" {
			selectedMemoryPluginID = record.ID
		}

		for _, raw := range record.Skills {
			trimmed := strings.TrimSpace(raw)
			if trimmed == "" {
				continue
			}
			candidate := filepath.Join(record.RootDir, trimmed)
			if _, err := os.Stat(candidate); err != nil {
				slog.Warn("plugin skill path not found",
					"pluginId", record.ID,
					"path", candidate)
				continue
			}
			if seen[candidate] {
				continue
			}
			seen[candidate] = true
			resolved = append(resolved, candidate)
		}
	}

	return resolved
}
