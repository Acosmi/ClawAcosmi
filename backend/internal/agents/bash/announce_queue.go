// bash/announce_queue.go — 子代理公告队列（全量移植）。
// TS 参考：src/agents/subagent-announce-queue.ts (192L)
//
// 管理子代理完成公告的排队、去抖、汇总和投递。
// 按 key 维护独立队列实例，支持 collect/individual/summarize 模式。
package bash

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ---------- 常量 ----------

const (
	defaultDebounceMs  = 1000
	defaultAnnounceCap = 20
)

// ---------- 类型 ----------

// AnnounceQueueItem 队列条目。
// TS 参考: subagent-announce-queue.ts AnnounceQueueItem L16-23
type AnnounceQueueItem struct {
	Prompt      string           `json:"prompt"`
	SummaryLine string           `json:"summaryLine,omitempty"`
	EnqueuedAt  int64            `json:"enqueuedAt"`
	SessionKey  string           `json:"sessionKey"`
	Origin      *DeliveryContext `json:"origin,omitempty"`
	OriginKey   string           `json:"originKey,omitempty"`
}

// AnnounceQueueSettings 队列设置。
// TS 参考: subagent-announce-queue.ts AnnounceQueueSettings L25-30
type AnnounceQueueSettings struct {
	Mode       QueueDropPolicy `json:"mode"` // collect | individual | summarize
	DebounceMs int             `json:"debounceMs,omitempty"`
	Cap        int             `json:"cap,omitempty"`
	DropPolicy QueueDropPolicy `json:"dropPolicy,omitempty"`
}

// AnnounceQueueMode 队列模式（复用 QueueDropPolicy 的名字空间避免混淆）。
type AnnounceQueueMode string

const (
	AnnounceModeCollect    AnnounceQueueMode = "collect"
	AnnounceModeIndividual AnnounceQueueMode = "individual"
	AnnounceModeSummarize  AnnounceQueueMode = "summarize"
)

// AnnounceSendFunc 投递回调。
type AnnounceSendFunc func(item AnnounceQueueItem) error

// ---------- 队列状态 ----------

// announceQueueState 单个 key 的队列状态。
// TS 参考: subagent-announce-queue.ts AnnounceQueueState L32-43
type announceQueueState struct {
	mu             sync.Mutex
	items          []AnnounceQueueItem
	draining       bool
	lastEnqueuedAt int64
	mode           AnnounceQueueMode
	debounceMs     int
	cap            int
	dropPolicy     QueueDropPolicy
	droppedCount   int
	summaryLines   []string
	send           AnnounceSendFunc
}

// ---------- 全局队列注册表 ----------

var (
	announceQueues   = make(map[string]*announceQueueState)
	announceQueuesMu sync.Mutex
)

// getAnnounceQueue 获取或创建指定 key 的队列。
// TS 参考: subagent-announce-queue.ts getAnnounceQueue L47-81
func getAnnounceQueue(key string, settings AnnounceQueueSettings, send AnnounceSendFunc) *announceQueueState {
	announceQueuesMu.Lock()
	defer announceQueuesMu.Unlock()

	if existing, ok := announceQueues[key]; ok {
		existing.mu.Lock()
		existing.mode = AnnounceQueueMode(settings.Mode)
		if settings.DebounceMs > 0 {
			existing.debounceMs = settings.DebounceMs
		}
		if settings.Cap > 0 {
			existing.cap = settings.Cap
		}
		if settings.DropPolicy != "" {
			existing.dropPolicy = settings.DropPolicy
		}
		existing.send = send
		existing.mu.Unlock()
		return existing
	}

	debounceMs := settings.DebounceMs
	if debounceMs <= 0 {
		debounceMs = defaultDebounceMs
	}
	cap := settings.Cap
	if cap <= 0 {
		cap = defaultAnnounceCap
	}
	dropPolicy := settings.DropPolicy
	if dropPolicy == "" {
		dropPolicy = QueueDropSummarize
	}
	mode := AnnounceQueueMode(settings.Mode)
	if mode == "" {
		mode = AnnounceModeCollect
	}

	created := &announceQueueState{
		items:      make([]AnnounceQueueItem, 0),
		mode:       mode,
		debounceMs: debounceMs,
		cap:        cap,
		dropPolicy: dropPolicy,
		send:       send,
	}
	announceQueues[key] = created
	return created
}

// ---------- 排水循环 ----------

