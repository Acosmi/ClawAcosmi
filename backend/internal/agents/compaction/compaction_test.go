package compaction

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	// 空串: min 1
	if EstimateTokens("") != 1 {
		t.Error("empty should be 1")
	}
	// 40 字符 → ~10 tokens
	if got := EstimateTokens(strings.Repeat("x", 40)); got != 10 {
		t.Errorf("40 chars = %d tokens, want 10", got)
	}
}

func TestEstimateMessagesTokens(t *testing.T) {
	msgs := []AgentMessage{
		{Content: strings.Repeat("a", 100)}, // 25
		{Content: strings.Repeat("b", 200)}, // 50
	}
	total := EstimateMessagesTokens(msgs)
	if total != 75 {
		t.Errorf("total = %d, want 75", total)
	}
}

func TestSplitMessagesByTokenShare(t *testing.T) {
	msgs := make([]AgentMessage, 10)
	for i := range msgs {
		msgs[i] = AgentMessage{Content: strings.Repeat("x", 40)} // 10 tokens each
	}
	// 分 2 片
	parts := SplitMessagesByTokenShare(msgs, 2)
	if len(parts) != 2 {
		t.Errorf("parts = %d, want 2", len(parts))
	}
	// 总数应保留
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	if total != 10 {
		t.Errorf("total messages = %d, want 10", total)
	}
}

func TestChunkMessagesByMaxTokens(t *testing.T) {
	msgs := []AgentMessage{
		{Content: strings.Repeat("a", 40)},  // 10 tokens
		{Content: strings.Repeat("b", 40)},  // 10 tokens
		{Content: strings.Repeat("c", 100)}, // 25 tokens
	}
	chunks := ChunkMessagesByMaxTokens(msgs, 20)
	if len(chunks) < 2 {
		t.Errorf("chunks = %d, want >= 2", len(chunks))
	}
}

func TestPruneHistoryForContextShare(t *testing.T) {
	msgs := make([]AgentMessage, 10)
	for i := range msgs {
		msgs[i] = AgentMessage{Content: strings.Repeat("x", 40)} // 10 tokens each
	}
	// Total = 100 tokens, budget = 50 (100 * 0.5)
	result := PruneHistoryForContextShare(msgs, 100, 0.5, 2)
	if result.BudgetTokens != 50 {
		t.Errorf("budget = %d, want 50", result.BudgetTokens)
	}
	if result.KeptTokens > 50 {
		t.Errorf("kept = %d, should be <= 50", result.KeptTokens)
	}
	if result.DroppedMessages <= 0 {
		t.Error("should drop some messages")
	}
	// 验证保留的是最近的消息
	if len(result.Messages) == 0 {
		t.Error("should keep some messages")
	}
}

func TestPruneHistoryForContextShare_UnderBudget(t *testing.T) {
	msgs := []AgentMessage{
		{Content: strings.Repeat("x", 20)}, // 5 tokens
	}
	result := PruneHistoryForContextShare(msgs, 1000, 0.5, 2)
	if result.DroppedMessages != 0 {
		t.Error("under budget should not drop")
	}
	if len(result.Messages) != 1 {
		t.Error("should keep all messages")
	}
}

func TestComputeAdaptiveChunkRatio(t *testing.T) {
	msgs := make([]AgentMessage, 5)
	for i := range msgs {
		msgs[i] = AgentMessage{Content: strings.Repeat("x", 400)} // 100 tokens each
	}
	ratio := ComputeAdaptiveChunkRatio(msgs, 1000)
	if ratio < 0.1 || ratio > 0.5 {
		t.Errorf("ratio = %f, should be in [0.1, 0.5]", ratio)
	}
}
