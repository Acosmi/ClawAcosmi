package signal

// format 测试 — 对齐 src/signal/format.test.ts

import (
	"testing"
)

func TestMarkdownToSignalText_InlineStyles(t *testing.T) {
	// 对齐 TS: "renders inline styles"
	tests := []struct {
		name      string
		input     string
		wantText  string
		wantStyle SignalTextStyleRange
	}{
		{
			name:      "bold",
			input:     "**bold**",
			wantText:  "bold",
			wantStyle: SignalTextStyleRange{Start: 0, Length: 4, Style: StyleBold},
		},
		{
			name:      "strikethrough",
			input:     "~~strike~~",
			wantText:  "strike",
			wantStyle: SignalTextStyleRange{Start: 0, Length: 6, Style: StyleStrikethrough},
		},
		{
			name:      "monospace",
			input:     "`code`",
			wantText:  "code",
			wantStyle: SignalTextStyleRange{Start: 0, Length: 4, Style: StyleMonospace},
		},
		{
			name:      "spoiler",
			input:     "||secret||",
			wantText:  "secret",
			wantStyle: SignalTextStyleRange{Start: 0, Length: 6, Style: StyleSpoiler},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MarkdownToSignalText(tt.input)
			if result.Text != tt.wantText {
				t.Errorf("text = %q, want %q", result.Text, tt.wantText)
			}
			if len(result.TextStyles) != 1 {
				t.Fatalf("styles count = %d, want 1", len(result.TextStyles))
			}
			got := result.TextStyles[0]
			if got.Start != tt.wantStyle.Start || got.Length != tt.wantStyle.Length || got.Style != tt.wantStyle.Style {
				t.Errorf("style = {%d,%d,%s}, want {%d,%d,%s}",
					got.Start, got.Length, got.Style,
					tt.wantStyle.Start, tt.wantStyle.Length, tt.wantStyle.Style)
			}
		})
	}
}

func TestMarkdownToSignalText_BoldWithPrefix(t *testing.T) {
	// 对齐 TS: 带前缀文本的粗体
	result := MarkdownToSignalText("hello **bold**")
	if result.Text != "hello bold" {
		t.Errorf("text = %q, want %q", result.Text, "hello bold")
	}
	if len(result.TextStyles) != 1 {
		t.Fatalf("styles count = %d, want 1", len(result.TextStyles))
	}
	s := result.TextStyles[0]
	if s.Style != StyleBold {
		t.Errorf("style = %s, want BOLD", s.Style)
	}
	if s.Start != 6 {
		t.Errorf("start = %d, want 6", s.Start)
	}
	if s.Length != 4 {
		t.Errorf("length = %d, want 4", s.Length)
	}
}

func TestMarkdownToSignalText_NoStylesForPlainText(t *testing.T) {
	result := MarkdownToSignalText("plain text here")
	if result.Text != "plain text here" {
		t.Errorf("text = %q, want %q", result.Text, "plain text here")
	}
	if len(result.TextStyles) != 0 {
		t.Errorf("styles count = %d, want 0", len(result.TextStyles))
	}
}

func TestMarkdownToSignalText_EmptyInput(t *testing.T) {
	result := MarkdownToSignalText("")
	if result.Text != "" {
		t.Errorf("text = %q, want empty", result.Text)
	}
	if len(result.TextStyles) != 0 {
		t.Errorf("styles count = %d, want 0", len(result.TextStyles))
	}
}

func TestMarkdownToSignalText_UTF16Offsets(t *testing.T) {
	// 对齐 TS: "uses UTF-16 code units for offsets"
	// emoji 😀 占 2 个 UTF-16 code units
	result := MarkdownToSignalText("😀 **bold**")
	if len(result.TextStyles) != 1 {
		t.Fatalf("styles count = %d, want 1", len(result.TextStyles))
	}
	s := result.TextStyles[0]
	if s.Style != StyleBold {
		t.Errorf("style = %s, want BOLD", s.Style)
	}
	// 😀 = 2 UTF-16 units, space = 1, so bold starts at 3
	if s.Start != 3 {
		t.Errorf("start = %d, want 3 (emoji=2 + space=1)", s.Start)
	}
	if s.Length != 4 {
		t.Errorf("length = %d, want 4", s.Length)
	}
}

func TestExpandMarkdownLinks(t *testing.T) {
	// 对齐 TS: "renders links as label plus url when needed"
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"label != url", "[docs](https://example.com)", "docs (https://example.com)"},
		{"label == url", "[https://example.com](https://example.com)", "https://example.com"},
		{"empty label", "[](https://example.com)", "https://example.com"},
		{"mailto stripped", "[alice](mailto:alice@example.com)", "alice (mailto:alice@example.com)"},
		{"mailto label matches", "[alice@example.com](mailto:alice@example.com)", "alice@example.com"},
		{"no links", "plain text", "plain text"},
		{"mixed text", "see [docs](https://example.com) and [api](https://api.com)", "see docs (https://example.com) and api (https://api.com)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandMarkdownLinks(tt.input)
			if got != tt.want {
				t.Errorf("expandMarkdownLinks(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMarkdownToSignalText_WithLinks(t *testing.T) {
	// 对齐 TS: links expanded in full pipeline
	result := MarkdownToSignalText("[docs](https://example.com)")
	if result.Text != "docs (https://example.com)" {
		t.Errorf("text = %q, want %q", result.Text, "docs (https://example.com)")
	}
}

func TestUtf16Len(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"hello", 5},
		{"😀", 2},    // surrogate pair
		{"😀abc", 5}, // 2 + 3
		{"", 0},
		{"中文", 2}, // CJK = BMP, each 1 unit
		{"🎉🎉", 4}, // 2 emojis = 4 units
	}
	for _, tt := range tests {
		got := utf16Len(tt.input)
		if got != tt.want {
			t.Errorf("utf16Len(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestMarkdownToSignalTextChunks_NoSplit(t *testing.T) {
	result := MarkdownToSignalTextChunks("short", 100)
	if len(result) != 1 {
		t.Fatalf("chunks count = %d, want 1", len(result))
	}
	if result[0].Text != "short" {
		t.Errorf("text = %q, want %q", result[0].Text, "short")
	}
}

func TestMarkdownToSignalTextChunks_Split(t *testing.T) {
	input := "line1\nline2\nline3"
	result := MarkdownToSignalTextChunks(input, 8)
	if len(result) < 2 {
		t.Fatalf("chunks count = %d, want >= 2", len(result))
	}
	// 验证所有块的组合包含原文内容
	combined := ""
	for _, c := range result {
		if combined != "" {
			combined += "\n"
		}
		combined += c.Text
	}
	if combined != input {
		t.Errorf("combined = %q, want %q", combined, input)
	}
}

func TestMarkdownToSignalTextChunks_ZeroLimit(t *testing.T) {
	result := MarkdownToSignalTextChunks("hello", 0)
	if len(result) != 1 {
		t.Fatalf("chunks count = %d, want 1 (no split)", len(result))
	}
}
