package plugins

import "sync"

// 运行时全局注册表状态
// 对应 TS: runtime.ts — Symbol.for("openacosmi.pluginRegistryState")
// Go 版：包级变量 + sync.RWMutex

var (
	runtimeMu      sync.RWMutex
	activeRegistry *PluginRegistry
	activeKey      string
)

// SetActivePluginRegistry 设置当前活动注册表
// 对应 TS: runtime.ts setActivePluginRegistry
func SetActivePluginRegistry(registry *PluginRegistry, cacheKey string) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	activeRegistry = registry
	activeKey = cacheKey
}

// GetActivePluginRegistry 获取当前活动注册表（可能为 nil）
// 对应 TS: runtime.ts getActivePluginRegistry
func GetActivePluginRegistry() *PluginRegistry {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return activeRegistry
}

// RequireActivePluginRegistry 获取当前活动注册表，如果不存在则创建空注册表
// 对应 TS: runtime.ts requireActivePluginRegistry
func RequireActivePluginRegistry() *PluginRegistry {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if activeRegistry == nil {
		activeRegistry = NewPluginRegistry(
			&NullPluginRuntime{VersionStr: "0.0.0"},
			PluginLogger{
				Info:  func(_ string) {},
				Warn:  func(_ string) {},
				Error: func(_ string) {},
			},
			nil,
		)
	}
	return activeRegistry
}

// GetActivePluginRegistryKey 获取当前活动注册表的缓存 key
// 对应 TS: runtime.ts getActivePluginRegistryKey
func GetActivePluginRegistryKey() string {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return activeKey
}
