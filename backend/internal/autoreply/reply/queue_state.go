package reply

import (
	"strings"
	"sync"
	"time"
)

// TS 对照: auto-reply/reply/queue/state.ts (77L)
// 队列状态管理：全局 followup 队列 Map + 获取/清理。

// 默认队列参数。
const (
	DefaultQueueDebounceMs = 1000
	DefaultQueueCap        = 20
)

// DefaultQueueDrop 默认丢弃策略。
const DefaultQueueDrop QueueDropPolicy = QueueDropSummarize

// FollowupQueueState 单个 followup 队列的完整状态。
type FollowupQueueState struct {
	mu             sync.RWMutex
	Items          []*FollowupRun
	Draining       bool
	LastEnqueuedAt time.Time
	Mode           QueueMode
	DebounceMs     int
	Cap            int
	DropPolicy     QueueDropPolicy
	DroppedCount   int
	SummaryLines   []string
	LastRun        *FollowupRunParams
}

// 全局 followup 队列表（TS: FOLLOWUP_QUEUES = new Map()）。
var (
	followupQueuesMu sync.RWMutex
	followupQueues   = make(map[string]*FollowupQueueState)
)

// GetFollowupQueue 获取或创建指定 key 的 followup 队列。
// 如果已存在，更新 settings；否则创建新队列。
func GetFollowupQueue(key string, settings QueueSettings) *FollowupQueueState {
	followupQueuesMu.Lock()
	defer followupQueuesMu.Unlock()

	existing, ok := followupQueues[key]
	if ok {
		existing.mu.Lock()
		existing.Mode = settings.Mode
		if settings.DebounceMs != nil {
			v := *settings.DebounceMs
			if v < 0 {
				v = 0
			}
			existing.DebounceMs = v
		}
		if settings.Cap != nil && *settings.Cap > 0 {
			existing.Cap = *settings.Cap
		}
		if settings.DropPolicy != "" {
			existing.DropPolicy = settings.DropPolicy
		}
		existing.mu.Unlock()
		return existing
	}

	debounceMs := DefaultQueueDebounceMs
	if settings.DebounceMs != nil {
		v := *settings.DebounceMs
		if v < 0 {
			v = 0
		}
		debounceMs = v
	}
	cap := DefaultQueueCap
	if settings.Cap != nil && *settings.Cap > 0 {
		cap = *settings.Cap
	}
	drop := DefaultQueueDrop
	if settings.DropPolicy != "" {
		drop = settings.DropPolicy
	}

	created := &FollowupQueueState{
		Items:      nil,
		Mode:       settings.Mode,
		DebounceMs: debounceMs,
		Cap:        cap,
		DropPolicy: drop,
	}
	followupQueues[key] = created
	return created
}

// ClearFollowupQueue 清除指定 key 的 followup 队列，返回被清除的项数。
func ClearFollowupQueue(key string) int {
	cleaned := strings.TrimSpace(key)
	if cleaned == "" {
		return 0
	}

	followupQueuesMu.Lock()
	defer followupQueuesMu.Unlock()

	queue, ok := followupQueues[cleaned]
	if !ok {
		return 0
	}
	queue.mu.Lock()
	cleared := len(queue.Items) + queue.DroppedCount
	queue.Items = nil
	queue.DroppedCount = 0
	queue.SummaryLines = nil
	queue.LastRun = nil
	queue.LastEnqueuedAt = time.Time{}
	queue.mu.Unlock()

	delete(followupQueues, cleaned)
	return cleared
}

// getFollowupQueueRaw 从全局 Map 获取队列（不创建）。
func getFollowupQueueRaw(key string) *FollowupQueueState {
	followupQueuesMu.RLock()
	defer followupQueuesMu.RUnlock()
	return followupQueues[key]
}

// deleteFollowupQueue 从全局 Map 删除队列。
func deleteFollowupQueue(key string) {
	followupQueuesMu.Lock()
	defer followupQueuesMu.Unlock()
	delete(followupQueues, key)
}
