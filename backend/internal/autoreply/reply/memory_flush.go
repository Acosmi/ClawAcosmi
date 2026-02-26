package reply

// TS 对照: auto-reply/reply/memory-flush.ts (106L)
// 内存冲刷决策逻辑：判断是否需要在 compaction 前执行 memory flush。

const (
	// DefaultMemoryFlushSoftTokens 软阈值（距上限的安全余量）。
	DefaultMemoryFlushSoftTokens = 4000

	// DefaultMemoryFlushPrompt 默认冲刷提示。
	DefaultMemoryFlushPrompt = "Pre-compaction memory flush. " +
		"Store durable memories now (use memory/YYYY-MM-DD.md; create memory/ if needed). " +
		"If nothing to store, reply with <no-reply>."

	// DefaultMemoryFlushSystemPrompt 默认冲刷系统提示。
	DefaultMemoryFlushSystemPrompt = "Pre-compaction memory flush turn. " +
		"The session is near auto-compaction; capture durable memories to disk. " +
		"You may reply, but usually <no-reply> is correct."

	// DefaultCompactionReserveTokensFloor 压缩保留 token 下限。
	DefaultCompactionReserveTokensFloor = 4000

	// DefaultContextTokens 默认上下文窗口 token 数。
	DefaultContextTokens = 128_000
)

// MemoryFlushSettings 内存冲刷配置。
// TS 对照: memory-flush.ts MemoryFlushSettings
type MemoryFlushSettings struct {
	Enabled             bool
	SoftThresholdTokens int
	Prompt              string
	SystemPrompt        string
	ReserveTokensFloor  int
}

// MemoryFlushConfig 从用户配置读取的冲刷设置（对应 agents.defaults.compaction.memoryFlush）。
type MemoryFlushConfig struct {
	Enabled             *bool  `json:"enabled,omitempty"`
	SoftThresholdTokens *int   `json:"softThresholdTokens,omitempty"`
	Prompt              string `json:"prompt,omitempty"`
	SystemPrompt        string `json:"systemPrompt,omitempty"`
}

// CompactionConfig 压缩配置（对应 agents.defaults.compaction）。
type CompactionConfig struct {
	MemoryFlush        *MemoryFlushConfig `json:"memoryFlush,omitempty"`
	ReserveTokensFloor *int               `json:"reserveTokensFloor,omitempty"`
}

// ResolveMemoryFlushSettings 从配置解析冲刷设置。
// 返回 nil 表示冲刷已禁用。
// TS 对照: memory-flush.ts resolveMemoryFlushSettings
func ResolveMemoryFlushSettings(flushCfg *MemoryFlushConfig, reserveFloor *int) *MemoryFlushSettings {
	enabled := true
	if flushCfg != nil && flushCfg.Enabled != nil {
		enabled = *flushCfg.Enabled
	}
	if !enabled {
		return nil
	}

	softThreshold := DefaultMemoryFlushSoftTokens
	if flushCfg != nil && flushCfg.SoftThresholdTokens != nil && *flushCfg.SoftThresholdTokens >= 0 {
		softThreshold = *flushCfg.SoftThresholdTokens
	}

	prompt := DefaultMemoryFlushPrompt
	if flushCfg != nil && flushCfg.Prompt != "" {
		prompt = flushCfg.Prompt
	}
	prompt = ensureNoReplyHint(prompt)

	systemPrompt := DefaultMemoryFlushSystemPrompt
	if flushCfg != nil && flushCfg.SystemPrompt != "" {
		systemPrompt = flushCfg.SystemPrompt
	}
	systemPrompt = ensureNoReplyHint(systemPrompt)

	reserveTokensFloor := DefaultCompactionReserveTokensFloor
	if reserveFloor != nil && *reserveFloor >= 0 {
		reserveTokensFloor = *reserveFloor
	}

	return &MemoryFlushSettings{
		Enabled:             true,
		SoftThresholdTokens: softThreshold,
		Prompt:              prompt,
		SystemPrompt:        systemPrompt,
		ReserveTokensFloor:  reserveTokensFloor,
	}
}

const silentReplyToken = "<no-reply>"

func ensureNoReplyHint(text string) string {
	for i := range text {
		if text[i:] == silentReplyToken || (len(text[i:]) >= len(silentReplyToken) && text[i:i+len(silentReplyToken)] == silentReplyToken) {
			return text
		}
	}
	return text + "\n\nIf no user-visible reply is needed, start with " + silentReplyToken + "."
}

// ShouldRunMemoryFlushParams 冲刷判断参数。
type ShouldRunMemoryFlushParams struct {
	TotalTokens                int64
	CompactionCount            int
	MemoryFlushCompactionCount int
	ContextWindowTokens        int
	ReserveTokensFloor         int
	SoftThresholdTokens        int
}

// ShouldRunMemoryFlush 判断是否应执行内存冲刷。
// TS 对照: memory-flush.ts shouldRunMemoryFlush
func ShouldRunMemoryFlush(params ShouldRunMemoryFlushParams) bool {
	if params.TotalTokens <= 0 {
		return false
	}

	contextWindow := params.ContextWindowTokens
	if contextWindow < 1 {
		contextWindow = 1
	}
	reserveTokens := params.ReserveTokensFloor
	if reserveTokens < 0 {
		reserveTokens = 0
	}
	softThreshold := params.SoftThresholdTokens
	if softThreshold < 0 {
		softThreshold = 0
	}

	threshold := contextWindow - reserveTokens - softThreshold
	if threshold <= 0 {
		return false
	}

	if params.TotalTokens < int64(threshold) {
		return false
	}

	// 如果已对当前 compaction count 进行过冲刷，则跳过。
	if params.MemoryFlushCompactionCount == params.CompactionCount {
		return false
	}

	return true
}

// ResolveMemoryFlushContextWindowTokens 解析内存冲刷使用的上下文窗口 token 数。
// TS 对照: memory-flush.ts resolveMemoryFlushContextWindowTokens
func ResolveMemoryFlushContextWindowTokens(agentCfgContextTokens int) int {
	if agentCfgContextTokens > 0 {
		return agentCfgContextTokens
	}
	return DefaultContextTokens
}
