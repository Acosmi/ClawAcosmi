package reply

import (
	"testing"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

func TestNormalizeInboundTextNewlines_CRLF(t *testing.T) {
	input := "Hello\r\nWorld\r\nFoo"
	got := NormalizeInboundTextNewlines(input)
	want := "Hello\nWorld\nFoo"
	if got != want {
		t.Errorf("CRLF normalization: got %q, want %q", got, want)
	}
}

func TestNormalizeInboundTextNewlines_MultiNewline(t *testing.T) {
	input := "Hello\n\n\n\n\nWorld"
	got := NormalizeInboundTextNewlines(input)
	want := "Hello\n\nWorld"
	if got != want {
		t.Errorf("multi-newline normalization: got %q, want %q", got, want)
	}
}

func TestNormalizeInboundTextNewlines_Empty(t *testing.T) {
	got := NormalizeInboundTextNewlines("")
	if got != "" {
		t.Errorf("empty input should return empty, got %q", got)
	}
}

func TestFormatInboundBodyWithSenderMeta_Group(t *testing.T) {
	ctx := &autoreply.MsgContext{
		IsGroup:           true,
		SenderDisplayName: "Alice",
	}
	got := FormatInboundBodyWithSenderMeta(ctx, "Hello")
	want := "Alice: Hello"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatInboundBodyWithSenderMeta_NonGroup(t *testing.T) {
	ctx := &autoreply.MsgContext{
		IsGroup:           false,
		SenderDisplayName: "Alice",
	}
	got := FormatInboundBodyWithSenderMeta(ctx, "Hello")
	if got != "Hello" {
		t.Errorf("non-group should not inject sender: got %q", got)
	}
}

func TestFormatInboundBodyWithSenderMeta_AlreadyPrefixed(t *testing.T) {
	ctx := &autoreply.MsgContext{
		IsGroup:           true,
		SenderDisplayName: "Alice",
	}
	got := FormatInboundBodyWithSenderMeta(ctx, "Alice: Hello")
	if got != "Alice: Hello" {
		t.Errorf("should not duplicate prefix: got %q", got)
	}
}

func TestFormatInboundBodyWithSenderMeta_FallbackToSenderName(t *testing.T) {
	ctx := &autoreply.MsgContext{
		IsGroup:    true,
		SenderName: "Bob",
	}
	got := FormatInboundBodyWithSenderMeta(ctx, "Hi")
	want := "Bob: Hi"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatInboundBodyWithSenderMeta_NoSender(t *testing.T) {
	ctx := &autoreply.MsgContext{IsGroup: true}
	got := FormatInboundBodyWithSenderMeta(ctx, "Hello")
	if got != "Hello" {
		t.Errorf("no sender name should not inject: got %q", got)
	}
}

func TestFinalizeInboundContext_BodyNormalization(t *testing.T) {
	ctx := &autoreply.MsgContext{
		Body:    "Hello\r\nWorld\n\n\n\nFoo",
		RawBody: "Raw\r\nText",
	}
	FinalizeInboundContext(ctx, nil)
	wantBody := "Hello\nWorld\n\nFoo"
	if ctx.Body != wantBody {
		t.Errorf("Body: got %q, want %q", ctx.Body, wantBody)
	}
	wantRaw := "Raw\nText"
	if ctx.RawBody != wantRaw {
		t.Errorf("RawBody: got %q, want %q", ctx.RawBody, wantRaw)
	}
}

func TestFinalizeInboundContext_BodyForAgent(t *testing.T) {
	ctx := &autoreply.MsgContext{
		Body: "Hello",
	}
	FinalizeInboundContext(ctx, nil)
	if ctx.BodyForAgent != "Hello" {
		t.Errorf("BodyForAgent should default to Body, got %q", ctx.BodyForAgent)
	}
}

func TestFinalizeInboundContext_BodyForCommands(t *testing.T) {
	ctx := &autoreply.MsgContext{
		Body:        "body",
		CommandBody: "/model gpt-4",
	}
	FinalizeInboundContext(ctx, nil)
	if ctx.BodyForCommands != "/model gpt-4" {
		t.Errorf("BodyForCommands should prefer CommandBody, got %q", ctx.BodyForCommands)
	}
}

func TestFinalizeInboundContext_BodyForCommandsFallback(t *testing.T) {
	ctx := &autoreply.MsgContext{
		Body:    "body text",
		RawBody: "raw body",
	}
	FinalizeInboundContext(ctx, nil)
	if ctx.BodyForCommands != "raw body" {
		t.Errorf("BodyForCommands should fallback to RawBody, got %q", ctx.BodyForCommands)
	}
}

func TestFinalizeInboundContext_UntrustedContext(t *testing.T) {
	ctx := &autoreply.MsgContext{
		UntrustedContext: []string{"Hello\r\nWorld", "", "  "},
	}
	FinalizeInboundContext(ctx, nil)
	// 空字符串会被 NormalizeInboundTextNewlines 返回为空，然后被过滤
	// "  " 不会被过滤因为它不为空
	// 只验证 CRLF 被规范化了
	if len(ctx.UntrustedContext) < 1 {
		t.Fatal("should have at least 1 item")
	}
	if ctx.UntrustedContext[0] != "Hello\nWorld" {
		t.Errorf("CRLF not normalized: got %q", ctx.UntrustedContext[0])
	}
}

func TestFinalizeInboundContext_NilCtx(t *testing.T) {
	// 不应 panic
	FinalizeInboundContext(nil, nil)
}

func TestFinalizeInboundContext_GroupSenderMeta(t *testing.T) {
	ctx := &autoreply.MsgContext{
		Body:              "Hello",
		IsGroup:           true,
		SenderDisplayName: "Alice",
	}
	FinalizeInboundContext(ctx, nil)
	if ctx.Body != "Alice: Hello" {
		t.Errorf("Body should have sender meta injected, got %q", ctx.Body)
	}
	if ctx.BodyForAgent != "Alice: Hello" {
		t.Errorf("BodyForAgent should have sender meta injected, got %q", ctx.BodyForAgent)
	}
}
