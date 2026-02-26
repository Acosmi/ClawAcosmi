package hooks

// plugin_hooks.go — 插件钩子动态加载层
// 对应 TS: hooks/plugin-hooks.ts (117L)
//
// 提供 RegisterPluginHooksFromDir — 从插件目录加载、规范化、过滤钩子，
// 注册到内部钩子注册表。
//
// 注意：TS 端通过 import() 动态加载 handler JS 模块，Go 端通过
// 静态注册模式实现等价功能 — handler 通过预注册函数引用调用。
// 目录扫描和条目加载复用 workspace.go 中的 LoadHookEntriesFromDir。

import (
	"fmt"
	"log/slog"
	"path/filepath"
)

// PluginHookLoadResult 插件钩子加载结果。
// 对应 TS: PluginHookLoadResult
type PluginHookLoadResult struct {
	Hooks   []HookEntry `json:"hooks"`
	Loaded  int         `json:"loaded"`
	Skipped int         `json:"skipped"`
	Errors  []string    `json:"errors"`
}

// PluginHookLoader 插件钩子加载器。
// 封装插件元数据，用于加载和注册插件钩子。
type PluginHookLoader struct {
	PluginID string
	Source   string // 插件来源路径（manifest 路径或安装目录）
	Logger   *slog.Logger
}

// ResolveHookDir 解析钩子目录路径。
// 对应 TS: resolveHookDir
//
// 如果是相对路径，则相对于插件 source 目录解析。
func ResolveHookDir(pluginSource, dir string) string {
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(filepath.Dir(pluginSource), dir)
}

// NormalizePluginHookEntry 规范化插件钩子条目。
// 对应 TS: normalizePluginHookEntry
//
// 设置 source 为 "openacosmi-plugin"，注入 pluginId，
// 生成默认 hookKey（格式：pluginId:hookName）。
func NormalizePluginHookEntry(pluginID string, entry HookEntry) HookEntry {
	normalized := entry
	normalized.Hook.Source = HookSourcePlugin
	normalized.Hook.PluginID = pluginID

	if normalized.Metadata == nil {
		normalized.Metadata = &HookMetadata{}
	}
	if normalized.Metadata.HookKey == "" {
		normalized.Metadata.HookKey = fmt.Sprintf("%s:%s", pluginID, entry.Hook.Name)
	}
	if normalized.Metadata.Events == nil {
		normalized.Metadata.Events = []string{}
	}

	return normalized
}

// RegisterPluginHooksFromDir 从插件目录加载并注册钩子。
// 对应 TS: registerPluginHooksFromDir
//
// 流程：
//  1. 解析目录路径（ResolveHookDir）
//  2. 扫描子目录中的 HOOK.md（复用 workspace.go → LoadHookEntriesFromDir）
//  3. 规范化每个钩子条目（NormalizePluginHookEntry）
//  4. 检查事件列表（无事件则跳过）
//  5. 注册 handler 到内部钩子注册表（RegisterInternalHook）
func RegisterPluginHooksFromDir(loader *PluginHookLoader, dir string) PluginHookLoadResult {
	logger := loader.Logger
	if logger == nil {
		logger = slog.Default()
	}

	resolvedDir := ResolveHookDir(loader.Source, dir)
	entries := LoadHookEntriesFromDir(resolvedDir, HookSourcePlugin, loader.PluginID)

	result := PluginHookLoadResult{
		Hooks:  entries,
		Errors: []string{},
	}

	for _, entry := range entries {
		normalized := NormalizePluginHookEntry(loader.PluginID, entry)
		events := normalized.Metadata.Events

		// 无事件 → 跳过
		if len(events) == 0 {
			logger.Warn("[hooks] hook has no events; skipping",
				"plugin", loader.PluginID,
				"hook", entry.Hook.Name)
			result.Skipped++
			continue
		}

		// 检查 handler 文件是否存在
		if normalized.Hook.HandlerPath == "" {
			result.Errors = append(result.Errors,
				fmt.Sprintf("[hooks] Failed to load %s: no handler file", entry.Hook.Name))
			result.Skipped++
			continue
		}

		// Go 端无法动态 import() JS handler —
		// 注册一个桩 handler，记录调用信息用于后续扩展。
		// 当 Go 端 plugin 包建立后，可替换为真实加载逻辑。
		for _, eventKey := range events {
			hookName := normalized.Hook.Name
			pluginID := loader.PluginID
			RegisterInternalHook(eventKey, func(event *InternalHookEvent) error {
				logger.Info("[hooks] plugin hook invoked",
					"plugin", pluginID,
					"hook", hookName,
					"event", event.Type,
					"action", event.Action)
				return nil
			})
		}

		result.Loaded++
	}

	return result
}
