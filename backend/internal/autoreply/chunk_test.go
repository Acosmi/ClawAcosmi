package autoreply

import (
	"strings"
	"testing"
)

// expectFencesBalanced 检查所有 chunk 中的围栏代码块是否闭合。
func expectFencesBalanced(t *testing.T, chunks []string) {
	t.Helper()
	for i, chunk := range chunks {
		type openState struct {
			markerChar byte
			markerLen  int
		}
		var open *openState
		for _, line := range strings.Split(chunk, "\n") {
			// 匹配围栏标记
			trimmed := line
			indent := 0
			for indent < len(trimmed) && trimmed[indent] == ' ' && indent < 3 {
				indent++
			}
			trimmed = trimmed[indent:]
			if len(trimmed) < 3 {
				continue
			}
			markerCh := trimmed[0]
			if markerCh != '`' && markerCh != '~' {
				continue
			}
			markerLen := 0
			for markerLen < len(trimmed) && trimmed[markerLen] == markerCh {
				markerLen++
			}
			if markerLen < 3 {
				continue
			}
			if open == nil {
				open = &openState{markerChar: markerCh, markerLen: markerLen}
			} else if open.markerChar == markerCh && markerLen >= open.markerLen {
				open = nil
			}
		}
		if open != nil {
			t.Errorf("chunk[%d] has unclosed fence", i)
		}
	}
}

// ---------- ChunkText ----------

func TestChunkText_KeepsMultiLineUnderLimit(t *testing.T) {
	text := "Line one\n\nLine two\n\nLine three"
	chunks := ChunkText(text, 1600)
	if len(chunks) != 1 || chunks[0] != text {
		t.Errorf("should keep as single chunk, got %d chunks", len(chunks))
	}
}

func TestChunkText_SplitsWhenExceeds(t *testing.T) {
	part := strings.Repeat("a", 20)
	text := strings.Repeat(part, 5) // 100 chars
	chunks := ChunkText(text, 60)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 60 {
		t.Errorf("first chunk length: got %d, want 60", len(chunks[0]))
	}
	if len(chunks[1]) != 40 {
		t.Errorf("second chunk length: got %d, want 40", len(chunks[1]))
	}
}

func TestChunkText_PrefersNewlineBreak(t *testing.T) {
	text := "paragraph one line\n\nparagraph two starts here and continues"
	chunks := ChunkText(text, 40)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0] != "paragraph one line" {
		t.Errorf("first chunk: got %q", chunks[0])
	}
}

func TestChunkText_BreaksAtWhitespace(t *testing.T) {
	text := "This is a message that should break nicely near a word boundary."
	chunks := ChunkText(text, 30)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len(c) > 30 {
			t.Errorf("chunk exceeds limit: %q (%d chars)", c, len(c))
		}
	}
}

func TestChunkText_HardBreakNoWhitespace(t *testing.T) {
	text := "Supercalifragilisticexpialidocious" // 34 chars
	chunks := ChunkText(text, 10)
	expected := []string{"Supercalif", "ragilistic", "expialidoc", "ious"}
	if len(chunks) != len(expected) {
		t.Fatalf("expected %d chunks, got %d", len(expected), len(chunks))
	}
	for i, c := range chunks {
		if c != expected[i] {
			t.Errorf("chunk[%d]: got %q, want %q", i, c, expected[i])
		}
	}
}

func TestChunkText_Parenthetical(t *testing.T) {
	text := "Heads up now (Though now I'm curious)ok"
	chunks := ChunkText(text, 35)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "Heads up now" {
		t.Errorf("chunk[0]: got %q, want %q", chunks[0], "Heads up now")
	}
}

func TestChunkText_Empty(t *testing.T) {
	chunks := ChunkText("", 100)
	if len(chunks) != 0 {
		t.Errorf("empty text should return nil, got %v", chunks)
	}
}

// ---------- ChunkMarkdownText ----------

func TestChunkMarkdownText_KeepsFencedBlockIntact(t *testing.T) {
	prefix := strings.Repeat("p", 60)
	fence := "```bash\nline1\nline2\n```"
	suffix := strings.Repeat("s", 60)
	text := prefix + "\n\n" + fence + "\n\n" + suffix

	chunks := ChunkMarkdownText(text, 40)
	foundFence := false
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk) == fence {
			foundFence = true
		}
	}
	if !foundFence {
		t.Error("fenced block should remain intact in one chunk")
	}
	expectFencesBalanced(t, chunks)
}

