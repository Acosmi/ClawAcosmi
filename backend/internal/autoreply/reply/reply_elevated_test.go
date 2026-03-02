package reply

import (
	"strings"
	"testing"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// TS 对照: reply-elevated.ts 单元测试

func TestNormalizeAllowToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Alice", "alice"},
		{"  BOB  ", "bob"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeAllowToken(tt.input); got != tt.want {
			t.Errorf("normalizeAllowToken(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestSlugAllowToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"@Alice_Bob", "alice-bob"},
		{"##Hello World", "hello-world"},
		{"user@email.com", "user-email-com"},
		{"  ", ""},
		{"simple", "simple"},
	}
	for _, tt := range tests {
		if got := slugAllowToken(tt.input); got != tt.want {
			t.Errorf("slugAllowToken(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripSenderPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"telegram:12345", "12345"},
		{"Telegram:12345", "12345"},
		{"user:alice", "alice"},
		{"discord:guildid", "guildid"},
		{"noprefix", "noprefix"},
		{"", ""},
		{"  whatsapp:+1234  ", "+1234"},
	}
	for _, tt := range tests {
		if got := stripSenderPrefix(tt.input); got != tt.want {
			t.Errorf("stripSenderPrefix(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func boolPtr(b bool) *bool { return &b }

func TestResolveElevatedPermissions_BothEnabled(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Elevated: &types.ToolsElevatedConfig{
				Enabled: boolPtr(true),
				AllowFrom: types.AgentElevatedAllowFromConfig{
					"telegram": {interface{}("alice")},
				},
			},
		},
	}
	ctx := &autoreply.MsgContext{
		SenderName: "alice",
		From:       "telegram:alice",
	}
	result := ResolveElevatedPermissions(cfg, "mybot", ctx, "telegram")
	if !result.Enabled {
		t.Error("expected enabled=true")
	}
	if !result.Allowed {
		t.Errorf("expected allowed=true, got failures=%v", result.Failures)
	}
}

func TestResolveElevatedPermissions_GlobalDisabled(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Elevated: &types.ToolsElevatedConfig{
				Enabled: boolPtr(false),
			},
		},
	}
	ctx := &autoreply.MsgContext{SenderName: "alice"}
	result := ResolveElevatedPermissions(cfg, "mybot", ctx, "telegram")
	if result.Enabled {
		t.Error("expected enabled=false")
	}
	if result.Allowed {
		t.Error("expected allowed=false")
	}
	if len(result.Failures) == 0 {
		t.Error("expected failure entries")
	}
}

func TestResolveElevatedPermissions_NoProvider(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Elevated: &types.ToolsElevatedConfig{
				Enabled: boolPtr(true),
			},
		},
	}
	ctx := &autoreply.MsgContext{SenderName: "alice"}
	result := ResolveElevatedPermissions(cfg, "mybot", ctx, "")
	if !result.Enabled {
		t.Error("expected enabled=true")
	}
	if result.Allowed {
		t.Error("expected allowed=false when provider is empty")
	}
	found := false
	for _, f := range result.Failures {
		if f.Gate == "provider" {
			found = true
		}
	}
	if !found {
		t.Error("expected provider failure gate")
	}
}

func TestResolveElevatedPermissions_WildcardAllow(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Elevated: &types.ToolsElevatedConfig{
				AllowFrom: types.AgentElevatedAllowFromConfig{
					"telegram": {interface{}("*")},
				},
			},
		},
	}
	ctx := &autoreply.MsgContext{SenderName: "anyone"}
	result := ResolveElevatedPermissions(cfg, "mybot", ctx, "telegram")
	if !result.Allowed {
		t.Errorf("expected allowed=true with wildcard, failures=%v", result.Failures)
	}
}

func TestResolveElevatedPermissions_SenderNotInList(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Elevated: &types.ToolsElevatedConfig{
				AllowFrom: types.AgentElevatedAllowFromConfig{
					"telegram": {interface{}("bob")},
				},
			},
		},
	}
	ctx := &autoreply.MsgContext{SenderName: "charlie"}
	result := ResolveElevatedPermissions(cfg, "mybot", ctx, "telegram")
	if result.Allowed {
		t.Error("expected allowed=false when sender not in allowFrom list")
	}
}

func TestResolveElevatedPermissions_NilConfig(t *testing.T) {
	// 当 tools/elevated 为 nil 时应默认 enabled=true
	cfg := &types.OpenAcosmiConfig{}
	ctx := &autoreply.MsgContext{SenderName: "alice"}
	result := ResolveElevatedPermissions(cfg, "mybot", ctx, "telegram")
	if !result.Enabled {
		t.Error("expected enabled=true when config is nil (default)")
	}
	// 但 allowFrom 为空，所以 allowed=false
	if result.Allowed {
		t.Error("expected allowed=false when no allowFrom configured")
	}
}

func TestFormatElevatedUnavailableMessage(t *testing.T) {
	msg := FormatElevatedUnavailableMessage(true, []ElevatedFailure{
		{Gate: "enabled", Key: "tools.elevated.enabled"},
	}, "test-session")
	if !strings.Contains(msg, "sandboxed") {
		t.Error("expected 'sandboxed' in message")
	}
	if !strings.Contains(msg, "tools.elevated.enabled") {
		t.Error("expected failure key in message")
	}
	if !strings.Contains(msg, "test-session") {
		t.Error("expected session key in message")
	}
}

func TestFormatElevatedUnavailableMessage_NoFailures(t *testing.T) {
	msg := FormatElevatedUnavailableMessage(false, nil, "")
	if !strings.Contains(msg, "direct") {
		t.Error("expected 'direct' in message")
	}
	if strings.Contains(msg, "openacosmi sandbox") {
		t.Error("should not contain sandbox command when no session")
	}
}
