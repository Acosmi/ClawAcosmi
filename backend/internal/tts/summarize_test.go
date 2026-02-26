package tts

import (
	"context"
	"fmt"
	"testing"
)

// ---------- SummarizeTextForTts ----------

func TestSummarizeTextForTts_NoSummarizer(t *testing.T) {
	// 清除全局 summarizer
	old := summarizer
	summarizer = nil
	defer func() { summarizer = old }()

	_, err := SummarizeTextForTts(SummarizeRequest{
		Text:         "Long text that needs summarization",
		TargetLength: 200,
		TimeoutMs:    5000,
	})
	if err == nil {
		t.Fatal("expected error when summarizer not registered")
	}
}

func TestSummarizeTextForTts_InvalidTargetLength(t *testing.T) {
	old := summarizer
	summarizer = func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		return "summary", nil
	}
	defer func() { summarizer = old }()

	tests := []int{0, 50, 99, 10001, 20000}
	for _, tl := range tests {
		_, err := SummarizeTextForTts(SummarizeRequest{
			Text:         "test",
			TargetLength: tl,
			TimeoutMs:    5000,
		})
		if err == nil {
			t.Errorf("expected error for targetLength=%d", tl)
		}
	}
}

func TestSummarizeTextForTts_Success(t *testing.T) {
	old := summarizer
	summarizer = func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		return "This is a concise summary", nil
	}
	defer func() { summarizer = old }()

	result, err := SummarizeTextForTts(SummarizeRequest{
		Text:         "A very long text that should be summarized into something shorter",
		TargetLength: 200,
		TimeoutMs:    5000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "This is a concise summary" {
		t.Errorf("unexpected summary: %q", result.Summary)
	}
	if result.LatencyMs < 0 {
		t.Error("latencyMs should be non-negative")
	}
}

func TestSummarizeTextForTts_EmptySummary(t *testing.T) {
	old := summarizer
	summarizer = func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		return "", nil
	}
	defer func() { summarizer = old }()

	_, err := SummarizeTextForTts(SummarizeRequest{
		Text:         "text",
		TargetLength: 200,
		TimeoutMs:    5000,
	})
	if err == nil {
		t.Fatal("expected error for empty summary")
	}
}

func TestSummarizeTextForTts_LLMError(t *testing.T) {
	old := summarizer
	summarizer = func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		return "", fmt.Errorf("API quota exceeded")
	}
	defer func() { summarizer = old }()

	_, err := SummarizeTextForTts(SummarizeRequest{
		Text:         "text",
		TargetLength: 200,
		TimeoutMs:    5000,
	})
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

// ---------- SummarizeTextPrompt ----------

func TestSummarizeTextPrompt(t *testing.T) {
	prompt := SummarizeTextPrompt("Hello world", 500)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	// Should contain the target length
	if !containsSubstring(prompt, "500") {
		t.Error("prompt should mention target length")
	}
	// Should contain the text
	if !containsSubstring(prompt, "Hello world") {
		t.Error("prompt should contain original text")
	}
	// Should contain XML tags matching TS
	if !containsSubstring(prompt, "<text_to_summarize>") {
		t.Error("prompt should contain <text_to_summarize> tag")
	}
}

// ---------- maybeSummarizeText ----------

func TestMaybeSummarizeText_ShortText(t *testing.T) {
	text, summarized := maybeSummarizeText("Hello", 100, "", ResolvedTtsConfig{MaxTextLength: 4096})
	if summarized {
		t.Error("short text should not be summarized")
	}
	if text != "Hello" {
		t.Errorf("text should be unchanged, got %q", text)
	}
}

func TestMaybeSummarizeText_TruncateWhenDisabled(t *testing.T) {
	// 摘要未启用（默认 prefs 不存在 = 看 DefaultTtsSummarize）
	longText := make([]rune, 2000)
	for i := range longText {
		longText[i] = 'x'
	}
	// 临时覆盖 summarizer 确保不走 LLM
	old := summarizer
	summarizer = nil
	defer func() { summarizer = old }()

	text, summarized := maybeSummarizeText(string(longText), 500, "/tmp/nonexistent-prefs.json", ResolvedTtsConfig{MaxTextLength: 4096})
	if summarized {
		t.Error("should not be summarized when summarizer is nil")
	}
	runeLen := len([]rune(text))
	if runeLen > 500 {
		t.Errorf("truncated text too long: %d > 500", runeLen)
	}
}

func TestMaybeSummarizeText_FallbackOnError(t *testing.T) {
	// 注册一个总是失败的 summarizer
	old := summarizer
	summarizer = func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		return "", fmt.Errorf("LLM unavailable")
	}
	defer func() { summarizer = old }()

	longText := make([]rune, 2000)
	for i := range longText {
		longText[i] = 'y'
	}
	text, summarized := maybeSummarizeText(string(longText), 500, "/tmp/nonexistent-prefs.json", ResolvedTtsConfig{MaxTextLength: 4096, TimeoutMs: 1000})
	if summarized {
		t.Error("should not be marked as summarized when LLM fails")
	}
	runeLen := len([]rune(text))
	if runeLen > 500 {
		t.Errorf("fallback truncated text too long: %d > 500", runeLen)
	}
}

// ---------- RegisterSummarizer ----------

func TestRegisterSummarizer(t *testing.T) {
	old := summarizer
	defer func() { summarizer = old }()

	RegisterSummarizer(func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		return "registered", nil
	})
	if summarizer == nil {
		t.Fatal("summarizer should be non-nil after registration")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