func TestChunkMarkdownText_ReopensFencedBlock(t *testing.T) {
	text := "```txt\n" + strings.Repeat("a", 500) + "\n```"
	limit := 120
	chunks := ChunkMarkdownText(text, limit)
	if len(chunks) <= 1 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len(chunk) > limit {
			t.Errorf("chunk[%d] exceeds limit: %d > %d", i, len(chunk), limit)
		}
		if !strings.HasPrefix(chunk, "```txt\n") {
			t.Errorf("chunk[%d] should start with fence open: %q", i, chunk[:min(20, len(chunk))])
		}
		if !strings.HasSuffix(strings.TrimSpace(chunk), "```") {
			t.Errorf("chunk[%d] should end with fence close", i)
		}
	}
	expectFencesBalanced(t, chunks)
}

func TestChunkMarkdownText_TildeFences(t *testing.T) {
	text := "~~~sh\n" + strings.Repeat("x", 600) + "\n~~~"
	limit := 140
	chunks := ChunkMarkdownText(text, limit)
	if len(chunks) <= 1 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len(chunk) > limit {
			t.Errorf("chunk[%d] exceeds limit: %d", i, len(chunk))
		}
		if !strings.HasPrefix(chunk, "~~~sh\n") {
			t.Errorf("chunk[%d] should start with ~~~sh", i)
		}
		if !strings.HasSuffix(strings.TrimSpace(chunk), "~~~") {
			t.Errorf("chunk[%d] should end with ~~~", i)
		}
	}
	expectFencesBalanced(t, chunks)
}

func TestChunkMarkdownText_LongerFenceMarkers(t *testing.T) {
	text := "````md\n" + strings.Repeat("y", 600) + "\n````"
	limit := 140
	chunks := ChunkMarkdownText(text, limit)
	if len(chunks) <= 1 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len(chunk) > limit {
			t.Errorf("chunk[%d] exceeds limit: %d", i, len(chunk))
		}
		if !strings.HasPrefix(chunk, "````md\n") {
			t.Errorf("chunk[%d] should start with ````md", i)
		}
		if !strings.HasSuffix(strings.TrimSpace(chunk), "````") {
			t.Errorf("chunk[%d] should end with ````", i)
		}
	}
	expectFencesBalanced(t, chunks)
}

func TestChunkMarkdownText_IndentedFences(t *testing.T) {
	text := "  ```js\n  " + strings.Repeat("z", 600) + "\n  ```"
	limit := 160
	chunks := ChunkMarkdownText(text, limit)
	if len(chunks) <= 1 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len(chunk) > limit {
			t.Errorf("chunk[%d] exceeds limit: %d", i, len(chunk))
		}
		if !strings.HasPrefix(chunk, "  ```js\n") {
			t.Errorf("chunk[%d] should start with '  ```js': %q", i, chunk[:min(20, len(chunk))])
		}
		if !strings.HasSuffix(strings.TrimSpace(chunk), "  ```") {
			t.Errorf("chunk[%d] should end with '  ```'", i)
		}
	}
	expectFencesBalanced(t, chunks)
}

func TestChunkMarkdownText_Parenthetical(t *testing.T) {
	text := "Heads up now (Though now I'm curious)ok"
	chunks := ChunkMarkdownText(text, 35)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "Heads up now" {
		t.Errorf("chunk[0]: got %q", chunks[0])
	}
}

// ---------- ChunkByParagraph ----------

func TestChunkByParagraph_Basic(t *testing.T) {
	text := "Para one\n\nPara two"
	chunks := ChunkByParagraph(text, 1000, true)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "Para one" {
		t.Errorf("chunk[0]: got %q", chunks[0])
	}
	if chunks[1] != "Para two" {
		t.Errorf("chunk[1]: got %q", chunks[1])
	}
}

func TestChunkByParagraph_NoBlankLines(t *testing.T) {
	text := "Line one\nLine two"
	chunks := ChunkByParagraph(text, 1000, true)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "Line one\nLine two" {
		t.Errorf("got %q", chunks[0])
	}
}

func TestChunkByParagraph_FencePreserved(t *testing.T) {
	text := "```python\ndef my_function():\n    x = 1\n\n    y = 2\n    return x + y\n```"
	chunks := ChunkByParagraph(text, 1000, true)
	if len(chunks) != 1 {
		t.Fatalf("fenced block should not be split, got %d chunks", len(chunks))
	}
}

func TestChunkByParagraph_FenceThenParagraph(t *testing.T) {
	fence := "```python\ndef my_function():\n    x = 1\n\n    y = 2\n    return x + y\n```"
	text := fence + "\n\nAfter"
	chunks := ChunkByParagraph(text, 1000, true)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != fence {
		t.Errorf("chunk[0] should be fence block")
	}
	if chunks[1] != "After" {
		t.Errorf("chunk[1]: got %q", chunks[1])
	}
}

