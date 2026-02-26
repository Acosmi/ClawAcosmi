package outbound

import (
	"context"
	"testing"
)

// ---------- Policy 测试 ----------

func TestEnforceCrossContextPolicy_NilToolContext(t *testing.T) {
	err := EnforceCrossContextPolicy(EnforcePolicyParams{
		Channel: "slack", Action: ActionSend, ToolContext: nil,
	})
	if err != nil {
		t.Errorf("nil ToolContext should allow: %v", err)
	}
}

func TestEnforceCrossContextPolicy_EmptyCurrentChannel(t *testing.T) {
	err := EnforceCrossContextPolicy(EnforcePolicyParams{
		Channel: "slack", Action: ActionSend,
		ToolContext: &ToolContext{CurrentChannelID: ""},
	})
	if err != nil {
		t.Errorf("empty currentChannel should allow: %v", err)
	}
}

func TestEnforceCrossContextPolicy_AllowCrossContextSend(t *testing.T) {
	err := EnforceCrossContextPolicy(EnforcePolicyParams{
		Channel: "slack", Action: ActionSend,
		Args:        map[string]interface{}{"to": "other-channel"},
		ToolContext: &ToolContext{CurrentChannelID: "my-channel"},
		Config:      &CrossContextConfig{AllowCrossContextSend: true},
	})
	if err != nil {
		t.Errorf("AllowCrossContextSend=true should allow: %v", err)
	}
}

func TestEnforceCrossContextPolicy_DenyCrossProvider(t *testing.T) {
	err := EnforceCrossContextPolicy(EnforcePolicyParams{
		Channel: "discord", Action: ActionSend,
		ToolContext: &ToolContext{CurrentChannelID: "ch1", CurrentChannelProvider: "slack"},
		Config:      &CrossContextConfig{},
	})
	if err == nil {
		t.Error("cross-provider should be denied by default")
	}
}

func TestShouldApplyCrossContextMarker(t *testing.T) {
	if !ShouldApplyCrossContextMarker(ActionSend) {
		t.Error("send should need marker")
	}
	if ShouldApplyCrossContextMarker(ActionThreadCreate) {
		t.Error("thread-create should not need marker")
	}
}

func TestApplyCrossContextDecoration(t *testing.T) {
	msg, _, _ := ApplyCrossContextDecoration("hello", CrossContextDecoration{Prefix: "[X] ", Suffix: " [/X]"}, false)
	if msg != "[X] hello [/X]" {
		t.Errorf("unexpected: %q", msg)
	}
}

// ---------- Send Service 测试 ----------

type mockSender struct{ called bool }

func (m *mockSender) Send(_ context.Context, p CoreSendParams) (*MessageSendResult, error) {
	m.called = true
	return &MessageSendResult{Success: true}, nil
}
func (m *mockSender) SendPoll(_ context.Context, p CorePollParams) (*MessagePollResult, error) {
	m.called = true
	return &MessagePollResult{Success: true}, nil
}

func TestSendService_FallbackToCore(t *testing.T) {
	sender := &mockSender{}
	svc := NewSendService(SendServiceOpts{Sender: sender})
	result, err := svc.ExecuteSendAction(context.Background(), SendActionParams{
		Ctx: OutboundSendContext{Channel: "slack"},
		To:  "user1", Message: "hi",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HandledBy != "core" {
		t.Errorf("expected core, got %s", result.HandledBy)
	}
	if !sender.called {
		t.Error("sender not called")
	}
}

func TestSendService_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := NewSendService(SendServiceOpts{Sender: &mockSender{}})
	_, err := svc.ExecuteSendAction(ctx, SendActionParams{
		Ctx: OutboundSendContext{Channel: "slack"},
		To:  "user1", Message: "hi",
	})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// ---------- Session 测试 ----------

type mockKeyBuilder struct{}

func (m *mockKeyBuilder) BuildAgentSessionKey(agentID, channel, accountID, peerKind, peerID, dmScope string) string {
	return agentID + ":" + channel + ":" + accountID + ":" + peerKind + ":" + peerID
}
func (m *mockKeyBuilder) ResolveThreadSessionKey(baseKey, threadID string) string {
	return baseKey + ":thread:" + threadID
}

func TestResolveOutboundSessionRoute_WhatsApp(t *testing.T) {
	kb := &mockKeyBuilder{}
	route, err := ResolveOutboundSessionRoute(SessionResolveParams{
		Channel: "whatsapp", AgentID: "a1", AccountID: "acc1", Target: "123456@s.whatsapp.net",
	}, kb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.PeerKind != "direct" {
		t.Errorf("expected direct, got %s", route.PeerKind)
	}
}

func TestResolveOutboundSessionRoute_WhatsAppGroup(t *testing.T) {
	kb := &mockKeyBuilder{}
	route, err := ResolveOutboundSessionRoute(SessionResolveParams{
		Channel: "whatsapp", AgentID: "a1", AccountID: "acc1", Target: "123456@g.us",
	}, kb)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if route.PeerKind != "group" {
		t.Errorf("expected group, got %s", route.PeerKind)
	}
}

func TestResolveOutboundSessionRoute_Slack(t *testing.T) {
	kb := &mockKeyBuilder{}
	route, err := ResolveOutboundSessionRoute(SessionResolveParams{
		Channel: "slack", AgentID: "a1", AccountID: "acc1", Target: "C12345",
	}, kb)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if route.PeerKind != "channel" {
		t.Errorf("expected channel, got %s", route.PeerKind)
	}
}

func TestResolveOutboundSessionRoute_SlackDM(t *testing.T) {
	kb := &mockKeyBuilder{}
	route, err := ResolveOutboundSessionRoute(SessionResolveParams{
		Channel: "slack", AgentID: "a1", AccountID: "acc1", Target: "user:U123",
	}, kb)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if route.PeerKind != "direct" {
		t.Errorf("expected direct, got %s", route.PeerKind)
	}
}

func TestResolveOutboundSessionRoute_DiscordWithThread(t *testing.T) {
	kb := &mockKeyBuilder{}
	route, err := ResolveOutboundSessionRoute(SessionResolveParams{
		Channel: "discord", AgentID: "a1", AccountID: "acc1",
		Target: "channel:123", ThreadID: "456",
	}, kb)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if route.ThreadID != "456" {
		t.Errorf("expected threadID=456, got %s", route.ThreadID)
	}
	if route.SessionKey == route.BaseSessionKey {
		t.Error("thread session key should differ from base")
	}
}

func TestResolveOutboundSessionRoute_UnsupportedChannel(t *testing.T) {
	kb := &mockKeyBuilder{}
	_, err := ResolveOutboundSessionRoute(SessionResolveParams{
		Channel: "nonexistent", Target: "foo",
	}, kb)
	if err == nil {
		t.Error("expected error for unsupported channel")
	}
}

func TestResolveOutboundSessionRoute_Telegram(t *testing.T) {
	kb := &mockKeyBuilder{}
	route, err := ResolveOutboundSessionRoute(SessionResolveParams{
		Channel: "telegram", AgentID: "a1", AccountID: "acc1", Target: "-100123",
	}, kb)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if route.PeerKind != "group" {
		t.Errorf("expected group for negative chatId, got %s", route.PeerKind)
	}
}
