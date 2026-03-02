package website

// ============================================================================
// website/plugin_test.go — 自有网站频道插件单元测试
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P4-2
// ============================================================================

import (
	"testing"
)

func TestNewWebsitePlugin(t *testing.T) {
	p := NewWebsitePlugin()
	if p == nil {
		t.Fatal("NewWebsitePlugin returned nil")
	}
	if p.ID() != ChannelWebsite {
		t.Errorf("ID: got %q, want %q", p.ID(), ChannelWebsite)
	}
}

func TestWebsitePlugin_ConfigureAndGet(t *testing.T) {
	p := NewWebsitePlugin()
	cfg := &WebsiteConfig{
		Enabled:        true,
		APIURL:         "https://example.com/api/posts",
		AuthType_:      AuthBearer,
		AuthToken:      "tok",
		TimeoutSeconds: 10,
	}

	if err := p.ConfigureAccount("acc1", cfg); err != nil {
		t.Fatalf("ConfigureAccount: %v", err)
	}

	client := p.GetClient("acc1")
	if client == nil {
		t.Fatal("GetClient returned nil after ConfigureAccount")
	}
}

func TestWebsitePlugin_ConfigureInvalid(t *testing.T) {
	p := NewWebsitePlugin()
	cfg := &WebsiteConfig{
		Enabled: true,
		// Missing APIURL, AuthType, AuthToken
	}

	err := p.ConfigureAccount("acc1", cfg)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

func TestWebsitePlugin_StartWithoutConfigure(t *testing.T) {
	p := NewWebsitePlugin()
	// Start without configure should not error, just warn.
	if err := p.Start("acc1"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if p.GetClient("acc1") != nil {
		t.Error("expected nil client before ConfigureAccount")
	}
}

func TestWebsitePlugin_StopCleansUp(t *testing.T) {
	p := NewWebsitePlugin()
	cfg := &WebsiteConfig{
		Enabled:        true,
		APIURL:         "https://example.com/api",
		AuthType_:      AuthBearer,
		AuthToken:      "tok",
		TimeoutSeconds: 10,
	}
	_ = p.ConfigureAccount("acc1", cfg)

	if err := p.Stop("acc1"); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if p.GetClient("acc1") != nil {
		t.Error("expected nil client after Stop")
	}
}

func TestWebsitePlugin_DefaultAccountID(t *testing.T) {
	p := NewWebsitePlugin()
	cfg := &WebsiteConfig{
		Enabled:        true,
		APIURL:         "https://example.com/api",
		AuthType_:      AuthBearer,
		AuthToken:      "tok",
		TimeoutSeconds: 10,
	}
	// Empty string should resolve to default.
	_ = p.ConfigureAccount("", cfg)

	// GetClient with empty should also resolve to default.
	client := p.GetClient("")
	if client == nil {
		t.Fatal("expected client for default account")
	}
}
