package extensions

// compaction_runtime.go — 紧凑保护运行时注册
// 对应 TS: agents/pi-extensions/compaction-safeguard-runtime.ts (36L)
//
// Session 级运行时参数注册表（WeakMap → Go sync.Map + sessionID key）。
// 允许调用方在 session 启动时设置 maxHistoryShare / contextWindowTokens。

import (
	"sync"
)

// CompactionSafeguardRuntimeValue 紧凑保护运行时值。
type CompactionSafeguardRuntimeValue struct {
	MaxHistoryShare     *float64 `json:"maxHistoryShare,omitempty"`
	ContextWindowTokens *int     `json:"contextWindowTokens,omitempty"`
}

var compactionRuntimeRegistry sync.Map // sessionID → *CompactionSafeguardRuntimeValue

// SetCompactionSafeguardRuntime 设置 session 的紧凑保护运行时参数。
// 对应 TS: setCompactionSafeguardRuntime (WeakMap<object, value>)
// Go 语义: sessionID 字符串 → value
func SetCompactionSafeguardRuntime(sessionID string, value *CompactionSafeguardRuntimeValue) {
	if sessionID == "" {
		return
	}
	if value == nil {
		compactionRuntimeRegistry.Delete(sessionID)
		return
	}
	compactionRuntimeRegistry.Store(sessionID, value)
}

// GetCompactionSafeguardRuntime 获取 session 的紧凑保护运行时参数。
// 对应 TS: getCompactionSafeguardRuntime
func GetCompactionSafeguardRuntime(sessionID string) *CompactionSafeguardRuntimeValue {
	if sessionID == "" {
		return nil
	}
	v, ok := compactionRuntimeRegistry.Load(sessionID)
	if !ok {
		return nil
	}
	return v.(*CompactionSafeguardRuntimeValue)
}

// ClearCompactionSafeguardRuntime 清除 session 的运行时参数。
func ClearCompactionSafeguardRuntime(sessionID string) {
	compactionRuntimeRegistry.Delete(sessionID)
}
