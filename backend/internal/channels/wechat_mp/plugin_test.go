package wechat_mp

// ============================================================================
// wechat_mp/plugin_test.go — 微信公众号 Plugin 单元测试
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-4
// ============================================================================

import (
	"testing"

	"github.com/openacosmi/claw-acismi/internal/media"
)

func TestWeChatMPPlugin_ID(t *testing.T) {
	p := NewWeChatMPPlugin()
	if p.ID() != media.ChannelWeChatMP {
		t.Errorf("ID: got %q, want %q", p.ID(), media.ChannelWeChatMP)
	}
}

func TestWeChatMPPlugin_Lifecycle(t *testing.T) {
	p := NewWeChatMPPlugin()

	// Start without config — should not fail.
	if err := p.Start("test-account"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Configure account.
	cfg := &WeChatMPConfig{
		AppID:     "test_id",
		AppSecret: "test_secret",
	}
	if err := p.ConfigureAccount("test-account", cfg); err != nil {
		t.Fatalf("ConfigureAccount: %v", err)
	}

	// Verify client and publisher exist.
	if p.GetClient("test-account") == nil {
		t.Error("expected non-nil client after ConfigureAccount")
	}
	if p.GetPublisher("test-account") == nil {
		t.Error("expected non-nil publisher after ConfigureAccount")
	}

	// Start again — should be idempotent.
	if err := p.Start("test-account"); err != nil {
		t.Fatalf("Start (second): %v", err)
	}

	// Stop.
	if err := p.Stop("test-account"); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// After stop, client and publisher should be nil.
	if p.GetClient("test-account") != nil {
		t.Error("expected nil client after Stop")
	}
	if p.GetPublisher("test-account") != nil {
		t.Error("expected nil publisher after Stop")
	}
}

func TestWeChatMPPlugin_ConfigureAccount_InvalidConfig(t *testing.T) {
	p := NewWeChatMPPlugin()

	err := p.ConfigureAccount("test", &WeChatMPConfig{})
	if err == nil {
		t.Fatal("expected error for empty config")
	}
}

func TestWeChatMPPlugin_DefaultAccountID(t *testing.T) {
	p := NewWeChatMPPlugin()

	cfg := &WeChatMPConfig{
		AppID:     "default_id",
		AppSecret: "default_secret",
	}
	if err := p.ConfigureAccount("", cfg); err != nil {
		t.Fatalf("ConfigureAccount: %v", err)
	}

	// Empty string should resolve to default.
	if p.GetClient("") == nil {
		t.Error("expected non-nil client for default account")
	}

	if err := p.Stop(""); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
