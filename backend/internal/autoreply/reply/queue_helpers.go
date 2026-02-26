package reply

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// TS 对照: utils/queue-helpers.ts (152L)
// 通用队列辅助函数，被 queue_state / queue_enqueue / queue_drain 使用。

// QueueSummaryState 队列摘要状态（嵌入到 FollowupQueueState）。
type QueueSummaryState struct {
	DropPolicy   QueueDropPolicy
	DroppedCount int
	SummaryLines []string
}

// QueueState 泛型队列状态（用于 ApplyQueueDropPolicy）。
type QueueState[T any] struct {
	QueueSummaryState
	Items []T
	Cap   int
}

// ElideQueueText 截断超长文本，添加 "…" 后缀。
func ElideQueueText(text string, limit int) string {
	if limit <= 0 {
		limit = 140
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return strings.TrimRight(string(runes[:limit-1]), " ") + "…"
}

// BuildQueueSummaryLine 清理空白 + 截断。
func BuildQueueSummaryLine(text string, limit int) string {
	if limit <= 0 {
		limit = 160
	}
	cleaned := collapseWhitespace(text)
	return ElideQueueText(cleaned, limit)
}

// ShouldSkipQueueItem 通过去重函数判定是否跳过。
func ShouldSkipQueueItem[T any](item T, items []T, dedupe func(T, []T) bool) bool {
	if dedupe == nil {
		return false
	}
	return dedupe(item, items)
}

// ApplyQueueDropPolicy 应用溢出丢弃策略。
// 返回 true 表示可以入队，false 表示被拒绝（dropPolicy == "new" 时）。
func ApplyQueueDropPolicy(state *FollowupQueueState, summarize func(*FollowupRun) string) bool {
	cap := state.Cap
	if cap <= 0 || len(state.Items) < cap {
		return true
	}
	if state.DropPolicy == QueueDropNew {
		return false
	}
	// 需要丢弃最旧的项来腾出空间
	dropCount := len(state.Items) - cap + 1
	dropped := make([]*FollowupRun, dropCount)
	copy(dropped, state.Items[:dropCount])
	state.Items = state.Items[dropCount:]

	if state.DropPolicy == QueueDropSummarize {
		for _, item := range dropped {
			state.DroppedCount++
			state.SummaryLines = append(state.SummaryLines, BuildQueueSummaryLine(summarize(item), 160))
		}
		// 限制摘要行数
		limit := cap
		if limit <= 0 {
			limit = 20
		}
		for len(state.SummaryLines) > limit {
			state.SummaryLines = state.SummaryLines[1:]
		}
	}
	return true
}

// WaitForDebounce 等待 debounce 时间窗口。
// 会在 ctx 取消时提前返回。
func WaitForDebounce(ctx context.Context, state *FollowupQueueState) {
	debounceMs := state.DebounceMs
	if debounceMs <= 0 {
		return
	}
	for {
		state.mu.RLock()
		lastEnqueued := state.LastEnqueuedAt
		state.mu.RUnlock()

		since := time.Since(lastEnqueued)
		target := time.Duration(debounceMs) * time.Millisecond
		if since >= target {
			return
		}
		remaining := target - since
		select {
		case <-ctx.Done():
			return
		case <-time.After(remaining):
			// 再次检查（可能在等待期间有新入队）
		}
	}
}

// BuildQueueSummaryPrompt 构建溢出摘要提示文本。
// 如果没有溢出则返回空字符串。
func BuildQueueSummaryPrompt(state *QueueSummaryState, noun string, title string) string {
	if state.DropPolicy != QueueDropSummarize || state.DroppedCount <= 0 {
		return ""
	}
	if title == "" {
		plural := "s"
		if state.DroppedCount == 1 {
			plural = ""
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
	// 消耗掉 droppedCount
	state.DroppedCount = 0
	state.SummaryLines = nil
	return strings.Join(lines, "\n")
}

// BuildCollectPrompt 构建 collect 模式合并文本。
func BuildCollectPrompt(title string, items []*FollowupRun, summary string, renderItem func(*FollowupRun, int) string) string {
	blocks := []string{title}
	if summary != "" {
		blocks = append(blocks, summary)
	}
	for idx, item := range items {
		blocks = append(blocks, renderItem(item, idx))
	}
	return strings.Join(blocks, "\n\n")
}

// HasCrossChannelItems 检测消息列表是否跨多个频道。
func HasCrossChannelItems(items []*FollowupRun, resolveKey func(*FollowupRun) (key string, isCross bool)) bool {
	keys := make(map[string]struct{})
	hasUnkeyed := false

	for _, item := range items {
		key, isCross := resolveKey(item)
		if isCross {
			return true
		}
		if key == "" {
			hasUnkeyed = true
			continue
		}
		keys[key] = struct{}{}
	}

	if len(keys) == 0 {
		return false
	}
	if hasUnkeyed {
		return true
	}
	return len(keys) > 1
}
