package extensions

// context_pruning_extension.go — 上下文剪枝扩展入口
// 对应 TS: agents/pi-extensions/context-pruning/extension.ts (42L)
//
// 提供 context 事件处理：检查 TTL 过期后调用 pruner。

import (
	"time"
)

// ContextEvent 上下文事件。
type ContextEvent struct {
	Messages []AgentMessage `json:"messages"`
}

// ContextPruningExtensionResult 扩展处理结果。
type ContextPruningExtensionResult struct {
	Messages []AgentMessage `json:"messages,omitempty"`
	Changed  bool           `json:"changed"`
}

// HandleContextPruningEvent 处理上下文剪枝事件。
// 对应 TS: contextPruningExtension → api.on("context", ...)
func HandleContextPruningEvent(
	event ContextEvent,
	sessionID string,
	contextWindowTokens int,
) *ContextPruningExtensionResult {
	runtime := GetContextPruningRuntime(sessionID)
	if runtime == nil {
		return nil
	}

	// cache-ttl 模式检查
	if runtime.Settings.Mode == PruningModeCacheTTL {
		ttlMs := runtime.Settings.TTLMs
		lastTouch := runtime.LastCacheTouchAt
		if lastTouch == nil || ttlMs <= 0 {
			return nil
		}
		now := time.Now().UnixMilli()
		if ttlMs > 0 && now-*lastTouch < ttlMs {
			return nil
		}
	}

	// 确定 contextWindowTokens
	cwTokens := contextWindowTokens
	if runtime.ContextWindowTokens != nil && *runtime.ContextWindowTokens > 0 {
		cwTokens = *runtime.ContextWindowTokens
	}

	// 执行剪枝
	isToolPrunable := runtime.IsToolPrunable
	if isToolPrunable == nil {
		isToolPrunable = func(string) bool { return true }
	}

	next := PruneContextMessages(event.Messages, runtime.Settings, cwTokens)

	// 检查是否有变化
	if len(next) == len(event.Messages) {
		changed := false
		for i := range next {
			if string(next[i].Content) != string(event.Messages[i].Content) {
				changed = true
				break
			}
		}
		if !changed {
			return nil
		}
	}

	// 更新 lastCacheTouchAt
	if runtime.Settings.Mode == PruningModeCacheTTL {
		now := time.Now().UnixMilli()
		runtime.LastCacheTouchAt = &now
	}

	return &ContextPruningExtensionResult{
		Messages: next,
		Changed:  true,
	}
}
