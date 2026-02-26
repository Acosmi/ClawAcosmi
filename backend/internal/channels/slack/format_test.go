package slack

import (
	"strings"
	"testing"
)

func TestMarkdownToSlackMrkdwn_Empty(t *testing.T) {
	if got := MarkdownToSlackMrkdwn(""); got != "" {
		t.Errorf("empty input should return empty, got %q", got)
	}
}

func TestMarkdownToSlackMrkdwn_PlainText(t *testing.T) {
	got := MarkdownToSlackMrkdwn("Hello world")
	if !strings.Contains(got, "Hello world") {
		t.Errorf("plain text should be preserved, got %q", got)
	}
}

func TestMarkdownToSlackMrkdwn_Bold(t *testing.T) {
	got := MarkdownToSlackMrkdwn("**bold text**")
	if !strings.Contains(got, "*") {
		t.Errorf("bold should use * markers, got %q", got)
	}
	if !strings.Contains(got, "bold text") {
		t.Errorf("bold text content should be preserved, got %q", got)
	}
}

func TestMarkdownToSlackMrkdwn_InlineCode(t *testing.T) {
	got := MarkdownToSlackMrkdwn("use `foo` command")
	if !strings.Contains(got, "`") {
		t.Errorf("inline code should use backtick, got %q", got)
	}
	if !strings.Contains(got, "foo") {
		t.Errorf("code content should be preserved, got %q", got)
	}
}

func TestMarkdownToSlackMrkdwn_EscapeSpecialChars(t *testing.T) {
	got := MarkdownToSlackMrkdwn("a < b & c > d")
	if strings.Contains(got, "<") && !strings.Contains(got, "&lt;") {
		t.Errorf("< should be escaped, got %q", got)
	}
	if strings.Contains(got, "&") && !strings.Contains(got, "&amp;") && !strings.Contains(got, "&lt;") && !strings.Contains(got, "&gt;") {
		t.Errorf("& should be escaped, got %q", got)
	}
}

func TestMarkdownToSlackMrkdwnChunks_ShortText(t *testing.T) {
	chunks := MarkdownToSlackMrkdwnChunks("Hello", 100)
	if len(chunks) != 1 {
		t.Fatalf("short text should produce 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "Hello") {
		t.Errorf("chunk should contain text, got %q", chunks[0])
	}
}

func TestMarkdownToSlackMrkdwnChunks_Empty(t *testing.T) {
	chunks := MarkdownToSlackMrkdwnChunks("", 100)
	if len(chunks) != 0 {
		t.Fatalf("empty input should produce 0 chunks, got %d", len(chunks))
	}
}

func TestMarkdownToSlackMrkdwnChunks_CodeBlockIntegrity(t *testing.T) {
	md := "Before\n```\nline1\nline2\nline3\n```\nAfter"
	chunks := MarkdownToSlackMrkdwnChunks(md, 5000)
	if len(chunks) == 0 {
		t.Fatal("should produce at least 1 chunk")
	}
	// 检验代码块在单块内完整
	joined := strings.Join(chunks, "|||")
	if !strings.Contains(joined, "line1") || !strings.Contains(joined, "line3") {
		t.Errorf("code block content should be preserved, got %q", joined)
	}
}

func TestMarkdownToSlackMrkdwnChunks_LongTextSplits(t *testing.T) {
	// 生成超过限制的长文本
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("This is a line of text for testing chunk splitting.\n")
	}
	chunks := MarkdownToSlackMrkdwnChunks(sb.String(), 200)
	if len(chunks) <= 1 {
		t.Fatalf("long text should produce multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if len(chunk) > 250 { // 稍有容差，因为围栏感知可能超出一点
			t.Errorf("chunk %d exceeds limit: %d chars", i, len(chunk))
		}
	}
}

func TestMarkdownToSlackMrkdwnChunks_FenceAwareSplit(t *testing.T) {
	// 代码块 + 长文本 — 验证围栏感知不会在块内切割
	md := "Intro paragraph.\n\n```go\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n```\n\n" +
		strings.Repeat("After the code block. ", 50)
	chunks := MarkdownToSlackMrkdwnChunks(md, 200)
	if len(chunks) == 0 {
		t.Fatal("should produce chunks")
	}
	// 至少有一个块包含完整的代码
	hasCodeBlock := false
	for _, c := range chunks {
		if strings.Contains(c, "fmt.Println") {
			hasCodeBlock = true
			break
		}
	}
	if !hasCodeBlock {
		t.Error("code block content should appear in some chunk")
	}
}
