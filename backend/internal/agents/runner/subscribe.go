package runner

// ============================================================================
// 流式订阅层 — 类型定义 + 状态管理 + 核心辅助
// 对应 TS: pi-embedded-subscribe.ts + handlers (1319L 合计)
// ============================================================================

import (
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

// ---------- 参数 + 回调 ----------

// SubscribeParams 流式订阅参数。
// TS 对照: pi-embedded-subscribe.types.ts → SubscribeEmbeddedPiSessionParams
type SubscribeParams struct {
	RunID          string
	SessionID      string
	BlockReplyMode string // "text_end" | "message_end"

	OnToolResult     func(text string)
	OnBlockReply     func(text string)
	OnBlockFlush     func()
	OnPartial        func(text string)
	OnReasoning      func(chunk ReasoningChunk) // S2
	OnAssistantStart func(index int)            // S7
	OnAgentEvent     func(stream string, data map[string]interface{})
}

// SubscribeResult 流式订阅结果。
type SubscribeResult struct {
	AssistantTexts   []string
	ToolMetas        []ToolMeta
	Usage            *NormalizedUsage
	CompactionCount  int
	MessagingTexts   []string
	MessagingTargets []MessagingToolSend
}

// ToolMeta 工具元数据。
type ToolMeta struct {
	ToolName string `json:"toolName,omitempty"`
	Meta     string `json:"meta,omitempty"`
}

// ---------- 状态 ----------

// SubscribeState 流式订阅状态。
// TS 对照: pi-embedded-subscribe.handlers.types.ts → EmbeddedPiSubscribeState
type SubscribeState struct {
	mu sync.Mutex

	AssistantTexts  []string
	ToolMetas       []ToolMeta
	ToolMetaByID    map[string]string
	ToolSummaryByID map[string]bool
	LastToolError   *ToolErrorEntry

	BlockBuffer string
	BlockState  TagStripState
	DeltaBuffer string

	LastStreamedAssistant  string
	EmittedAssistantUpdate bool
	AssistantMessageIndex  int
	AssistantTextBaseline  int
	SuppressBlockChunks    bool
	LastBlockReplyText     string // S8: block reply 去重

	CompactionInFlight     bool
	PendingCompactionRetry int
	CompactionRetryDone    chan struct{}

	MessagingTexts     []string
	MessagingTextsNorm []string
	MessagingTargets   []MessagingToolSend
	PendingMsgTexts    map[string]string
	PendingMsgTargets  map[string]MessagingToolSend

	usageAcc        *UsageAccumulator
	compactionCount int
}

// ToolErrorEntry 上次工具错误。
type ToolErrorEntry struct {
	ToolName string
	Meta     string
	Error    string
}

// TagStripState think/final 标签剥离状态。
type TagStripState struct {
	Thinking   bool
	Final      bool
	InlineCode bool // 在反引号内 (单反引号)
	FencedCode bool // 在代码围栏内 (三反引号)
}

// ---------- 初始化 ----------

// NewSubscribeState 创建初始状态。
func NewSubscribeState() *SubscribeState {
	return &SubscribeState{
		ToolMetaByID:      make(map[string]string),
		ToolSummaryByID:   make(map[string]bool),
		PendingMsgTexts:   make(map[string]string),
		PendingMsgTargets: make(map[string]MessagingToolSend),
		usageAcc:          NewUsageAccumulator(),
	}
}

// ---------- 辅助方法 ----------

// RecordUsage 记录 usage。
func (s *SubscribeState) RecordUsage(usage *NormalizedUsage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usageAcc.MergeUsage(usage)
}

// GetUsage 获取累计 usage。
func (s *SubscribeState) GetUsage() *NormalizedUsage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.usageAcc.ToNormalizedUsage()
}

// IncrementCompaction 增加 compaction 计数。
func (s *SubscribeState) IncrementCompaction() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactionCount++
}

// GetCompactionCount 获取 compaction 计数。
func (s *SubscribeState) GetCompactionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.compactionCount
}

// ResetForCompactionRetry 重置状态以进行 compaction 重试。
func (s *SubscribeState) ResetForCompactionRetry() {
	s.AssistantTexts = nil
	s.ToolMetas = nil
	s.BlockBuffer = ""
	s.DeltaBuffer = ""
	s.BlockState = TagStripState{}
	s.LastStreamedAssistant = ""
	s.EmittedAssistantUpdate = false
	s.SuppressBlockChunks = false
	s.MessagingTexts = nil
	s.MessagingTextsNorm = nil
	s.MessagingTargets = nil
	s.PendingMsgTexts = make(map[string]string)
	s.PendingMsgTargets = make(map[string]MessagingToolSend)
}