// ---------- ChunkTextWithMode ----------

func TestChunkTextWithMode_Length(t *testing.T) {
	text := "Line one\nLine two"
	chunks := ChunkTextWithMode(text, 1000, ChunkModeLength)
	if len(chunks) != 1 || chunks[0] != text {
		t.Errorf("length mode: got %v", chunks)
	}
}

func TestChunkTextWithMode_Newline(t *testing.T) {
	text := "Para one\n\nPara two"
	chunks := ChunkTextWithMode(text, 1000, ChunkModeNewline)
	if len(chunks) != 2 {
		t.Fatalf("newline mode: expected 2 chunks, got %d", len(chunks))
	}
}

// ---------- ChunkMarkdownTextWithMode ----------

func TestChunkMarkdownTextWithMode_NewlineFencePreserved(t *testing.T) {
	text := "```js\nconst a = 1;\nconst b = 2;\n```\nAfter"
	chunks := ChunkMarkdownTextWithMode(text, 1000, ChunkModeNewline)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk (no blank line separator), got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != text {
		t.Errorf("got %q", chunks[0])
	}
}

func TestChunkMarkdownTextWithMode_NewlineBlankLineSplit(t *testing.T) {
	text := "Para one\n\nPara two"
	chunks := ChunkMarkdownTextWithMode(text, 1000, ChunkModeNewline)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
}

func TestChunkMarkdownTextWithMode_FenceBlankLineNoSplit(t *testing.T) {
	text := "```python\ndef my_function():\n    x = 1\n\n    y = 2\n    return x + y\n```"
	chunks := ChunkMarkdownTextWithMode(text, 1000, ChunkModeNewline)
	if len(chunks) != 1 {
		t.Fatalf("fence with internal blank line should not split, got %d", len(chunks))
	}
}

func TestChunkMarkdownTextWithMode_FenceThenParagraph(t *testing.T) {
	fence := "```python\ndef my_function():\n    x = 1\n\n    y = 2\n    return x + y\n```"
	text := fence + "\n\nAfter"
	chunks := ChunkMarkdownTextWithMode(text, 1000, ChunkModeNewline)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != fence {
		t.Errorf("chunk[0] should be fence block")
	}
	if chunks[1] != "After" {
		t.Errorf("chunk[1]: got %q", chunks[1])
	}
}

// ---------- 配置解析 ----------

func TestResolveTextChunkLimit_Default(t *testing.T) {
	limit := ResolveTextChunkLimit(nil, "", 0)
	if limit != DefaultChunkLimit {
		t.Errorf("got %d, want %d", limit, DefaultChunkLimit)
	}
}

func TestResolveTextChunkLimit_Fallback(t *testing.T) {
	limit := ResolveTextChunkLimit(nil, "", 2000)
	if limit != 2000 {
		t.Errorf("got %d, want 2000", limit)
	}
}

func TestResolveTextChunkLimit_ProviderOverride(t *testing.T) {
	cfg := &ProviderChunkConfig{TextChunkLimit: 1234}
	limit := ResolveTextChunkLimit(cfg, "", 0)
	if limit != 1234 {
		t.Errorf("got %d, want 1234", limit)
	}
}

func TestResolveTextChunkLimit_AccountOverride(t *testing.T) {
	cfg := &ProviderChunkConfig{
		TextChunkLimit: 2000,
		Accounts: map[string]AccountChunkConfig{
			"primary": {TextChunkLimit: 777},
		},
	}
	limit := ResolveTextChunkLimit(cfg, "primary", 0)
	if limit != 777 {
		t.Errorf("got %d, want 777", limit)
	}
}

func TestResolveChunkMode_Default(t *testing.T) {
	mode := ResolveChunkMode(nil, "")
	if mode != ChunkModeLength {
		t.Errorf("got %q, want %q", mode, ChunkModeLength)
	}
}

func TestResolveChunkMode_ProviderOverride(t *testing.T) {
	cfg := &ProviderChunkConfig{ChunkMode: ChunkModeNewline}
	mode := ResolveChunkMode(cfg, "")
	if mode != ChunkModeNewline {
		t.Errorf("got %q, want %q", mode, ChunkModeNewline)
	}
}

func TestResolveChunkMode_AccountOverride(t *testing.T) {
	cfg := &ProviderChunkConfig{
		ChunkMode: ChunkModeLength,
		Accounts: map[string]AccountChunkConfig{
			"primary": {ChunkMode: ChunkModeNewline},
		},
	}
	mode := ResolveChunkMode(cfg, "primary")
	if mode != ChunkModeNewline {
		t.Errorf("got %q, want %q", mode, ChunkModeNewline)
	}
}
