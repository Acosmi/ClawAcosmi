package compaction

import (
	"math"
	"strings"
)

// ---------- 历史压缩 ----------

// TS 参考: src/agents/compaction.ts (374 行)
// 核心: token 估算、分片、裁剪。LLM 摘要调用仅定义接口，具体实现留待 PI Runner 集成。

const (
	DefaultSummaryFallback = "No prior history."
	DefaultParts           = 2
)

// AgentMessage 代理消息（简化版，完整定义在 PI Agent Core）。
type AgentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// EstimateTokens 粗略估算单条消息的 token 数。
// 使用 ~4 字符/token 的经验公式。
func EstimateTokens(content string) int {
	return max(1, len(content)/4)
}

// EstimateMessagesTokens 估算消息列表的总 token 数。
func EstimateMessagesTokens(messages []AgentMessage) int {
	total := 0
	for _, m := range messages {
		total += EstimateTokens(m.Content)
	}
	return total
}

// NormalizeParts 规范化分片数。
func NormalizeParts(parts, messageCount int) int {
	if parts <= 0 {
		parts = DefaultParts
	}
	if parts > messageCount {
		return messageCount
	}
	return parts
}

// SplitMessagesByTokenShare 按 token 份额平均分片。
// TS 参考: compaction.ts → splitMessagesByTokenShare()
func SplitMessagesByTokenShare(messages []AgentMessage, parts int) [][]AgentMessage {
	parts = NormalizeParts(parts, len(messages))
	if parts <= 1 || len(messages) == 0 {
		return [][]AgentMessage{messages}
	}

	totalTokens := EstimateMessagesTokens(messages)
	if totalTokens == 0 {
		return [][]AgentMessage{messages}
	}

	targetPerPart := float64(totalTokens) / float64(parts)
	var result [][]AgentMessage
	var current []AgentMessage
	currentTokens := 0

	for _, msg := range messages {
		msgTokens := EstimateTokens(msg.Content)
		current = append(current, msg)
		currentTokens += msgTokens

		// 当前分片达到目标且还有余量
		if float64(currentTokens) >= targetPerPart && len(result) < parts-1 {
			result = append(result, current)
			current = nil
			currentTokens = 0
		}
	}
	if len(current) > 0 {
		result = append(result, current)
	}

	return result
}

// ChunkMessagesByMaxTokens 按最大 token 数分块。
// TS 参考: compaction.ts → chunkMessagesByMaxTokens()
func ChunkMessagesByMaxTokens(messages []AgentMessage, maxTokens int) [][]AgentMessage {
	if maxTokens <= 0 || len(messages) == 0 {
		return [][]AgentMessage{messages}
	}

	var result [][]AgentMessage
	var current []AgentMessage
	currentTokens := 0

	for _, msg := range messages {
		msgTokens := EstimateTokens(msg.Content)

		// 单条消息超限，独立成块
		if msgTokens > maxTokens && len(current) > 0 {
			result = append(result, current)
			current = nil
			currentTokens = 0
		}

		if currentTokens+msgTokens > maxTokens && len(current) > 0 {
			result = append(result, current)
			current = nil
			currentTokens = 0
		}

		current = append(current, msg)
		currentTokens += msgTokens
	}

	if len(current) > 0 {
		result = append(result, current)
	}

	return result
}

// ComputeAdaptiveChunkRatio 计算自适应分块比例。
// TS 参考: compaction.ts → computeAdaptiveChunkRatio()
func ComputeAdaptiveChunkRatio(messages []AgentMessage, contextWindow int) float64 {
	if len(messages) == 0 || contextWindow <= 0 {
		return 0.25
	}
	totalTokens := EstimateMessagesTokens(messages)
	avgTokens := float64(totalTokens) / float64(len(messages))
	ratio := avgTokens / float64(contextWindow)

	// 限制在 [0.1, 0.5] 范围
	if ratio < 0.1 {
		return 0.1
	}
	if ratio > 0.5 {
		return 0.5
	}
	return math.Round(ratio*100) / 100
}

