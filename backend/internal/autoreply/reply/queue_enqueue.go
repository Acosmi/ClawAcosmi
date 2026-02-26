package reply

import (
	"strings"
	"time"
)

// TS 对照: auto-reply/reply/queue/enqueue.ts (70L)
// 入队逻辑：去重 + drop policy + 入队。

// QueueDedupeMode 队列去重模式。
type QueueDedupeMode string

const (
	QueueDedupeMessageID QueueDedupeMode = "message-id"
	QueueDedupePrompt    QueueDedupeMode = "prompt"
	QueueDedupeNone      QueueDedupeMode = "none"
)

// isRunAlreadyQueued 检查 run 是否已在队列中（通过路由 + ID/prompt 去重）。
func isRunAlreadyQueued(run *FollowupRun, items []*FollowupRun, allowPromptFallback bool) bool {
	hasSameRouting := func(item *FollowupRun) bool {
		return item.OriginatingChannel == run.OriginatingChannel &&
			item.OriginatingTo == run.OriginatingTo &&
			item.OriginatingAccountID == run.OriginatingAccountID &&
			item.OriginatingThreadID == run.OriginatingThreadID
	}

	messageID := strings.TrimSpace(run.MessageID)
	if messageID != "" {
		for _, item := range items {
			if strings.TrimSpace(item.MessageID) == messageID && hasSameRouting(item) {
				return true
			}
		}
		return false
	}
	if !allowPromptFallback {
		return false
	}
	for _, item := range items {
		if item.Prompt == run.Prompt && hasSameRouting(item) {
			return true
		}
	}
	return false
}

// EnqueueFollowupRun 将 followup run 入队。
// 返回 true 表示成功入队，false 表示被去重或被 drop policy 拒绝。
func EnqueueFollowupRun(key string, run *FollowupRun, settings QueueSettings, dedupeMode QueueDedupeMode) bool {
	if dedupeMode == "" {
		dedupeMode = QueueDedupeMessageID
	}
	queue := GetFollowupQueue(key, settings)

	queue.mu.Lock()
	defer queue.mu.Unlock()

	// 去重检查
	if dedupeMode != QueueDedupeNone {
		allowPromptFallback := dedupeMode == QueueDedupePrompt
		if isRunAlreadyQueued(run, queue.Items, allowPromptFallback) {
			return false
		}
	}

	queue.LastEnqueuedAt = time.Now()
	queue.LastRun = &run.Run

	// 应用 drop policy
	shouldEnqueue := ApplyQueueDropPolicy(queue, func(item *FollowupRun) string {
		line := strings.TrimSpace(item.SummaryLine)
		if line != "" {
			return line
		}
		return strings.TrimSpace(item.Prompt)
	})
	if !shouldEnqueue {
		return false
	}

	queue.Items = append(queue.Items, run)
	return true
}

// GetFollowupQueueDepth 获取指定 key 的队列深度。
func GetFollowupQueueDepth(key string) int {
	cleaned := strings.TrimSpace(key)
	if cleaned == "" {
		return 0
	}
	queue := getFollowupQueueRaw(cleaned)
	if queue == nil {
		return 0
	}
	queue.mu.RLock()
	defer queue.mu.RUnlock()
	return len(queue.Items)
}
