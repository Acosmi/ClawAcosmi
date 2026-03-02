package reply

import (
	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/internal/session"
)

// TS 对照: auto-reply/reply/agent-runner-memory.ts (203L)

// SessionEntry 统一使用 session.SessionEntry（与 TS SessionEntry 1:1 对齐）。
// Phase 10 Window 4: 将类型从 gateway 迁移到独立的 session 包以避免循环导入。
type SessionEntry = session.SessionEntry

// MemoryFlusher 内存冲刷接口（DI 接口）。
// 完整实现依赖 agents/model-fallback 和 agents/pi-embedded，延迟到集成阶段。
type MemoryFlusher interface {
	// RunFlush 执行内存冲刷。返回更新后的 session entry。
	RunFlush(params MemoryFlushParams) (*SessionEntry, error)
}

// MemoryFlushParams 内存冲刷参数。
type MemoryFlushParams struct {
	FollowupRun           FollowupRun
	SessionEntry          *SessionEntry
	SessionKey            string
	StorePath             string
	DefaultModel          string
	AgentCfgContextTokens int
	ResolvedVerboseLevel  autoreply.VerboseLevel
	IsHeartbeat           bool
	FlushSettings         *MemoryFlushSettings // nil = 使用默认配置
}

// StubMemoryFlusher 内存冲刷 stub。
type StubMemoryFlusher struct{}

func (s StubMemoryFlusher) RunFlush(params MemoryFlushParams) (*SessionEntry, error) {
	return params.SessionEntry, nil
}

// RunMemoryFlushIfNeeded 执行内存冲刷（如果需要）。
// TS 对照: agent-runner-memory.ts L27-202
// Phase 9 D5: 集成 ShouldRunMemoryFlush 决策逻辑。
func RunMemoryFlushIfNeeded(flusher MemoryFlusher, params MemoryFlushParams) (*SessionEntry, error) {
	if flusher == nil {
		return params.SessionEntry, nil
	}

	// 决策：是否需要冲刷？
	settings := params.FlushSettings
	if settings == nil {
		settings = ResolveMemoryFlushSettings(nil, nil)
	}
	if settings == nil {
		return params.SessionEntry, nil
	}

	// Heartbeat 不触发冲刷。
	if params.IsHeartbeat {
		return params.SessionEntry, nil
	}

	// 检查 token 阈值。
	entry := params.SessionEntry
	var totalTokens int64
	var compactionCount int
	var memoryFlushCompactionCount int
	if entry != nil {
		totalTokens = entry.TotalTokens
		compactionCount = entry.CompactionCount
		memoryFlushCompactionCount = entry.MemoryFlushCompactionCount
	}

	contextWindowTokens := ResolveMemoryFlushContextWindowTokens(params.AgentCfgContextTokens)

	if !ShouldRunMemoryFlush(ShouldRunMemoryFlushParams{
		TotalTokens:                totalTokens,
		CompactionCount:            compactionCount,
		MemoryFlushCompactionCount: memoryFlushCompactionCount,
		ContextWindowTokens:        contextWindowTokens,
		ReserveTokensFloor:         settings.ReserveTokensFloor,
		SoftThresholdTokens:        settings.SoftThresholdTokens,
	}) {
		return params.SessionEntry, nil
	}

	return flusher.RunFlush(params)
}
