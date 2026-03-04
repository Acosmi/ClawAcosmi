package signal

// reaction_level 测试 — 对齐 src/signal/reaction-level.ts

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

func TestResolveSignalReactionLevel_Off(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					ReactionLevel: types.SignalReactionLevelOff,
				},
			},
		},
	}
	result := ResolveSignalReactionLevel(cfg, "default")
	if result.Level != types.SignalReactionLevelOff {
		t.Errorf("level = %s, want off", result.Level)
	}
	if result.AckEnabled {
		t.Error("ack should be disabled for off")
	}
	if result.AgentReactionsEnabled {
		t.Error("agent reactions should be disabled for off")
	}
}

func TestResolveSignalReactionLevel_Ack(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					ReactionLevel: types.SignalReactionLevelAck,
				},
			},
		},
	}
	result := ResolveSignalReactionLevel(cfg, "default")
	if result.Level != types.SignalReactionLevelAck {
		t.Errorf("level = %s, want ack", result.Level)
	}
	if !result.AckEnabled {
		t.Error("ack should be enabled for ack level")
	}
	if result.AgentReactionsEnabled {
		t.Error("agent reactions should be disabled for ack level")
	}
}

func TestResolveSignalReactionLevel_Minimal(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					ReactionLevel: types.SignalReactionLevelMinimal,
				},
			},
		},
	}
	result := ResolveSignalReactionLevel(cfg, "default")
	if result.Level != types.SignalReactionLevelMinimal {
		t.Errorf("level = %s, want minimal", result.Level)
	}
	if result.AckEnabled {
		t.Error("ack should be disabled for minimal")
	}
	if !result.AgentReactionsEnabled {
		t.Error("agent reactions should be enabled for minimal")
	}
	if result.AgentReactionGuidance != "minimal" {
		t.Errorf("guidance = %q, want minimal", result.AgentReactionGuidance)
	}
}

func TestResolveSignalReactionLevel_Extensive(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					ReactionLevel: types.SignalReactionLevelExtensive,
				},
			},
		},
	}
	result := ResolveSignalReactionLevel(cfg, "default")
	if result.Level != types.SignalReactionLevelExtensive {
		t.Errorf("level = %s, want extensive", result.Level)
	}
	if !result.AgentReactionsEnabled {
		t.Error("agent reactions should be enabled for extensive")
	}
	if result.AgentReactionGuidance != "extensive" {
		t.Errorf("guidance = %q, want extensive", result.AgentReactionGuidance)
	}
}

func TestResolveSignalReactionLevel_Default(t *testing.T) {
	// 无配置时默认 minimal
	cfg := &types.OpenAcosmiConfig{}
	result := ResolveSignalReactionLevel(cfg, "default")
	if result.Level != types.SignalReactionLevelMinimal {
		t.Errorf("level = %s, want minimal (default)", result.Level)
	}
}

func TestResolveSignalReactionLevel_Unknown(t *testing.T) {
	// 未知值回退到 minimal
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Signal: &types.SignalConfig{
				SignalAccountConfig: types.SignalAccountConfig{
					ReactionLevel: "unknown_value",
				},
			},
		},
	}
	result := ResolveSignalReactionLevel(cfg, "default")
	if result.Level != types.SignalReactionLevelMinimal {
		t.Errorf("level = %s, want minimal (fallback)", result.Level)
	}
}
