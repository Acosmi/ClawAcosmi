package config

// 运行时配置覆盖 — 对应 src/config/runtime-overrides.ts (77 行)
//
// 提供动态配置覆盖能力，不修改配置文件。
// 配置覆盖在 LoadConfig 流程的最后阶段应用 (applyConfigOverrides)。
// 用于 API/WebSocket 热更新场景。
//
// 依赖: configpath.go (ParseConfigPath, SetConfigValueAtPath 等)

import (
	"encoding/json"
	"sync"
)

// OverrideTree 配置覆盖树
type OverrideTree = map[string]interface{}

// ConfigOverrides 全局配置覆盖管理器（线程安全）
type ConfigOverrides struct {
	mu        sync.RWMutex
	overrides OverrideTree
}

// 全局单例
var globalOverrides = &ConfigOverrides{
	overrides: make(OverrideTree),
}

// GetConfigOverrides 获取当前覆盖树的副本
func GetConfigOverrides() OverrideTree {
	globalOverrides.mu.RLock()
	defer globalOverrides.mu.RUnlock()
	// 返回浅拷贝
	copy := make(OverrideTree, len(globalOverrides.overrides))
	for k, v := range globalOverrides.overrides {
		copy[k] = v
	}
	return copy
}

// ResetConfigOverrides 清空所有覆盖
func ResetConfigOverrides() {
	globalOverrides.mu.Lock()
	defer globalOverrides.mu.Unlock()
	globalOverrides.overrides = make(OverrideTree)
}

// SetConfigOverride 设置一个配置覆盖
// pathRaw: dot-notation 路径 (e.g. "agents.defaults.contextTokens")
func SetConfigOverride(pathRaw string, value interface{}) (bool, string) {
	path, errMsg := ParseConfigPath(pathRaw)
	if errMsg != "" {
		return false, errMsg
	}

	globalOverrides.mu.Lock()
	defer globalOverrides.mu.Unlock()
	SetConfigValueAtPath(globalOverrides.overrides, path, value)
	return true, ""
}

// UnsetConfigOverride 移除一个配置覆盖
// 返回 (ok, removed, error)
func UnsetConfigOverride(pathRaw string) (bool, bool, string) {
	path, errMsg := ParseConfigPath(pathRaw)
	if errMsg != "" {
		return false, false, errMsg
	}

	globalOverrides.mu.Lock()
	defer globalOverrides.mu.Unlock()
	removed := UnsetConfigValueAtPath(globalOverrides.overrides, path)
	return true, removed, ""
}

// ApplyConfigOverrides 将覆盖树合并到配置对象上
// 返回合并后的新配置 (不修改原对象)。
// 对应 TS: applyConfigOverrides(cfg)
func ApplyConfigOverrides(cfg map[string]interface{}) map[string]interface{} {
	globalOverrides.mu.RLock()
	overrides := globalOverrides.overrides
	globalOverrides.mu.RUnlock()

	if len(overrides) == 0 {
		return cfg
	}

	result := mergeOverrides(cfg, overrides)
	if resultMap, ok := result.(map[string]interface{}); ok {
		return resultMap
	}
	// 顶层 overrides 应始终为 map，此处为安全回退
	return cfg
}

// mergeOverrides 递归合并覆盖到基础配置
// 对应 TS: mergeOverrides(base, override) -> unknown
func mergeOverrides(base interface{}, override interface{}) interface{} {
	baseMap, baseOk := base.(map[string]interface{})
	overMap, overOk := override.(map[string]interface{})

	if !baseOk || !overOk {
		// 非 map 场景：override 直接替换 base
		// 对应 TS: return override;
		return override
	}

	// 深拷贝 base
	result := make(map[string]interface{}, len(baseMap))
	for k, v := range baseMap {
		result[k] = v
	}

	// 递归合并 override
	for key, overVal := range overMap {
		if overVal == nil {
			continue
		}
		baseVal, exists := result[key]
		if !exists {
			result[key] = overVal
			continue
		}
		// 如果两边都是 map，递归合并
		if _, bOk := baseVal.(map[string]interface{}); bOk {
			if _, oOk := overVal.(map[string]interface{}); oOk {
				result[key] = mergeOverrides(baseVal, overVal)
				continue
			}
		}
		// 覆盖值直接替换
		result[key] = overVal
	}

	return result
}

// SerializeOverrides 将覆盖树序列化为 JSON（调试用）
func SerializeOverrides() ([]byte, error) {
	globalOverrides.mu.RLock()
	defer globalOverrides.mu.RUnlock()
	return json.MarshalIndent(globalOverrides.overrides, "", "  ")
}
