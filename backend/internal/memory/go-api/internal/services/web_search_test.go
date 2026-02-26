// Package services — WebSearchProvider 单元测试。
package services

import (
	"context"
	"testing"

	"github.com/uhms/go-api/internal/config"
)

// --- mockSearchLLM — 搜索模块专用 mock ---

// mockSearchLLM 是 WebSearchProvider 测试专用的 mock LLMProvider。
// 与 memory_manager_reflection_test.go 中的 mockLLMProvider 字段不同，
// 此处需要 Generate 返回指定内容。
type mockSearchLLM struct {
	generateOutput string
	generateErr    error
}

func (m *mockSearchLLM) Generate(_ context.Context, _ string) (string, error) {
	return m.generateOutput, m.generateErr
}
func (m *mockSearchLLM) ExtractEntities(_ context.Context, _ string) (*ExtractionResult, error) {
	return &ExtractionResult{}, nil
}
func (m *mockSearchLLM) ScoreImportance(_ context.Context, _ string) (*ImportanceScore, error) {
	return &ImportanceScore{Score: 0.5}, nil
}
func (m *mockSearchLLM) GenerateReflection(_ context.Context, _ []string, _ string) (string, error) {
	return "", nil
}

// --- FallbackSearcher 降级行为 ---

func TestFallbackSearcher_UsesLLM(t *testing.T) {
	mockLLM := &mockSearchLLM{
		generateOutput: "模拟搜索结果：AI 领域最新进展",
	}
	fb := &fallbackSearcher{llm: mockLLM}

	result, err := fb.Search(t.Context(), "AI 发展趋势")
	if err != nil {
		t.Fatalf("FallbackSearcher 不应失败: %v", err)
	}
	if result.Provider != "llm_inference" {
		t.Fatalf("Provider 应为 llm_inference，实际为 %s", result.Provider)
	}
	if result.Summary == "" {
		t.Fatal("Summary 不应为空")
	}
	if result.SourceURLs == nil || len(result.SourceURLs) != 0 {
		t.Fatal("SourceURLs 应为空切片")
	}
}

// --- WebSearchResult 结构体完整性 ---

func TestWebSearchResult_Defaults(t *testing.T) {
	r := &WebSearchResult{}
	if r.Summary != "" {
		t.Fatal("默认 Summary 应为空")
	}
	if r.Provider != "" {
		t.Fatal("默认 Provider 应为空")
	}
	if r.SourceURLs != nil {
		t.Fatal("默认 SourceURLs 应为 nil")
	}
}

func TestWebSearchResult_WithData(t *testing.T) {
	r := &WebSearchResult{
		Summary:    "测试摘要",
		SourceURLs: []string{"https://example.com/1", "https://example.com/2"},
		Provider:   "deepseek_search",
	}
	if r.Summary != "测试摘要" {
		t.Fatalf("Summary 不匹配: %s", r.Summary)
	}
	if len(r.SourceURLs) != 2 {
		t.Fatalf("SourceURLs 应有 2 条，实际 %d", len(r.SourceURLs))
	}
	if r.Provider != "deepseek_search" {
		t.Fatalf("Provider 应为 deepseek_search，实际为 %s", r.Provider)
	}
}

// --- extractSourceURLs 辅助函数 ---

func TestExtractSourceURLs_WithURLs(t *testing.T) {
	text := `1. AI 领域最新进展
2. 大模型技术突破
[来源] https://example.com/ai-news
另一个来源 https://example.org/tech`

	summary, urls := extractSourceURLs(text)
	if summary == "" {
		t.Fatal("Summary 不应为空")
	}
	if len(urls) != 2 {
		t.Fatalf("应提取 2 个 URL，实际 %d: %v", len(urls), urls)
	}
}

func TestExtractSourceURLs_NoURLs(t *testing.T) {
	text := "纯文本内容，没有链接"
	summary, urls := extractSourceURLs(text)
	if summary != text {
		t.Fatal("无 URL 时 summary 应等于原文")
	}
	if len(urls) != 0 {
		t.Fatalf("应无 URL，实际 %d", len(urls))
	}
}

// --- AutoSearcher 降级链 ---

func TestAutoSearcher_FallsBackToLLM(t *testing.T) {
	mockLLM := &mockSearchLLM{
		generateOutput: "LLM 降级结果",
	}
	fb := &fallbackSearcher{llm: mockLLM}

	auto := &autoSearcher{
		primary:   nil,
		secondary: nil,
		fallback:  fb,
	}

	result, err := auto.Search(t.Context(), "测试查询")
	if err != nil {
		t.Fatalf("AutoSearcher 降级不应失败: %v", err)
	}
	if result.Provider != "llm_inference" {
		t.Fatalf("降级后 Provider 应为 llm_inference，实际为 %s", result.Provider)
	}
}

// --- mockWebSearchProvider ---

type mockWebSearchProvider struct {
	result *WebSearchResult
	err    error
}

func (m *mockWebSearchProvider) Search(_ context.Context, _ string) (*WebSearchResult, error) {
	return m.result, m.err
}

func TestAutoSearcher_UsesPrimary(t *testing.T) {
	primary := &mockWebSearchProvider{
		result: &WebSearchResult{
			Summary:    "主引擎结果",
			SourceURLs: []string{"https://example.com"},
			Provider:   "deepseek_search",
		},
	}
	fb := &fallbackSearcher{llm: &mockSearchLLM{generateOutput: "fallback"}}

	auto := &autoSearcher{
		primary:  primary,
		fallback: fb,
	}

	result, err := auto.Search(t.Context(), "测试")
	if err != nil {
		t.Fatalf("不应失败: %v", err)
	}
	if result.Provider != "deepseek_search" {
		t.Fatalf("应使用主引擎，实际 Provider: %s", result.Provider)
	}
}

// --- DeepSeek/Gemini Searcher 构造函数 ---

func TestNewDeepSeekSearcher_DefaultBaseURL(t *testing.T) {
	cfg := &config.Config{}
	ds := newDeepSeekSearcher(cfg)
	if ds.baseURL != "https://api.deepseek.com/v1" {
		t.Fatalf("默认 baseURL 应为 DeepSeek 官方，实际为 %s", ds.baseURL)
	}
}

func TestNewGeminiSearcher_DefaultModel(t *testing.T) {
	cfg := &config.Config{}
	gm := newGeminiSearcher(cfg)
	if gm.model != "gemini-1.5-pro" {
		t.Fatalf("默认 model 应为 gemini-1.5-pro，实际为 %s", gm.model)
	}
}
