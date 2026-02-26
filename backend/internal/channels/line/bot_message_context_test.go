package line

import (
	"testing"
)

func TestBuildBotMessageContext_DirectMessage(t *testing.T) {
	event := &LineInboundEvent{
		Type:        LineEventMessage,
		SourceType:  LineSourceUser,
		SourceID:    "U123456",
		UserID:      "U123456",
		ReplyToken:  "reply-tok",
		MessageType: "text",
		MessageText: "Hello!",
	}
	ctx := BuildBotMessageContext(event, "bot-channel")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.ChatType != "direct" {
		t.Errorf("ChatType = %q; want direct", ctx.ChatType)
	}
	if ctx.IsGroup {
		t.Error("expected IsGroup=false for user source")
	}
	if ctx.From != "U123456" {
		t.Errorf("From = %q", ctx.From)
	}
	if ctx.To != "bot-channel" {
		t.Errorf("To = %q", ctx.To)
	}
	if ctx.Body != "Hello!" {
		t.Errorf("Body = %q", ctx.Body)
	}
	if ctx.Channel != "line" {
		t.Errorf("Channel = %q", ctx.Channel)
	}
}

func TestBuildBotMessageContext_GroupMessage(t *testing.T) {
	event := &LineInboundEvent{
		Type:        LineEventMessage,
		SourceType:  LineSourceGroup,
		SourceID:    "G987654",
		UserID:      "U111222",
		ReplyToken:  "reply-tok-2",
		MessageType: "text",
		MessageText: "Group msg",
	}
	ctx := BuildBotMessageContext(event, "bot-ch-2")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if ctx.ChatType != "group" {
		t.Errorf("ChatType = %q; want group", ctx.ChatType)
	}
	if !ctx.IsGroup {
		t.Error("expected IsGroup=true for group source")
	}
	if ctx.GroupID != "G987654" {
		t.Errorf("GroupID = %q", ctx.GroupID)
	}
	if ctx.SenderID != "U111222" {
		t.Errorf("SenderID = %q", ctx.SenderID)
	}
}

func TestBuildBotMessageContext_Nil(t *testing.T) {
	if ctx := BuildBotMessageContext(nil, "bot"); ctx != nil {
		t.Error("expected nil for nil event")
	}
}
