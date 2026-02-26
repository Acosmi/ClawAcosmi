package telegram

import (
	"strings"
	"testing"
)

func TestMarkdownToTelegramHTML_Empty(t *testing.T) {
	if got := MarkdownToTelegramHTML("", TableModeDefault); got != "" {
		t.Errorf("empty input should return empty, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_PlainText(t *testing.T) {
	got := MarkdownToTelegramHTML("Hello world", TableModeDefault)
	if !strings.Contains(got, "Hello world") {
		t.Errorf("plain text should be preserved, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_Bold(t *testing.T) {
	got := MarkdownToTelegramHTML("**bold**", TableModeDefault)
	if !strings.Contains(got, "<b>") || !strings.Contains(got, "</b>") {
		t.Errorf("bold should use <b> tags, got %q", got)
	}
	if !strings.Contains(got, "bold") {
		t.Errorf("bold text content should be preserved, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_Italic(t *testing.T) {
	got := MarkdownToTelegramHTML("*italic*", TableModeDefault)
	if !strings.Contains(got, "<i>") || !strings.Contains(got, "</i>") {
		t.Errorf("italic should use <i> tags, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_InlineCode(t *testing.T) {
	got := MarkdownToTelegramHTML("use `foo` command", TableModeDefault)
	if !strings.Contains(got, "<code>") {
		t.Errorf("inline code should use <code> tag, got %q", got)
	}
	if !strings.Contains(got, "foo") {
		t.Errorf("code content should be preserved, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_EscapeHTML(t *testing.T) {
	got := MarkdownToTelegramHTML("a < b & c > d", TableModeDefault)
	if strings.Contains(got, "<") && !strings.Contains(got, "&lt;") && !strings.Contains(got, "<b>") && !strings.Contains(got, "<i>") && !strings.Contains(got, "<code>") {
		t.Errorf("< should be escaped, got %q", got)
	}
}

func TestMarkdownToTelegramHTML_Link(t *testing.T) {
	got := MarkdownToTelegramHTML("[click](https://example.com)", TableModeDefault)
	if !strings.Contains(got, `<a href="https://example.com">`) {
		t.Errorf("link should produce <a> tag, got %q", got)
	}
	if !strings.Contains(got, "click") {
		t.Errorf("link text should be preserved, got %q", got)
	}
}

func TestMarkdownToTelegramChunks_ShortText(t *testing.T) {
	chunks := MarkdownToTelegramChunks("Hello", 100, TableModeDefault)
	if len(chunks) != 1 {
		t.Fatalf("short text should produce 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Text == "" {
		t.Error("chunk Text should be populated")
	}
	if chunks[0].HTML == "" {
		t.Error("chunk HTML should be populated")
	}
}

func TestMarkdownToTelegramChunks_Empty(t *testing.T) {
	chunks := MarkdownToTelegramChunks("", 100, TableModeDefault)
	if len(chunks) != 0 {
		t.Fatalf("empty input should produce 0 chunks, got %d", len(chunks))
	}
}

func TestMarkdownToTelegramChunks_CodeBlockIntegrity(t *testing.T) {
	md := "Before\n```\nline1\nline2\n```\nAfter"
	chunks := MarkdownToTelegramChunks(md, 5000, TableModeDefault)
	if len(chunks) == 0 {
		t.Fatal("should produce at least 1 chunk")
	}
	joined := strings.Join(func() []string {
		var s []string
		for _, c := range chunks {
			s = append(s, c.HTML)
		}
		return s
	}(), "|||")
	if !strings.Contains(joined, "line1") || !strings.Contains(joined, "line2") {
		t.Errorf("code block content should be preserved, got %q", joined)
	}
}

func TestMarkdownToTelegramChunks_LongTextSplits(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("This is a line of text for testing chunk splitting.\n")
	}
	chunks := MarkdownToTelegramChunks(sb.String(), 200, TableModeDefault)
	if len(chunks) <= 1 {
		t.Fatalf("long text should produce multiple chunks, got %d", len(chunks))
	}
}

func TestMarkdownToTelegramHTMLChunks_ReturnStrings(t *testing.T) {
	chunks := MarkdownToTelegramHTMLChunks("Hello **world**", 5000)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "<b>") {
		t.Errorf("HTML chunk should contain <b> tag, got %q", chunks[0])
	}
}

func TestRenderTelegramHTMLText_HTMLMode(t *testing.T) {
	raw := "<b>already html</b>"
	got := RenderTelegramHTMLText(raw, "html", TableModeDefault)
	if got != raw {
		t.Errorf("html mode should pass through, got %q", got)
	}
}

func TestRenderTelegramHTMLText_MarkdownMode(t *testing.T) {
	got := RenderTelegramHTMLText("**bold**", "markdown", TableModeDefault)
	if !strings.Contains(got, "<b>") {
		t.Errorf("markdown mode should convert to HTML, got %q", got)
	}
}

func TestEscapeHTML_Basic(t *testing.T) {
	got := EscapeHTML("<script>alert('xss')</script>")
	if strings.Contains(got, "<script>") {
		t.Errorf("should escape HTML tags, got %q", got)
	}
}
