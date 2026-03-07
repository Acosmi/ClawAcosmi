package gateway

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

func TestMonitorManager_StartStop(t *testing.T) {
	dctx := &ChannelDepsContext{}
	mgr := NewChannelMonitorManager(dctx, nil)

	// Start with nil config should not panic
	mgr.Start(nil)
	mgr.Stop()
}

func TestMonitorManager_ReloadSameConfig(t *testing.T) {
	dctx := &ChannelDepsContext{}
	mgr := NewChannelMonitorManager(dctx, nil)

	cfg := &types.OpenAcosmiConfig{}
	mgr.Start(cfg)
	defer mgr.Stop()

	// Reload with same config should be no-op (hash unchanged)
	mgr.Reload(cfg)
}

func TestMonitorManager_ReloadChangedConfig(t *testing.T) {
	dctx := &ChannelDepsContext{}
	mgr := NewChannelMonitorManager(dctx, nil)

	cfg1 := &types.OpenAcosmiConfig{}
	mgr.Start(cfg1)
	defer mgr.Stop()

	// Modify config — add a Discord section
	cfg2 := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{
				DiscordAccountConfig: types.DiscordAccountConfig{
					Token: types.StringPtr("test-token-123"),
				},
			},
		},
	}
	mgr.Reload(cfg2)

	// Verify hash changed (internal state)
	h1 := hashChannelConfig(cfg1)
	h2 := hashChannelConfig(cfg2)
	if h1 == h2 {
		t.Error("hashes should differ after config change")
	}
}

func TestMonitorManager_ReloadNilChannels(t *testing.T) {
	dctx := &ChannelDepsContext{}
	mgr := NewChannelMonitorManager(dctx, nil)

	mgr.Start(nil)
	defer mgr.Stop()

	// Reload with nil should be safe
	mgr.Reload(nil)
}

func TestHashChannelConfig_NilConfig(t *testing.T) {
	if h := hashChannelConfig(nil); h != "empty" {
		t.Errorf("expected 'empty', got %q", h)
	}
}

func TestHashChannelConfig_EmptyChannels(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{Channels: &types.ChannelsConfig{}}
	h := hashChannelConfig(cfg)
	if h == "empty" || h == "" {
		t.Error("non-nil Channels should produce a real hash")
	}
}

func TestHashChannelConfig_DifferentDiscord(t *testing.T) {
	cfg1 := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{DiscordAccountConfig: types.DiscordAccountConfig{Token: types.StringPtr("aaa")}},
		},
	}
	cfg2 := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{DiscordAccountConfig: types.DiscordAccountConfig{Token: types.StringPtr("bbb")}},
		},
	}
	if hashChannelConfig(cfg1) == hashChannelConfig(cfg2) {
		t.Error("different Discord configs should produce different hashes")
	}
}

func TestHashChannelConfig_IgnoresNonMonitorChannels(t *testing.T) {
	// Feishu config changes should NOT affect the hash
	cfg1 := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{DiscordAccountConfig: types.DiscordAccountConfig{Token: types.StringPtr("same")}},
		},
	}
	cfg2 := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{DiscordAccountConfig: types.DiscordAccountConfig{Token: types.StringPtr("same")}},
			Feishu:  &types.FeishuConfig{},
		},
	}
	if hashChannelConfig(cfg1) != hashChannelConfig(cfg2) {
		t.Error("Feishu changes should not affect monitor channel hash")
	}
}
