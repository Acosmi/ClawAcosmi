package reply

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// TS 对照: auto-reply/reply/queue/drain.ts (136L)
// 排干逻辑：goroutine 驱动的异步队列消费。

// ScheduleFollowupDrain 启动 followup 队列排干 goroutine。
// runFollowup 是每条消息的执行回调。
func ScheduleFollowupDrain(key string, runFollowup func(*FollowupRun) error) {
	queue := getFollowupQueueRaw(key)
	if queue == nil {
		return
	}

	queue.mu.Lock()
	if queue.Draining {
		queue.mu.Unlock()
		return
	}
	queue.Draining = true
	queue.mu.Unlock()

	go func() {
		ctx := context.Background()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[followup-drain] panic for key=%s: %v", key, r)
			}

			queue.mu.Lock()
			queue.Draining = false
			hasRemaining := len(queue.Items) > 0 || queue.DroppedCount > 0
			queue.mu.Unlock()

			if !hasRemaining {
				deleteFollowupQueue(key)
			} else {
				// 还有残留 → 递归调度
				ScheduleFollowupDrain(key, runFollowup)
			}
		}()

		forceIndividualCollect := false

		for {
			queue.mu.RLock()
			hasItems := len(queue.Items) > 0 || queue.DroppedCount > 0
			queue.mu.RUnlock()
			if !hasItems {
				break
			}

			// debounce 等待
			WaitForDebounce(ctx, queue)

			queue.mu.Lock()
			mode := queue.Mode
			queue.mu.Unlock()

			if mode == QueueModeCollect {
				if err := drainCollectMode(key, queue, runFollowup, &forceIndividualCollect); err != nil {
					log.Printf("[followup-drain] collect failed for key=%s: %v", key, err)
					return
				}
				continue
			}

			// 非 collect 模式：检查溢出摘要
			queue.mu.Lock()
			summaryState := &QueueSummaryState{
				DropPolicy:   queue.DropPolicy,
				DroppedCount: queue.DroppedCount,
				SummaryLines: queue.SummaryLines,
			}
			summaryPrompt := BuildQueueSummaryPrompt(summaryState, "message", "")
			queue.DroppedCount = summaryState.DroppedCount
			queue.SummaryLines = summaryState.SummaryLines
			lastRun := queue.LastRun
			queue.mu.Unlock()

			if summaryPrompt != "" {
				if lastRun == nil {
					break
				}
				if err := runFollowup(&FollowupRun{
					Prompt:     summaryPrompt,
					Run:        *lastRun,
					EnqueuedAt: time.Now().UnixMilli(),
				}); err != nil {
					log.Printf("[followup-drain] summary run failed for key=%s: %v", key, err)
				}
				continue
			}

			// 逐条处理
			queue.mu.Lock()
			if len(queue.Items) == 0 {
				queue.mu.Unlock()
				break
			}
			next := queue.Items[0]
			queue.Items = queue.Items[1:]
			queue.mu.Unlock()

			if err := runFollowup(next); err != nil {
				log.Printf("[followup-drain] run failed for key=%s: %v", key, err)
			}
		}
	}()
}

// drainCollectMode 处理 collect 模式排干。
func drainCollectMode(
	key string,
	queue *FollowupQueueState,
	runFollowup func(*FollowupRun) error,
	forceIndividualCollect *bool,
) error {
	_ = key

	// 如果已标记为强制逐条（因跨频道），逐条处理
	if *forceIndividualCollect {
		queue.mu.Lock()
		if len(queue.Items) == 0 {
			queue.mu.Unlock()
			return nil
		}
		next := queue.Items[0]
		queue.Items = queue.Items[1:]
		queue.mu.Unlock()
		return runFollowup(next)
	}

	// 检测跨频道
	queue.mu.RLock()
	itemsCopy := make([]*FollowupRun, len(queue.Items))
	copy(itemsCopy, queue.Items)
	queue.mu.RUnlock()

	isCrossChannel := HasCrossChannelItems(itemsCopy, func(item *FollowupRun) (string, bool) {
		channel := item.OriginatingChannel
		to := item.OriginatingTo
		accountID := item.OriginatingAccountID
		threadID := item.OriginatingThreadID

		if channel == "" && to == "" && accountID == "" && threadID == "" {
			return "", false
		}
		if channel == "" || to == "" {
			return "", true // cross
		}
		parts := []string{channel, to}
		if accountID != "" {
			parts = append(parts, accountID)
		} else {
			parts = append(parts, "")
		}
		if threadID != "" {
			parts = append(parts, threadID)
		} else {
			parts = append(parts, "")
		}
		return strings.Join(parts, "|"), false
	})

	if isCrossChannel {
		*forceIndividualCollect = true
		queue.mu.Lock()
		if len(queue.Items) == 0 {
			queue.mu.Unlock()
			return nil
		}
		next := queue.Items[0]
		queue.Items = queue.Items[1:]
		queue.mu.Unlock()
		return runFollowup(next)
	}

	// 同频道 collect：合并所有项
	queue.mu.Lock()
	items := make([]*FollowupRun, len(queue.Items))
	copy(items, queue.Items)
	queue.Items = nil

	summaryState := &QueueSummaryState{
		DropPolicy:   queue.DropPolicy,
		DroppedCount: queue.DroppedCount,
		SummaryLines: queue.SummaryLines,
	}
	summary := BuildQueueSummaryPrompt(summaryState, "message", "")
	queue.DroppedCount = summaryState.DroppedCount
	queue.SummaryLines = summaryState.SummaryLines

	// 获取最后一个 run 的参数
	var run *FollowupRunParams
	if len(items) > 0 {
		run = &items[len(items)-1].Run
	} else if queue.LastRun != nil {
		run = queue.LastRun
	}
	queue.mu.Unlock()

	if run == nil {
		return nil
	}

	// 保留来源频道信息
	var originChannel, originTo, originAccountID, originThreadID string
	for _, item := range items {
		if item.OriginatingChannel != "" && originChannel == "" {
			originChannel = item.OriginatingChannel
		}
		if item.OriginatingTo != "" && originTo == "" {
			originTo = item.OriginatingTo
		}
		if item.OriginatingAccountID != "" && originAccountID == "" {
			originAccountID = item.OriginatingAccountID
		}
		if item.OriginatingThreadID != "" && originThreadID == "" {
			originThreadID = item.OriginatingThreadID
		}
	}

	prompt := BuildCollectPrompt(
		"[Queued messages while agent was busy]",
		items,
		summary,
		func(item *FollowupRun, idx int) string {
			return strings.TrimSpace(fmt.Sprintf("---\nQueued #%d\n%s", idx+1, item.Prompt))
		},
	)

	return runFollowup(&FollowupRun{
		Prompt:               prompt,
		Run:                  *run,
		EnqueuedAt:           time.Now().UnixMilli(),
		OriginatingChannel:   originChannel,
		OriginatingTo:        originTo,
		OriginatingAccountID: originAccountID,
		OriginatingThreadID:  originThreadID,
	})
}