// scheduleAnnounceDrain 启动异步排水 goroutine。
// TS 参考: subagent-announce-queue.ts scheduleAnnounceDrain L83-163
func scheduleAnnounceDrain(key string) {
	announceQueuesMu.Lock()
	queue, ok := announceQueues[key]
	announceQueuesMu.Unlock()
	if !ok {
		return
	}

	queue.mu.Lock()
	if queue.draining {
		queue.mu.Unlock()
		return
	}
	queue.draining = true
	queue.mu.Unlock()

	go func() {
		defer func() {
			queue.mu.Lock()
			queue.draining = false

			// 检查是否还有残余
			remaining := len(queue.items) > 0 || queue.droppedCount > 0
			queue.mu.Unlock()

			if remaining {
				scheduleAnnounceDrain(key)
			} else {
				// 清理空队列
				announceQueuesMu.Lock()
				queue.mu.Lock()
				if len(queue.items) == 0 && queue.droppedCount == 0 {
					delete(announceQueues, key)
				}
				queue.mu.Unlock()
				announceQueuesMu.Unlock()
			}
		}()

		forceIndividualCollect := false

		for {
			queue.mu.Lock()
			hasWork := len(queue.items) > 0 || queue.droppedCount > 0
			queue.mu.Unlock()
			if !hasWork {
				break
			}

			// 去抖等待
			waitForDebounce(queue)

			queue.mu.Lock()
			mode := queue.mode

			// ---------- collect 模式 ----------
			if mode == AnnounceModeCollect {
				if forceIndividualCollect {
					if len(queue.items) == 0 {
						queue.mu.Unlock()
						break
					}
					next := queue.items[0]
					queue.items = queue.items[1:]
					sendFn := queue.send
					queue.mu.Unlock()
					_ = sendFn(next)
					continue
				}

				// 跨频道检测
				items := queue.items
				isCrossChannel := hasCrossChannelItemsAnnounce(items)
				if isCrossChannel {
					forceIndividualCollect = true
					if len(queue.items) == 0 {
						queue.mu.Unlock()
						break
					}
					next := queue.items[0]
					queue.items = queue.items[1:]
					sendFn := queue.send
					queue.mu.Unlock()
					_ = sendFn(next)
					continue
				}

				// 批量收集
				collected := make([]AnnounceQueueItem, len(queue.items))
				copy(collected, queue.items)
				queue.items = queue.items[:0]

				// 构建摘要 prompt
				summaryPrompt := buildAnnounceSummaryPrompt(queue)
				sendFn := queue.send
				queue.mu.Unlock()

				if len(collected) == 0 {
					break
				}

				// 构建 collect prompt
				prompt := buildAnnounceCollectPrompt(collected, summaryPrompt)
				last := collected[len(collected)-1]
				last.Prompt = prompt
				_ = sendFn(last)
				continue
			}

			// ---------- individual / summarize 模式 ----------

			// 先处理摘要
			summaryPrompt := buildAnnounceSummaryPrompt(queue)
			if summaryPrompt != "" {
				if len(queue.items) == 0 {
					queue.mu.Unlock()
					break
				}
				next := queue.items[0]
				queue.items = queue.items[1:]
				sendFn := queue.send
				queue.mu.Unlock()
				next.Prompt = summaryPrompt
				_ = sendFn(next)
				continue
			}

			if len(queue.items) == 0 {
				queue.mu.Unlock()
				break
			}
			next := queue.items[0]
			queue.items = queue.items[1:]
			sendFn := queue.send
			queue.mu.Unlock()
			_ = sendFn(next)
		}
	}()
}

// ---------- 公共 API ----------

// EnqueueAnnounce 将公告条目入队。
// TS 参考: subagent-announce-queue.ts enqueueAnnounce L166-191
func EnqueueAnnounce(key string, item AnnounceQueueItem, settings AnnounceQueueSettings, send AnnounceSendFunc) bool {
	queue := getAnnounceQueue(key, settings, send)

	queue.mu.Lock()
	queue.lastEnqueuedAt = time.Now().UnixMilli()

	// 应用丢弃策略
	shouldEnqueue := applyAnnounceDropPolicy(queue)
	if !shouldEnqueue {
		needDrain := queue.dropPolicy == QueueDropNew
		queue.mu.Unlock()
		if needDrain {
			scheduleAnnounceDrain(key)
		}
		return false
	}

	// 规范化来源
	if item.Origin != nil {
		item.Origin = NormalizeDeliveryContext(item.Origin)
		item.OriginKey = DeliveryContextKey(item.Origin)
	}

	queue.items = append(queue.items, item)
	queue.mu.Unlock()

	scheduleAnnounceDrain(key)
	return true
}

// ---------- 内部辅助 ----------

