package extensions

// context_pruning_runtime.go — 上下文剪枝运行时注册
// 对应 TS: agents/pi-extensions/context-pruning/runtime.ts (41L)
//
// Session 级上下文剪枝运行时注册表。

import (
	"sync"
)

// ContextPruningRuntimeValue 上下文剪枝运行时值。
type ContextPruningRuntimeValue struct {
	Settings            EffectiveContextPruningSettings `json:"settings"`
	ContextWindowTokens *int                            `json:"contextWindowTokens,omitempty"`
	IsToolPrunable      func(toolName string) bool      `json:"-"`
	LastCacheTouchAt    *int64                          `json:"lastCacheTouchAt,omitempty"`
}

var contextPruningRuntimeRegistry sync.Map // sessionID → *ContextPruningRuntimeValue

// SetContextPruningRuntime 设置 session 的上下文剪枝运行时参数。
// 对应 TS: setContextPruningRuntime (WeakMap<object, value>)
func SetContextPruningRuntime(sessionID string, value *ContextPruningRuntimeValue) {
	if sessionID == "" {
		return
	}
	if value == nil {
		contextPruningRuntimeRegistry.Delete(sessionID)
		return
	}
	contextPruningRuntimeRegistry.Store(sessionID, value)
}

// GetContextPruningRuntime 获取 session 的上下文剪枝运行时参数。
// 对应 TS: getContextPruningRuntime
func GetContextPruningRuntime(sessionID string) *ContextPruningRuntimeValue {
	if sessionID == "" {
		return nil
	}
	v, ok := contextPruningRuntimeRegistry.Load(sessionID)
	if !ok {
		return nil
	}
	return v.(*ContextPruningRuntimeValue)
}

// ClearContextPruningRuntime 清除 session 的运行时参数。
func ClearContextPruningRuntime(sessionID string) {
	contextPruningRuntimeRegistry.Delete(sessionID)
}
