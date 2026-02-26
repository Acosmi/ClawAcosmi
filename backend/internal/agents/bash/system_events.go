// bash/system_events.go — 会话级系统事件队列。
// TS 参考：src/infra/system-events.ts (110L)
//
// 轻量级内存队列，存储人类可读的系统事件，
// 前缀到下一次 prompt。事件是临时的（不持久化），
// 按 sessionKey 分隔。
package bash

import (
	"strings"
	"sync"
	"time"
)

// ---------- 常量 ----------

const maxSystemEvents = 20

// ---------- 类型 ----------

// SystemEvent 单条系统事件。
type SystemEvent struct {
	Text string `json:"text"`
	Ts   int64  `json:"ts"`
}

// SystemEventOptions 入队选项。
type SystemEventOptions struct {
	SessionKey string
	ContextKey string
}

// sessionQueue 会话级事件队列。
type sessionQueue struct {
	queue          []SystemEvent
	lastText       string
	lastContextKey string
}

// ---------- 全局队列 ----------

var (
	eventQueues   = make(map[string]*sessionQueue)
	eventQueuesMu sync.RWMutex
)

// ---------- 内部函数 ----------

func requireSessionKey(key string) string {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	return trimmed
}

func normalizeContextKey(key string) string {
	if key == "" {
		return ""
	}
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

// ---------- 公共 API ----------

// IsSystemEventContextChanged 检查给定 sessionKey 的上下文是否已变化。
// TS 参考: system-events.ts isSystemEventContextChanged L41-49
func IsSystemEventContextChanged(sessionKey, contextKey string) bool {
	key := requireSessionKey(sessionKey)
	if key == "" {
		return false
	}
	normalized := normalizeContextKey(contextKey)

	eventQueuesMu.RLock()
	defer eventQueuesMu.RUnlock()

	existing, ok := eventQueues[key]
	if !ok {
		return normalized != ""
	}
	return normalized != existing.lastContextKey
}

// EnqueueSystemEvent 将事件文本入队到对应 session 的队列。
// TS 参考: system-events.ts enqueueSystemEvent L51-77
func EnqueueSystemEvent(text string, opts SystemEventOptions) {
	key := requireSessionKey(opts.SessionKey)
	if key == "" {
		return
	}
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return
	}

	eventQueuesMu.Lock()
	defer eventQueuesMu.Unlock()

	entry, ok := eventQueues[key]
	if !ok {
		entry = &sessionQueue{
			queue: make([]SystemEvent, 0, 4),
		}
		eventQueues[key] = entry
	}

	entry.lastContextKey = normalizeContextKey(opts.ContextKey)

	// 跳过连续重复
	if entry.lastText == cleaned {
		return
	}
	entry.lastText = cleaned

	entry.queue = append(entry.queue, SystemEvent{
		Text: cleaned,
		Ts:   time.Now().UnixMilli(),
	})

	// 超出容量时移除最老的
	if len(entry.queue) > maxSystemEvents {
		entry.queue = entry.queue[1:]
	}
}

// DrainSystemEventEntries 排空并返回该 session 的所有事件。
// TS 参考: system-events.ts drainSystemEventEntries L79-91
func DrainSystemEventEntries(sessionKey string) []SystemEvent {
	key := requireSessionKey(sessionKey)
	if key == "" {
		return nil
	}

	eventQueuesMu.Lock()
	defer eventQueuesMu.Unlock()

	entry, ok := eventQueues[key]
	if !ok || len(entry.queue) == 0 {
		return nil
	}

	out := make([]SystemEvent, len(entry.queue))
	copy(out, entry.queue)

	// 清理
	delete(eventQueues, key)
	return out
}

// DrainSystemEvents 排空并返回事件文本列表。
// TS 参考: system-events.ts drainSystemEvents L93-95
func DrainSystemEvents(sessionKey string) []string {
	entries := DrainSystemEventEntries(sessionKey)
	if len(entries) == 0 {
		return nil
	}
	texts := make([]string, len(entries))
	for i, e := range entries {
		texts[i] = e.Text
	}
	return texts
}

// PeekSystemEvents 查看（不排空）事件文本。
// TS 参考: system-events.ts peekSystemEvents L97-100
func PeekSystemEvents(sessionKey string) []string {
	key := requireSessionKey(sessionKey)
	if key == "" {
		return nil
	}

	eventQueuesMu.RLock()
	defer eventQueuesMu.RUnlock()

	entry, ok := eventQueues[key]
	if !ok {
		return nil
	}
	texts := make([]string, len(entry.queue))
	for i, e := range entry.queue {
		texts[i] = e.Text
	}
	return texts
}

// HasSystemEvents 检查是否有待处理事件。
// TS 参考: system-events.ts hasSystemEvents L102-105
func HasSystemEvents(sessionKey string) bool {
	key := requireSessionKey(sessionKey)
	if key == "" {
		return false
	}

	eventQueuesMu.RLock()
	defer eventQueuesMu.RUnlock()

	entry, ok := eventQueues[key]
	return ok && len(entry.queue) > 0
}

// ResetSystemEventsForTest 清理所有队列（仅测试用）。
// TS 参考: system-events.ts resetSystemEventsForTest L107-109
func ResetSystemEventsForTest() {
	eventQueuesMu.Lock()
	defer eventQueuesMu.Unlock()
	eventQueues = make(map[string]*sessionQueue)
}
