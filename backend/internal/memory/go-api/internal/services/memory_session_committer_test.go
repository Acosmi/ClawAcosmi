// Package services — SessionCommitter 单元测试。
package services

import (
	"context"
	"testing"
)

// TestCommitEmptyMessages 验证空消息返回空结果。
func TestCommitEmptyMessages(t *testing.T) {
	sc := NewSessionCommitter(nil, nil, nil)
	result, err := sc.Commit(context.Background(), "session-1", "user-1", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExtractedCount != 0 {
		t.Errorf("expected 0 extracted, got %d", result.ExtractedCount)
	}
	if result.CreatedCount != 0 {
		t.Errorf("expected 0 created, got %d", result.CreatedCount)
	}
}

// TestCommitWhitespaceMessages 验证纯白空格消息返回空结果。
func TestCommitWhitespaceMessages(t *testing.T) {
	sc := NewSessionCommitter(nil, nil, nil)
	result, err := sc.Commit(context.Background(), "s2", "u2", "   \n\t  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExtractedCount != 0 {
		t.Errorf("expected 0 extracted, got %d", result.ExtractedCount)
	}
}

// TestCommitSummaryGeneration 验证无 LLM 时的降级摘要。
func TestCommitSummaryGeneration(t *testing.T) {
	sc := NewSessionCommitter(nil, nil, nil) // no LLM → fallback
	result, err := sc.Commit(context.Background(), "s3", "u3", "用户讨论了 Rust 和 Go 的选择")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SummaryContent == "" {
		t.Error("expected non-empty summary content")
	}
	// 无 LLM → ExtractedCount 应为 0（extractMemories 返回 nil）
	if result.ExtractedCount != 0 {
		t.Errorf("expected 0 extracted without LLM, got %d", result.ExtractedCount)
	}
}

// TestCommitExtractionParsing 验证 mock LLM 返回的 6 分类解析。
func TestCommitExtractionParsing(t *testing.T) {
	mockLLM := &mockCommitterLLM{
		summaryResp: `{"one_line":"讨论技术选型","analysis":"用户分析了多种语言","core_intent":"选择开发语言","key_concepts":["Rust","Go"],"action_items":["调研性能"]}`,
		extractResp: `[
			{"content":"用户是全栈工程师","category":"profile","owner":"user"},
			{"content":"偏好使用暗色主题","category":"preferences","owner":"user"},
			{"content":"项目名为 UHMS","category":"entities","owner":"user"}
		]`,
	}

	sc := NewSessionCommitter(nil, mockLLM, nil)
	result, err := sc.Commit(context.Background(), "s4", "u4", "一段对话内容")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SummaryContent != "讨论技术选型" {
		t.Errorf("expected summary '讨论技术选型', got '%s'", result.SummaryContent)
	}
	if result.ExtractedCount != 3 {
		t.Errorf("expected 3 extracted, got %d", result.ExtractedCount)
	}
	// 无 DB/VectorStore → CreatedCount 应为 0（DB create 会失败但被跳过）
}

// TestMapCategoryToInternal 验证 6 分类映射。
func TestMapCategoryToInternal(t *testing.T) {
	sc := &SessionCommitter{}

	tests := []struct {
		input    string
		expected string
	}{
		{"profile", CategoryProfile},
		{"preferences", CategoryPreference},
		{"entities", CategoryRelationship},
		{"events", CategoryEvent},
		{"cases", CategoryInsight},
		{"patterns", CategorySkill},
		{"unknown", CategoryFact},
		{"PROFILE", CategoryProfile}, // 大小写不敏感
	}

	for _, tt := range tests {
		got := sc.mapCategoryToInternal(tt.input)
		if got != tt.expected {
			t.Errorf("mapCategoryToInternal(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- Mock LLM ---

// mockCommitterLLM 是 SessionCommitter 测试专用的 mock LLMProvider。
type mockCommitterLLM struct {
	summaryResp string
	extractResp string
	callCount   int
}

func (m *mockCommitterLLM) Generate(_ context.Context, _ string) (string, error) {
	m.callCount++
	// 第一次调用返回摘要，第二次返回提取结果
	if m.callCount == 1 {
		return m.summaryResp, nil
	}
	return m.extractResp, nil
}

func (m *mockCommitterLLM) ExtractEntities(_ context.Context, _ string) (*ExtractionResult, error) {
	return &ExtractionResult{}, nil
}

func (m *mockCommitterLLM) ScoreImportance(_ context.Context, _ string) (*ImportanceScore, error) {
	return &ImportanceScore{Score: 0.5}, nil
}

func (m *mockCommitterLLM) GenerateReflection(_ context.Context, _ []string, _ string) (string, error) {
	return "", nil
}
