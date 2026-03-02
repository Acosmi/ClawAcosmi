package scope

import (
	"testing"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

func TestResolveAckReaction_Default(t *testing.T) {
	emoji := ResolveAckReaction(nil, "any")
	if emoji != DefaultAckReaction {
		t.Errorf("default = %q, want %q", emoji, DefaultAckReaction)
	}
}

func TestResolveAckReaction_FromMessages(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Messages: &types.MessagesConfig{
			AckReaction: "✅",
		},
	}
	emoji := ResolveAckReaction(cfg, "any")
	if emoji != "✅" {
		t.Errorf("from messages = %q, want ✅", emoji)
	}
}

func TestResolveAckReaction_FromIdentity(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{
					ID: "bot",
					Identity: &types.IdentityConfig{
						Emoji: "🤖",
					},
				},
			},
		},
	}
	emoji := ResolveAckReaction(cfg, "bot")
	if emoji != "🤖" {
		t.Errorf("from identity = %q, want 🤖", emoji)
	}
}

func TestResolveIdentityNamePrefix(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{
					ID:       "bot",
					Identity: &types.IdentityConfig{Name: "Helper"},
				},
			},
		},
	}
	prefix := ResolveIdentityNamePrefix(cfg, "bot")
	if prefix != "[Helper]" {
		t.Errorf("prefix = %q, want [Helper]", prefix)
	}

	// 无身份
	prefix = ResolveIdentityNamePrefix(cfg, "missing")
	if prefix != "" {
		t.Errorf("missing = %q, want empty", prefix)
	}
}

func TestResolveMessagePrefix(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{
					ID:       "bot",
					Identity: &types.IdentityConfig{Name: "Helper"},
				},
			},
		},
	}
	// hasAllowFrom = true → empty
	p := ResolveMessagePrefix(cfg, "bot", "", true, "")
	if p != "" {
		t.Errorf("allowFrom = %q, want empty", p)
	}

	// identity prefix
	p = ResolveMessagePrefix(cfg, "bot", "", false, "")
	if p != "[Helper]" {
		t.Errorf("identity = %q, want [Helper]", p)
	}

	// fallback
	p = ResolveMessagePrefix(cfg, "missing", "", false, "fallback")
	if p != "fallback" {
		t.Errorf("fallback = %q, want fallback", p)
	}
}

func TestResolveHumanDelayConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			Defaults: &types.AgentDefaultsConfig{
				HumanDelay: &types.HumanDelayConfig{
					Mode:  "natural",
					MinMs: 800,
					MaxMs: 2500,
				},
			},
			List: []types.AgentListItemConfig{
				{
					ID: "fast",
					HumanDelay: &types.HumanDelayConfig{
						MinMs: 100,
					},
				},
			},
		},
	}
	delay := ResolveHumanDelayConfig(cfg, "fast")
	if delay == nil {
		t.Fatal("delay is nil")
	}
	if delay.Mode != "natural" {
		t.Errorf("mode = %q, want natural (from defaults)", delay.Mode)
	}
	if delay.MinMs != 100 {
		t.Errorf("minMs = %d, want 100 (overridden)", delay.MinMs)
	}
	if delay.MaxMs != 2500 {
		t.Errorf("maxMs = %d, want 2500 (from defaults)", delay.MaxMs)
	}
}
