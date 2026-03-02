package gateway

import (
	"testing"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/internal/session"
)

// TS 对照: config/sessions/metadata.test.ts

func TestMergeOrigin_BothNil(t *testing.T) {
	if got := mergeOrigin(nil, nil); got != nil {
		t.Error("expected nil")
	}
}

func TestMergeOrigin_NextOverrides(t *testing.T) {
	existing := &session.SessionOrigin{Provider: "telegram", From: "user-1"}
	next := &session.SessionOrigin{Provider: "slack", AccountId: "acc-2"}
	got := mergeOrigin(existing, next)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Provider != "slack" {
		t.Errorf("Provider = %q; want slack", got.Provider)
	}
	if got.From != "user-1" {
		t.Errorf("From = %q; want user-1 (from existing)", got.From)
	}
	if got.AccountId != "acc-2" {
		t.Errorf("AccountId = %q; want acc-2", got.AccountId)
	}
}

func TestMergeOrigin_NilNext(t *testing.T) {
	existing := &session.SessionOrigin{Label: "test"}
	got := mergeOrigin(existing, nil)
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Label != "test" {
		t.Errorf("Label = %q", got.Label)
	}
}

func TestDeriveSessionOrigin_Full(t *testing.T) {
	ctx := &autoreply.MsgContext{
		ConversationLabel:  "Bot Chat",
		Provider:           "Telegram",
		Surface:            "telegram",
		ChatType:           "group",
		From:               "user-123",
		To:                 "bot-456",
		AccountID:          "acc-789",
		MessageThreadID:    "thread-42",
		OriginatingChannel: "",
		OriginatingTo:      "",
	}
	origin := DeriveSessionOrigin(ctx)
	if origin == nil {
		t.Fatal("expected non-nil origin")
	}
	if origin.Label != "Bot Chat" {
		t.Errorf("Label = %q", origin.Label)
	}
	if origin.Provider != "telegram" {
		t.Errorf("Provider = %q", origin.Provider)
	}
	if origin.Surface != "telegram" {
		t.Errorf("Surface = %q", origin.Surface)
	}
	if origin.ChatType != "group" {
		t.Errorf("ChatType = %q", origin.ChatType)
	}
	if origin.From != "user-123" {
		t.Errorf("From = %q", origin.From)
	}
}

func TestDeriveSessionOrigin_Nil(t *testing.T) {
	if got := DeriveSessionOrigin(nil); got != nil {
		t.Error("expected nil")
	}
}

func TestDeriveSessionOrigin_EmptyCtx(t *testing.T) {
	if got := DeriveSessionOrigin(&autoreply.MsgContext{}); got != nil {
		t.Error("expected nil for empty context")
	}
}

func TestSnapshotSessionOrigin_WithOrigin(t *testing.T) {
	entry := &SessionEntry{
		Origin: &session.SessionOrigin{Provider: "telegram", From: "user"},
	}
	snap := SnapshotSessionOrigin(entry)
	if snap == nil {
		t.Fatal("expected non-nil")
	}
	if snap.Provider != "telegram" {
		t.Errorf("Provider = %q", snap.Provider)
	}
	// 修改原始不影响快照
	entry.Origin.Provider = "slack"
	if snap.Provider != "telegram" {
		t.Error("snapshot should be independent copy")
	}
}

func TestSnapshotSessionOrigin_Nil(t *testing.T) {
	if got := SnapshotSessionOrigin(nil); got != nil {
		t.Error("expected nil")
	}
}

func TestDeriveSessionMetaPatch_WithGroup(t *testing.T) {
	ctx := &autoreply.MsgContext{
		IsGroup:     true,
		ChatType:    "group",
		Provider:    "slack",
		ChannelType: "slack",
		From:        "user-1",
	}
	patch := DeriveSessionMetaPatch(ctx, "test-session", nil)
	if patch == nil {
		t.Fatal("expected non-nil patch")
	}
	if patch.ChatType != "group" {
		t.Errorf("ChatType = %q", patch.ChatType)
	}
	if patch.Channel != "slack" {
		t.Errorf("Channel = %q", patch.Channel)
	}
}

func TestDeriveSessionMetaPatch_NoGroupNoOrigin(t *testing.T) {
	ctx := &autoreply.MsgContext{}
	if got := DeriveSessionMetaPatch(ctx, "test", nil); got != nil {
		t.Error("expected nil for empty context")
	}
}

func TestNormalizeChatType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"group", "group"},
		{"Group", "group"},
		{"channel", "channel"},
		{"supergroup", "group"},
		{"direct", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := NormalizeChatType(tt.input); got != tt.want {
			t.Errorf("NormalizeChatType(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeMessageChannel(t *testing.T) {
	if got := NormalizeMessageChannel("Telegram"); got != "telegram" {
		t.Errorf("got %q", got)
	}
	if got := NormalizeMessageChannel("  SLACK  "); got != "slack" {
		t.Errorf("got %q", got)
	}
}
