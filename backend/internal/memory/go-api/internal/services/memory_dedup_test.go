// Package services — memory_dedup 单元测试。
package services

import (
	"context"
	"testing"
)

// mockLLMForFacts 是用于事实提取测试的 Mock LLM。
type mockLLMForFacts struct {
	response string
	err      error
}

func (m *mockLLMForFacts) Generate(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}
func (m *mockLLMForFacts) GenerateReflection(ctx context.Context, memories []string, coreMemoryContext string) (string, error) {
	return "", nil
}
func (m *mockLLMForFacts) ExtractEntities(ctx context.Context, content string) (*ExtractionResult, error) {
	return &ExtractionResult{}, nil
}
func (m *mockLLMForFacts) ScoreImportance(ctx context.Context, text string) (*ImportanceScore, error) {
	return &ImportanceScore{Score: 0.5}, nil
}

func TestExtractFacts_Success(t *testing.T) {
	mock := &mockLLMForFacts{
		response: `[{"fact": "用户喜欢深色模式", "category": "preference", "confidence": 0.9}]`,
	}

	facts, err := ExtractFacts(context.Background(), mock, "我更喜欢深色模式的界面")
	if err != nil {
		t.Fatalf("ExtractFacts 不应返回错误: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("期望 1 个事实，得到 %d", len(facts))
	}
	if facts[0].Content != "用户喜欢深色模式" {
		t.Errorf("事实内容不匹配: %s", facts[0].Content)
	}
	if facts[0].Category != "preference" {
		t.Errorf("事实分类不匹配: %s", facts[0].Category)
	}
	if facts[0].Confidence != 0.9 {
		t.Errorf("置信度不匹配: %f", facts[0].Confidence)
	}
}

func TestExtractFacts_EmptyContent(t *testing.T) {
	facts, err := ExtractFacts(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("空内容不应返回错误: %v", err)
	}
	if facts != nil {
		t.Fatalf("空内容应返回 nil，得到 %v", facts)
	}
}

func TestExtractFacts_NoLLM(t *testing.T) {
	facts, err := ExtractFacts(context.Background(), nil, "一些内容")
	if err != nil {
		t.Fatalf("无 LLM 不应返回错误: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("无 LLM 应退化为 1 条事实，得到 %d", len(facts))
	}
	if facts[0].Content != "一些内容" {
		t.Errorf("退化事实内容不匹配: %s", facts[0].Content)
	}
}

func TestExtractFacts_MultipleFacts(t *testing.T) {
	mock := &mockLLMForFacts{
		response: `[
			{"fact": "用户名叫张三", "category": "profile", "confidence": 0.95},
			{"fact": "用户会 Python", "category": "skill", "confidence": 0.8}
		]`,
	}

	facts, err := ExtractFacts(context.Background(), mock, "我叫张三，我会写 Python")
	if err != nil {
		t.Fatalf("ExtractFacts 不应返回错误: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("期望 2 个事实，得到 %d", len(facts))
	}
}

func TestDeduplicateFacts_NoVectorStore(t *testing.T) {
	facts := []Fact{
		{Content: "用户喜欢咖啡", Category: "preference", Confidence: 0.9},
	}

	result := DeduplicateFacts(context.Background(), facts, "user1", nil, 0.85)
	if len(result.NewFacts) != 1 {
		t.Fatalf("无向量存储时所有事实应为新事实，得到 %d", len(result.NewFacts))
	}
	if result.SkippedCount != 0 {
		t.Errorf("不应有跳过: %d", result.SkippedCount)
	}
}

func TestDeduplicateFacts_EmptyFacts(t *testing.T) {
	result := DeduplicateFacts(context.Background(), nil, "user1", nil, 0.85)
	if len(result.NewFacts) != 0 {
		t.Errorf("空事实列表应返回空结果")
	}
}

func TestMergeFacts_NoLLM(t *testing.T) {
	pairs := []DedupPair{
		{
			NewFact:      Fact{Content: "新事实"},
			ExistingFact: Fact{Content: "旧事实"},
			ExistingID:   "test-id",
		},
	}

	result := MergeFacts(context.Background(), nil, pairs)
	if len(result) != 1 {
		t.Fatalf("无 LLM 应原样返回")
	}
	// 无 LLM 时内容不变
	if result[0].NewFact.Content != "新事实" {
		t.Errorf("无 LLM 时不应修改内容")
	}
}

func TestMergeFacts_WithLLM(t *testing.T) {
	mock := &mockLLMForFacts{response: "用户非常喜欢喝浓咖啡"}
	pairs := []DedupPair{
		{
			NewFact:      Fact{Content: "用户喜欢浓咖啡"},
			ExistingFact: Fact{Content: "用户喜欢喝咖啡"},
			ExistingID:   "test-id",
		},
	}

	result := MergeFacts(context.Background(), mock, pairs)
	if result[0].NewFact.Content != "用户非常喜欢喝浓咖啡" {
		t.Errorf("合并结果不匹配: %s", result[0].NewFact.Content)
	}
}

func TestParseFactsJSON_Valid(t *testing.T) {
	input := `[{"fact": "test", "category": "fact", "confidence": 0.9}]`
	facts, err := parseFactsJSON(input)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("期望 1 个事实，得到 %d", len(facts))
	}
}

func TestParseFactsJSON_WithMarkdown(t *testing.T) {
	input := "```json\n[{\"fact\": \"test\", \"category\": \"fact\", \"confidence\": 0.9}]\n```"
	facts, err := parseFactsJSON(input)
	if err != nil {
		t.Fatalf("解析带 markdown 的 JSON 失败: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("期望 1 个事实，得到 %d", len(facts))
	}
}

func TestParseFactsJSON_Empty(t *testing.T) {
	facts, err := parseFactsJSON("[]")
	if err != nil {
		t.Fatalf("解析空数组不应失败: %v", err)
	}
	if len(facts) != 0 {
		t.Errorf("空数组应返回 0 个事实")
	}
}
