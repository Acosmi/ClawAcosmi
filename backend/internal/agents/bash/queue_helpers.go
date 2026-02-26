// bash/queue_helpers.go — 通用队列辅助工具。
// TS 参考：src/utils/queue-helpers.ts (152L)
//
// 提供队列溢出策略、去抖等待、摘要/收集 prompt 构建、
// 跨频道检测等通用队列原语。
package bash

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ---------- 类型 ----------

// QueueDropPolicy 丢弃策略。
type QueueDropPolicy string

const (
	QueueDropSummarize QueueDropPolicy = "summarize"
	QueueDropOld       QueueDropPolicy = "old"
	QueueDropNew       QueueDropPolicy = "new"
)

// QueueSummaryState 队列摘要状态。
// TS 参考: queue-helpers.ts QueueSummaryState L1-5
type QueueSummaryState struct {
	DropPolicy   QueueDropPolicy `json:"dropPolicy"`
	DroppedCount int             `json:"droppedCount"`
	SummaryLines []string        `json:"summaryLines"`
}

// QueueState 通用队列状态（带 interface{} 项）。
// TS 参考: queue-helpers.ts QueueState L9-12
type QueueState struct {
	QueueSummaryState
	Items []interface{} `json:"items"`
	Cap   int           `json:"cap"`
}

// ---------- 文本工具 ----------

// ElideQueueText 截断队列文本到指定长度。
// TS 参考: queue-helpers.ts elideQueueText L14-19
func ElideQueueText(text string, limit int) string {
	if limit <= 0 {
		limit = 140
	}
	if len(text) <= limit {
		return text
	}
	cutoff := limit - 1
	if cutoff < 0 {
		cutoff = 0
	}
	return strings.TrimRight(text[:cutoff], " \t") + "…"
}

// BuildQueueSummaryLine 构建队列摘要行。
// TS 参考: queue-helpers.ts buildQueueSummaryLine L21-24
func BuildQueueSummaryLine(text string, limit int) string {
	if limit <= 0 {
		limit = 160
	}
	// 合并空白
	cleaned := strings.Join(strings.Fields(text), " ")
	return ElideQueueText(cleaned, limit)
}

// ---------- 去重 ----------

// ShouldSkipQueueItem 检查项是否应跳过（去重）。
// TS 参考: queue-helpers.ts shouldSkipQueueItem L26-35
func ShouldSkipQueueItem(item interface{}, items []interface{}, dedupe func(interface{}, []interface{}) bool) bool {
	if dedupe == nil {
		return false
	}
	return dedupe(item, items)
}

// ---------- 溢出策略 ----------

// ApplyQueueDropPolicy 应用队列溢出丢弃策略。
// 返回 true 表示可以继续入队，false 表示丢弃新项（"new" 策略）。
// TS 参考: queue-helpers.ts applyQueueDropPolicy L37-62
func ApplyQueueDropPolicy(queue *QueueState, summarize func(interface{}) string, summaryLimit int) bool {
	cap := queue.Cap
	if cap <= 0 || len(queue.Items) < cap {
		return true
	}

	if queue.DropPolicy == QueueDropNew {
		return false
	}

	// 丢弃最老的直到低于 cap
	dropCount := len(queue.Items) - cap + 1
	if dropCount > len(queue.Items) {
		dropCount = len(queue.Items)
	}
	dropped := queue.Items[:dropCount]
	queue.Items = queue.Items[dropCount:]

	if queue.DropPolicy == QueueDropSummarize {
		for _, item := range dropped {
			queue.DroppedCount++
			queue.SummaryLines = append(queue.SummaryLines, BuildQueueSummaryLine(summarize(item), 160))
		}
		limit := summaryLimit
		if limit <= 0 {
			limit = cap
		}
		for len(queue.SummaryLines) > limit {
			queue.SummaryLines = queue.SummaryLines[1:]
		}
	}

	return true
}

// ---------- 去抖等待 ----------

// WaitForQueueDebounce 等待队列去抖时间到期。
// TS 参考: queue-helpers.ts waitForQueueDebounce L64-83
func WaitForQueueDebounce(debounceMs int, lastEnqueuedAt *int64, mu *sync.Mutex) {
	if debounceMs <= 0 {
		return
	}

	for {
		mu.Lock()
		last := *lastEnqueuedAt
		mu.Unlock()

		since := time.Since(time.UnixMilli(last))
		target := time.Duration(debounceMs) * time.Millisecond
		if since >= target {
			return
		}
		time.Sleep(target - since)
	}
}

// ---------- Prompt 构建 ----------

// BuildQueueSummaryPrompt 构建队列溢出摘要 prompt。
// TS 参考: queue-helpers.ts buildQueueSummaryPrompt L85-107
func BuildQueueSummaryPrompt(state *QueueSummaryState, noun, title string) string {
	if state.DropPolicy != QueueDropSummarize || state.DroppedCount <= 0 {
		return ""
	}

	if title == "" {
		plural := ""
		if state.DroppedCount != 1 {
			plural = "s"
		}
		title = fmt.Sprintf("[Queue overflow] Dropped %d %s%s due to cap.", state.DroppedCount, noun, plural)
	}

	lines := []string{title}
	if len(state.SummaryLines) > 0 {
		lines = append(lines, "Summary:")
		for _, line := range state.SummaryLines {
			lines = append(lines, "- "+line)
		}
	}

	// 重置
	state.DroppedCount = 0
	state.SummaryLines = nil

	return strings.Join(lines, "\n")
}

// BuildCollectPrompt 构建收集模式 prompt。
// TS 参考: queue-helpers.ts buildCollectPrompt L109-123
func BuildCollectPrompt(title string, items []interface{}, summary string, renderItem func(interface{}, int) string) string {
	blocks := []string{title}
	if summary != "" {
		blocks = append(blocks, summary)
	}
	for i, item := range items {
		blocks = append(blocks, renderItem(item, i))
	}
	return strings.Join(blocks, "\n\n")
}

// ---------- 跨频道检测 ----------

// CrossChannelResult 跨频道解析结果。
type CrossChannelResult struct {
	Key   string
	Cross bool
}

// HasCrossChannelItems 检查队列中是否有来自多个频道的项。
// TS 参考: queue-helpers.ts hasCrossChannelItems L125-151
func HasCrossChannelItems(items []interface{}, resolveKey func(interface{}) CrossChannelResult) bool {
	keys := make(map[string]bool)
	hasUnkeyed := false

	for _, item := range items {
		resolved := resolveKey(item)
		if resolved.Cross {
			return true
		}
		if resolved.Key == "" {
			hasUnkeyed = true
			continue
		}
		keys[resolved.Key] = true
	}

	if len(keys) == 0 {
		return false
	}
	if hasUnkeyed {
		return true
	}
	return len(keys) > 1
}