// ResetAssistantMessage 每个新 assistant 消息重置。
func (s *SubscribeState) ResetAssistantMessage(nextBaseline int) {
	s.DeltaBuffer = ""
	s.BlockBuffer = ""
	s.BlockState = TagStripState{}
	s.LastStreamedAssistant = ""
	s.EmittedAssistantUpdate = false
	s.AssistantTextBaseline = nextBaseline
	s.SuppressBlockChunks = false
	s.LastBlockReplyText = ""
}

// BuildResult 构建最终结果。
func (s *SubscribeState) BuildResult() SubscribeResult {
	return SubscribeResult{
		AssistantTexts:   s.AssistantTexts,
		ToolMetas:        s.ToolMetas,
		Usage:            s.GetUsage(),
		CompactionCount:  s.GetCompactionCount(),
		MessagingTexts:   s.MessagingTexts,
		MessagingTargets: s.MessagingTargets,
	}
}

// ---------- Tag Stripping ----------

// thinkOpenRE <think> 标签。
var thinkOpenRE = regexp.MustCompile(`(?i)<think>`)
var thinkCloseRE = regexp.MustCompile(`(?i)</think>`)
var finalOpenRE = regexp.MustCompile(`(?i)<final>`)
var finalCloseRE = regexp.MustCompile(`(?i)</final>`)

// StripBlockTags 从文本中移除 <think>/<final> 标签及其内容。
// 增强: 跟踪反引号状态，在行内代码或代码围栏内不剥离标签。
// TS 对照: pi-embedded-subscribe.ts → stripBlockTags() + InlineCodeState
func StripBlockTags(text string, state *TagStripState) string {
	if state == nil {
		state = &TagStripState{}
	}

	var result strings.Builder
	remaining := text

	for len(remaining) > 0 {
		// --- 反引号跟踪 ---
		if !state.Thinking && !state.Final {
			tripleIdx := strings.Index(remaining, "```")
			singleIdx := strings.Index(remaining, "`")

			// 优先检查三反引号
			if tripleIdx == 0 {
				state.FencedCode = !state.FencedCode
				result.WriteString("```")
				remaining = remaining[3:]
				continue
			}

			// 在代码围栏内，不做标签处理
			if state.FencedCode {
				if tripleIdx > 0 {
					result.WriteString(remaining[:tripleIdx+3])
					remaining = remaining[tripleIdx+3:]
					state.FencedCode = false
					continue
				}
				// 未找到关闭的 ```，全部输出
				result.WriteString(remaining)
				break
			}

			// 在行内代码内，找关闭的反引号
			if state.InlineCode {
				if singleIdx >= 0 {
					result.WriteString(remaining[:singleIdx+1])
					remaining = remaining[singleIdx+1:]
					state.InlineCode = false
					continue
				}
				result.WriteString(remaining)
				break
			}

			// 检测新的反引号开始（非三反引号）
			if singleIdx == 0 && tripleIdx != 0 {
				state.InlineCode = true
				result.WriteString("`")
				remaining = remaining[1:]
				continue
			}
		}

		if state.Thinking {
			idx := thinkCloseRE.FindStringIndex(remaining)
			if idx == nil {
				break // 剩余全在 think 块中，丢弃
			}
			state.Thinking = false
			remaining = remaining[idx[1]:]
			continue
		}

		if state.Final {
			idx := finalCloseRE.FindStringIndex(remaining)
			if idx == nil {
				break // final 块未关闭，丢弃剩余
			}
			state.Final = false
			remaining = remaining[idx[1]:]
			continue
		}

		// 查找下一个标签
		thinkIdx := thinkOpenRE.FindStringIndex(remaining)
		finalIdx := finalOpenRE.FindStringIndex(remaining)

		nextTag := -1
		isThink := false
		if thinkIdx != nil {
			nextTag = thinkIdx[0]
			isThink = true
		}
		if finalIdx != nil && (nextTag == -1 || finalIdx[0] < nextTag) {
			nextTag = finalIdx[0]
			isThink = false
		}

		if nextTag == -1 {
			result.WriteString(remaining)
			break
		}

		// 输出标签前的内容
		result.WriteString(remaining[:nextTag])
		if isThink {
			state.Thinking = true
			remaining = remaining[thinkIdx[1]:]
		} else {
			state.Final = true
			remaining = remaining[finalIdx[1]:]
		}
	}

	return result.String()
}

// NormalizeTextForComparison 标准化文本用于比较。
func NormalizeTextForComparison(text string) string {
	return strings.TrimSpace(strings.ToLower(text))
}

// ---------- 日志辅助 ----------

var subscribeLog = slog.Default().With("subsystem", "subscribe")
