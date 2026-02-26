package config

import (
	"testing"
)

func TestDeriveSessionKey_Global(t *testing.T) {
	key := DeriveSessionKey(SessionScopeGlobal, MsgContext{From: "+1234"})
	if key != "global" {
		t.Errorf("expected 'global', got %q", key)
	}
}

func TestDeriveSessionKey_PerSender_Direct(t *testing.T) {
	key := DeriveSessionKey(SessionScopePerSender, MsgContext{From: "+1234567890"})
	if key != "+1234567890" {
		t.Errorf("expected '+1234567890', got %q", key)
	}
}

func TestDeriveSessionKey_PerSender_Unknown(t *testing.T) {
	key := DeriveSessionKey(SessionScopePerSender, MsgContext{})
	if key != "unknown" {
		t.Errorf("expected 'unknown', got %q", key)
	}
}

func TestDeriveSessionKey_Group(t *testing.T) {
	ctx := MsgContext{
		From:     "whatsapp:group:12345@g.us",
		ChatType: "group",
		Provider: "whatsapp",
	}
	key := DeriveSessionKey(SessionScopePerSender, ctx)
	if key == "unknown" {
		t.Errorf("expected group key, got 'unknown'")
	}
	// 群组消息应由 ResolveGroupSessionKey 返回非空 key
	t.Logf("group key: %s", key)
}

func TestResolveSessionKey_ExplicitSessionKey(t *testing.T) {
	ctx := MsgContext{
		From:       "+1234",
		SessionKey: "  Custom-Key  ",
	}
	key := ResolveSessionKey(SessionScopePerSender, ctx, "main")
	if key != "custom-key" {
		t.Errorf("expected 'custom-key', got %q", key)
	}
}

func TestResolveSessionKey_GlobalScope(t *testing.T) {
	key := ResolveSessionKey(SessionScopeGlobal, MsgContext{From: "+1234"}, "main")
	if key != "global" {
		t.Errorf("expected 'global', got %q", key)
	}
}

func TestResolveSessionKey_DirectCollapsesToMainKey(t *testing.T) {
	key := ResolveSessionKey(SessionScopePerSender, MsgContext{From: "+1234"}, "main")
	// Direct non-group sessions collapse to canonical "agent:main:main"
	if key != "agent:main:main" {
		t.Errorf("expected 'agent:main:main', got %q", key)
	}
}

func TestResolveSessionKey_CustomMainKey(t *testing.T) {
	key := ResolveSessionKey(SessionScopePerSender, MsgContext{From: "+1234"}, "mychat")
	if key != "agent:main:mychat" {
		t.Errorf("expected 'agent:main:mychat', got %q", key)
	}
}