// IsOversizedForSummary 检查单条消息是否过大无法安全摘要。
func IsOversizedForSummary(msg AgentMessage, contextWindow int) bool {
	if contextWindow <= 0 {
		return false
	}
	msgTokens := EstimateTokens(msg.Content)
	return msgTokens > contextWindow/2
}

// PruneResult 裁剪结果。
type PruneResult struct {
	Messages        []AgentMessage
	DroppedChunks   int
	DroppedMessages int
	DroppedTokens   int
	KeptTokens      int
	BudgetTokens    int
}

// PruneHistoryForContextShare 在上下文预算内裁剪历史消息。
// TS 参考: compaction.ts → pruneHistoryForContextShare()
func PruneHistoryForContextShare(messages []AgentMessage, maxContextTokens int, maxHistoryShare float64, parts int) PruneResult {
	if maxHistoryShare <= 0 {
		maxHistoryShare = 0.5
	}
	budget := int(math.Floor(float64(maxContextTokens) * maxHistoryShare))

	totalTokens := EstimateMessagesTokens(messages)
	if totalTokens <= budget {
		return PruneResult{
			Messages:     messages,
			KeptTokens:   totalTokens,
			BudgetTokens: budget,
		}
	}

	// 从末尾保留，丢弃旧消息
	kept := make([]AgentMessage, 0, len(messages))
	keptTokens := 0
	dropIdx := -1

	for i := len(messages) - 1; i >= 0; i-- {
		msgTokens := EstimateTokens(messages[i].Content)
		if keptTokens+msgTokens > budget {
			dropIdx = i
			break
		}
		kept = append([]AgentMessage{messages[i]}, kept...)
		keptTokens += msgTokens
	}

	droppedMessages := 0
	droppedTokens := 0
	if dropIdx >= 0 {
		droppedMessages = dropIdx + 1
		for i := 0; i <= dropIdx; i++ {
			droppedTokens += EstimateTokens(messages[i].Content)
		}
	}

	return PruneResult{
		Messages:        kept,
		DroppedChunks:   1,
		DroppedMessages: droppedMessages,
		DroppedTokens:   droppedTokens,
		KeptTokens:      keptTokens,
		BudgetTokens:    budget,
	}
}

// ResolveContextWindowTokens 从模型信息中解析上下文窗口 token 数。
func ResolveContextWindowTokens(contextWindow int, defaultTokens int) int {
	if contextWindow > 0 {
		return contextWindow
	}
	return defaultTokens
}

// ---------- 摘要接口（LLM 调用） ----------

// SummarizeFunc 摘要函数签名。
// 实际实现需要 LLM API 调用，在 PI Runner 集成时注入。
type SummarizeFunc func(messages []AgentMessage, instructions string) (string, error)

// SummarizeChunks 分块摘要（接口定义）。
// TS 参考: compaction.ts → summarizeChunks()
func SummarizeChunks(messages []AgentMessage, maxChunkTokens int, summarize SummarizeFunc, instructions string) (string, error) {
	chunks := ChunkMessagesByMaxTokens(messages, maxChunkTokens)
	var summaries []string
	for _, chunk := range chunks {
		summary, err := summarize(chunk, instructions)
		if err != nil {
			return DefaultSummaryFallback, err
		}
		summaries = append(summaries, summary)
	}
	if len(summaries) == 0 {
		return DefaultSummaryFallback, nil
	}
	if len(summaries) == 1 {
		return summaries[0], nil
	}
	// 合并多个摘要
	merged := strings.Join(summaries, "\n\n---\n\n")
	return summarize([]AgentMessage{{Role: "user", Content: merged}},
		"Merge these partial summaries into a single cohesive summary. Preserve decisions, TODOs, open questions, and any constraints.")
}
