package media

import (
	"testing"
)

// TestSplitMedia_FencedCodeBlock 围栏代码块内的 MEDIA: 不被截获。
func TestSplitMedia_FencedCodeBlock(t *testing.T) {
	input := "Here is code:\n```\nMEDIA:./image.png\n```\nDone"
	result := SplitMediaFromOutput(input)
	if len(result.MediaURLs) != 0 {
		t.Errorf("expected no media urls inside fenced block, got %v", result.MediaURLs)
	}
	if result.Text != input {
		t.Errorf("expected original text preserved, got %q", result.Text)
	}
}

// TestSplitMedia_OutsideFence 围栏代码块外的 MEDIA: 正常截获。
func TestSplitMedia_OutsideFence(t *testing.T) {
	input := "```\nsome code\n```\nMEDIA:./screenshot.png"
	result := SplitMediaFromOutput(input)
	if len(result.MediaURLs) != 1 || result.MediaURLs[0] != "./screenshot.png" {
		t.Errorf("expected [./screenshot.png], got %v", result.MediaURLs)
	}
}

// TestSplitMedia_MixedFenceAndMedia 围栏块前后混合 MEDIA:。
func TestSplitMedia_MixedFenceAndMedia(t *testing.T) {
	input := "MEDIA:./before.png\n```\nMEDIA:./inside.png\n```\nMEDIA:./after.png"
	result := SplitMediaFromOutput(input)
	if len(result.MediaURLs) != 2 {
		t.Fatalf("expected 2 media urls, got %d: %v", len(result.MediaURLs), result.MediaURLs)
	}
	if result.MediaURLs[0] != "./before.png" {
		t.Errorf("expected first url ./before.png, got %s", result.MediaURLs[0])
	}
	if result.MediaURLs[1] != "./after.png" {
		t.Errorf("expected second url ./after.png, got %s", result.MediaURLs[1])
	}
}

// TestSplitMedia_UnterminatedFence 未关闭围栏块中的 MEDIA: 不被截获。
func TestSplitMedia_UnterminatedFence(t *testing.T) {
	input := "```\nMEDIA:./should-not-match.png"
	result := SplitMediaFromOutput(input)
	if len(result.MediaURLs) != 0 {
		t.Errorf("expected no media urls in unterminated fence, got %v", result.MediaURLs)
	}
}

// TestSplitMedia_TildeFence ~~~ 围栏也正确跳过。
func TestSplitMedia_TildeFence(t *testing.T) {
	input := "~~~\nMEDIA:./image.png\n~~~\nok"
	result := SplitMediaFromOutput(input)
	if len(result.MediaURLs) != 0 {
		t.Errorf("expected no media urls inside tilde fence, got %v", result.MediaURLs)
	}
}

// TestSplitMedia_AudioAsVoice 检测 [[audio_as_voice]] 标签。
func TestSplitMedia_AudioAsVoice(t *testing.T) {
	result := SplitMediaFromOutput("Hello [[audio_as_voice]] world")
	if !result.AudioAsVoice {
		t.Error("expected AudioAsVoice=true")
	}
	if result.Text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", result.Text)
	}
}

// TestSplitMedia_RejectsAbsolutePaths 拒绝绝对路径。
func TestSplitMedia_RejectsAbsolutePaths(t *testing.T) {
	result := SplitMediaFromOutput("MEDIA:/Users/pete/My File.png")
	if len(result.MediaURLs) != 0 {
		t.Errorf("expected no media urls for abs path, got %v", result.MediaURLs)
	}
}

// TestSplitMedia_RejectsTraversal 拒绝目录穿越。
func TestSplitMedia_RejectsTraversal(t *testing.T) {
	result := SplitMediaFromOutput("MEDIA:../../etc/passwd")
	if len(result.MediaURLs) != 0 {
		t.Errorf("expected no media urls for traversal, got %v", result.MediaURLs)
	}
}

// TestSplitMedia_SafeRelativePath 安全相对路径正常截获。
func TestSplitMedia_SafeRelativePath(t *testing.T) {
	result := SplitMediaFromOutput("MEDIA:./screenshots/image.png")
	if len(result.MediaURLs) != 1 || result.MediaURLs[0] != "./screenshots/image.png" {
		t.Errorf("expected [./screenshots/image.png], got %v", result.MediaURLs)
	}
	if result.Text != "" {
		t.Errorf("expected empty text, got %q", result.Text)
	}
}

// TestSplitMedia_LeadingWhitespace 带前置空白的 MEDIA:。
func TestSplitMedia_LeadingWhitespace(t *testing.T) {
	result := SplitMediaFromOutput("  MEDIA:./screenshot.png")
	if len(result.MediaURLs) != 1 || result.MediaURLs[0] != "./screenshot.png" {
		t.Errorf("expected [./screenshot.png], got %v", result.MediaURLs)
	}
}

// TestSplitMedia_HTTPURL HTTP URL 正常截获。
func TestSplitMedia_HTTPURL(t *testing.T) {
	result := SplitMediaFromOutput("MEDIA:https://example.com/img.png")
	if len(result.MediaURLs) != 1 || result.MediaURLs[0] != "https://example.com/img.png" {
		t.Errorf("expected [https://example.com/img.png], got %v", result.MediaURLs)
	}
}

// TestSplitMedia_FencedBlockWithLanguage 带语言标记的围栏块跳过。
func TestSplitMedia_FencedBlockWithLanguage(t *testing.T) {
	input := "```python\nprint('MEDIA:./fake.png')\n```"
	result := SplitMediaFromOutput(input)
	if len(result.MediaURLs) != 0 {
		t.Errorf("expected no media urls inside fenced block with lang, got %v", result.MediaURLs)
	}
}
