package reply

import (
	"testing"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

func TestNormalizeReplyPayload_Basic(t *testing.T) {
	payload := autoreply.ReplyPayload{Text: "  Hello World  "}
	result := NormalizeReplyPayload(payload, nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Text != "Hello World" {
		t.Errorf("got %q, want %q", result.Text, "Hello World")
	}
}

func TestNormalizeReplyPayload_HeartbeatStrip(t *testing.T) {
	payload := autoreply.ReplyPayload{
		Text: autoreply.HeartbeatToken + " actual reply",
	}
	stripped := false
	result := NormalizeReplyPayload(payload, &NormalizeReplyOptions{
		OnHeartbeatStrip: func() { stripped = true },
	})
	if !stripped {
		t.Error("OnHeartbeatStrip should have been called")
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Text != "actual reply" {
		t.Errorf("got %q, want %q", result.Text, "actual reply")
	}
}

func TestNormalizeReplyPayload_SilentReply(t *testing.T) {
	payload := autoreply.ReplyPayload{Text: autoreply.SilentReplyToken}
	skipReason := NormalizeReplySkipReason("")
	result := NormalizeReplyPayload(payload, &NormalizeReplyOptions{
		OnSkip: func(reason NormalizeReplySkipReason) { skipReason = reason },
	})
	if result != nil {
		t.Error("silent reply should return nil")
	}
	if skipReason != SkipReasonEmpty {
		t.Errorf("got skip reason %q, want %q", skipReason, SkipReasonEmpty)
	}
}

func TestNormalizeReplyPayload_EmptyTextNoMedia(t *testing.T) {
	payload := autoreply.ReplyPayload{Text: "   "}
	result := NormalizeReplyPayload(payload, nil)
	if result != nil {
		t.Error("empty text with no media should return nil")
	}
}

func TestNormalizeReplyPayload_EmptyTextWithMedia(t *testing.T) {
	payload := autoreply.ReplyPayload{Text: "", MediaURL: "https://example.com/img.png"}
	result := NormalizeReplyPayload(payload, nil)
	if result == nil {
		t.Fatal("empty text with media URL should return non-nil")
	}
}

func TestNormalizeReplyPayload_ResponsePrefix(t *testing.T) {
	payload := autoreply.ReplyPayload{Text: "Hello"}
	result := NormalizeReplyPayload(payload, &NormalizeReplyOptions{
		ResponsePrefix: "[Bot] ",
	})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Text != "[Bot] Hello" {
		t.Errorf("got %q, want %q", result.Text, "[Bot] Hello")
	}
}

func TestNormalizeReplyPayload_ResponsePrefixAlreadyPresent(t *testing.T) {
	payload := autoreply.ReplyPayload{Text: "[Bot] Hello"}
	result := NormalizeReplyPayload(payload, &NormalizeReplyOptions{
		ResponsePrefix: "[Bot] ",
	})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Text != "[Bot] Hello" {
		t.Errorf("prefix should not be duplicated, got %q", result.Text)
	}
}

func TestNormalizeReplyPayload_ResponsePrefixTemplate(t *testing.T) {
	payload := autoreply.ReplyPayload{Text: "Hello"}
	result := NormalizeReplyPayload(payload, &NormalizeReplyOptions{
		ResponsePrefix: "[{{model}}] ",
		ResponsePrefixContext: &ResponsePrefixContext{
			Model: "gpt-4",
		},
	})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Text != "[gpt-4] Hello" {
		t.Errorf("got %q, want %q", result.Text, "[gpt-4] Hello")
	}
}