// waitForDebounce 等待去抖时间到期。
func waitForDebounce(queue *announceQueueState) {
	queue.mu.Lock()
	debounceMs := queue.debounceMs
	lastAt := queue.lastEnqueuedAt
	queue.mu.Unlock()

	if debounceMs <= 0 {
		return
	}

	for {
		since := time.Since(time.UnixMilli(lastAt))
		target := time.Duration(debounceMs) * time.Millisecond
		if since >= target {
			return
		}
		time.Sleep(target - since)

		queue.mu.Lock()
		lastAt = queue.lastEnqueuedAt
		queue.mu.Unlock()
	}
}

// applyAnnounceDropPolicy 应用丢弃策略（调用时 queue.mu 必须已锁定）。
func applyAnnounceDropPolicy(queue *announceQueueState) bool {
	if queue.cap <= 0 || len(queue.items) < queue.cap {
		return true
	}

	if queue.dropPolicy == QueueDropNew {
		return false
	}

	dropCount := len(queue.items) - queue.cap + 1
	if dropCount > len(queue.items) {
		dropCount = len(queue.items)
	}
	dropped := queue.items[:dropCount]
	queue.items = queue.items[dropCount:]

	if queue.dropPolicy == QueueDropSummarize {
		for _, item := range dropped {
			queue.droppedCount++
			line := strings.TrimSpace(item.SummaryLine)
			if line == "" {
				line = strings.TrimSpace(item.Prompt)
			}
			queue.summaryLines = append(queue.summaryLines, BuildQueueSummaryLine(line, 160))
		}
		limit := queue.cap
		if limit <= 0 {
			limit = defaultAnnounceCap
		}
		for len(queue.summaryLines) > limit {
			queue.summaryLines = queue.summaryLines[1:]
		}
	}

	return true
}

// buildAnnounceSummaryPrompt 构建摘要 prompt（调用时 queue.mu 必须已锁定）。
func buildAnnounceSummaryPrompt(queue *announceQueueState) string {
	if queue.dropPolicy != QueueDropSummarize || queue.droppedCount <= 0 {
		return ""
	}

	plural := ""
	if queue.droppedCount != 1 {
		plural = "s"
	}
	title := fmt.Sprintf("[Queue overflow] Dropped %d announce%s due to cap.", queue.droppedCount, plural)

	lines := []string{title}
	if len(queue.summaryLines) > 0 {
		lines = append(lines, "Summary:")
		for _, line := range queue.summaryLines {
			lines = append(lines, "- "+line)
		}
	}

	queue.droppedCount = 0
	queue.summaryLines = nil

	return strings.Join(lines, "\n")
}

// buildAnnounceCollectPrompt 构建 collect 模式的合并 prompt。
func buildAnnounceCollectPrompt(items []AnnounceQueueItem, summary string) string {
	blocks := []string{"[Queued announce messages while agent was busy]"}
	if summary != "" {
		blocks = append(blocks, summary)
	}
	for i, item := range items {
		blocks = append(blocks, fmt.Sprintf("---\nQueued #%d\n%s", i+1, strings.TrimSpace(item.Prompt)))
	}
	return strings.Join(blocks, "\n\n")
}

// hasCrossChannelItemsAnnounce 检测公告项是否来自多个频道。
func hasCrossChannelItemsAnnounce(items []AnnounceQueueItem) bool {
	keys := make(map[string]bool)
	hasUnkeyed := false

	for _, item := range items {
		if item.Origin == nil {
			continue
		}
		if item.OriginKey == "" {
			hasUnkeyed = true
			continue
		}
		keys[item.OriginKey] = true
	}

	if len(keys) == 0 {
		return false
	}
	if hasUnkeyed {
		return true
	}
	return len(keys) > 1
}

// ---------- 跨频道检测（兼容旧接口）----------

// IsCrossChannel 检测消息来源和请求者是否不在同一频道。
func IsCrossChannel(sourceCtx, requesterCtx *DeliveryContext) bool {
	if sourceCtx == nil || requesterCtx == nil {
		return false
	}
	if sourceCtx.Platform != requesterCtx.Platform {
		return true
	}
	if sourceCtx.ChannelID != "" && requesterCtx.ChannelID != "" {
		return sourceCtx.ChannelID != requesterCtx.ChannelID
	}
	return false
}

// ResetAnnounceQueuesForTest 清除全部队列（仅测试用）。
func ResetAnnounceQueuesForTest() {
	announceQueuesMu.Lock()
	defer announceQueuesMu.Unlock()
	announceQueues = make(map[string]*announceQueueState)
}
