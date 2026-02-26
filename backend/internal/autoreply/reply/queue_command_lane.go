package reply

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// TS 对照: process/command-queue.ts (161L) + process/lanes.ts (7L)
// + agents/pi-embedded-runner/lanes.ts (16L)
//
// 最小化 in-process 队列，用于序列化命令执行。
// 默认 lane (main) 保留已有行为。额外 lane 允许低风险并行
// （如 cron 任务），避免与主 auto-reply 工作流的 stdin/log 交错。

// CommandLane 命令通道常量。
const (
	LaneMain     = "main"
	LaneCron     = "cron"
	LaneSubagent = "subagent"
	LaneNested   = "nested"
)

// commandLaneEntry 队列中的单个任务。
type commandLaneEntry struct {
	task        func() error
	done        chan error
	enqueuedAt  time.Time
	warnAfterMs int
}

// commandLaneState 单个 lane 的状态。
type commandLaneState struct {
	mu            sync.Mutex
	queue         []*commandLaneEntry
	active        int
	maxConcurrent int
}

var (
	lanesmu sync.RWMutex
	lanes   = make(map[string]*commandLaneState)
)

func getLaneState(lane string) *commandLaneState {
	lanesmu.RLock()
	state, ok := lanes[lane]
	lanesmu.RUnlock()
	if ok {
		return state
	}

	lanesmu.Lock()
	defer lanesmu.Unlock()
	// double-check
	if state, ok = lanes[lane]; ok {
		return state
	}
	state = &commandLaneState{
		maxConcurrent: 1,
	}
	lanes[lane] = state
	return state
}

func cleanLaneName(lane string) string {
	cleaned := strings.TrimSpace(lane)
	if cleaned == "" {
		return LaneMain
	}
	return cleaned
}

// drainLane 尝试泵送 lane 中待执行的任务。
func drainLane(lane string) {
	state := getLaneState(lane)
	state.mu.Lock()
	defer state.mu.Unlock()

	for state.active < state.maxConcurrent && len(state.queue) > 0 {
		entry := state.queue[0]
		state.queue = state.queue[1:]
		waitedMs := time.Since(entry.enqueuedAt).Milliseconds()
		if waitedMs >= int64(entry.warnAfterMs) {
			slog.Warn("lane wait exceeded",
				"lane", lane,
				"waitedMs", waitedMs,
				"queueAhead", len(state.queue),
			)
		}
		state.active++

		go func(e *commandLaneEntry) {
			err := e.task()
			e.done <- err

			state.mu.Lock()
			state.active--
			state.mu.Unlock()

			drainLane(lane)
		}(entry)
	}
}

// SetCommandLaneConcurrency 设置 lane 的最大并发度。
func SetCommandLaneConcurrency(lane string, maxConcurrent int) {
	cleaned := cleanLaneName(lane)
	state := getLaneState(cleaned)
	state.mu.Lock()
	mc := maxConcurrent
	if mc < 1 {
		mc = 1
	}
	state.maxConcurrent = mc
	state.mu.Unlock()
	drainLane(cleaned)
}

// EnqueueCommandInLane 将任务加入指定 lane 的队列。
// 返回任务执行结果的 error。阻塞直到任务完成。
func EnqueueCommandInLane(lane string, task func() error, warnAfterMs int) error {
	cleaned := cleanLaneName(lane)
	if warnAfterMs <= 0 {
		warnAfterMs = 2000
	}
	entry := &commandLaneEntry{
		task:        task,
		done:        make(chan error, 1),
		enqueuedAt:  time.Now(),
		warnAfterMs: warnAfterMs,
	}
	state := getLaneState(cleaned)
	state.mu.Lock()
	state.queue = append(state.queue, entry)
	slog.Debug("lane enqueue",
		"lane", cleaned,
		"totalSize", len(state.queue)+state.active,
	)
	state.mu.Unlock()
	drainLane(cleaned)
	return <-entry.done
}

// ClearCommandLane 清空指定 lane 的等待队列，返回被清除的项数。
// 不影响正在执行的任务。
// TS 对照: clearCommandLane(lane) in command-queue.ts L151-160
func ClearCommandLane(lane string) int {
	cleaned := cleanLaneName(lane)
	lanesmu.RLock()
	state, ok := lanes[cleaned]
	lanesmu.RUnlock()
	if !ok {
		return 0
	}
	state.mu.Lock()
	removed := len(state.queue)
	// 通知所有等待中的任务已被清除
	for _, entry := range state.queue {
		entry.done <- fmt.Errorf("lane cleared: %s", cleaned)
	}
	state.queue = state.queue[:0]
	state.mu.Unlock()
	return removed
}

// GetCommandLaneQueueSize 返回指定 lane 的总任务数（等待 + 执行中）。
func GetCommandLaneQueueSize(lane string) int {
	cleaned := cleanLaneName(lane)
	lanesmu.RLock()
	state, ok := lanes[cleaned]
	lanesmu.RUnlock()
	if !ok {
		return 0
	}
	state.mu.Lock()
	size := len(state.queue) + state.active
	state.mu.Unlock()
	return size
}

// GetTotalCommandLaneQueueSize 返回所有 lane 的总任务数。
func GetTotalCommandLaneQueueSize() int {
	lanesmu.RLock()
	defer lanesmu.RUnlock()
	total := 0
	for _, state := range lanes {
		state.mu.Lock()
		total += len(state.queue) + state.active
		state.mu.Unlock()
	}
	return total
}

// resolveEmbeddedSessionLane 将 session key 转为 lane 名。
// TS 对照: resolveEmbeddedSessionLane(key) in pi-embedded-runner/lanes.ts L13-15
func resolveEmbeddedSessionLane(key string) string {
	cleaned := strings.TrimSpace(key)
	if cleaned == "" {
		cleaned = LaneMain
	}
	if strings.HasPrefix(cleaned, "session:") {
		return cleaned
	}
	return "session:" + cleaned
}
