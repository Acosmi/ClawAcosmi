package config

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ============================================================
// IsChannelConfigured 频道检测测试
// ============================================================

func TestIsChannelConfigured_TelegramByEnv(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	env := func(key string) string {
		if key == "TELEGRAM_BOT_TOKEN" {
			return "123:ABC"
		}
		return ""
	}
	if !IsChannelConfigured(cfg, "telegram", env) {
		t.Error("telegram should be configured via env")
	}
}

func TestIsChannelConfigured_TelegramByConfig(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Telegram: &types.TelegramConfig{
				TelegramAccountConfig: types.TelegramAccountConfig{BotToken: "test-token"},
			},
		},
	}
	noEnv := func(string) string { return "" }
	if !IsChannelConfigured(cfg, "telegram", noEnv) {
		t.Error("telegram should be configured via config")
	}
}

func TestIsChannelConfigured_NotConfigured(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	noEnv := func(string) string { return "" }
	if IsChannelConfigured(cfg, "telegram", noEnv) {
		t.Error("telegram should not be configured")
	}
}

func TestIsChannelConfigured_DiscordByEnv(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	env := func(key string) string {
		if key == "DISCORD_BOT_TOKEN" {
			return "discord-token"
		}
		return ""
	}
	if !IsChannelConfigured(cfg, "discord", env) {
		t.Error("discord should be configured via env")
	}
}

func TestIsChannelConfigured_SlackByEnv(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	env := func(key string) string {
		if key == "SLACK_BOT_TOKEN" {
			return "xoxb-test"
		}
		return ""
	}
	if !IsChannelConfigured(cfg, "slack", env) {
		t.Error("slack should be configured via env")
	}
}

func TestIsChannelConfigured_WhatsApp(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			WhatsApp: &types.WhatsAppConfig{},
		},
	}
	noEnv := func(string) string { return "" }
	if !IsChannelConfigured(cfg, "whatsapp", noEnv) {
		t.Error("whatsapp should be configured")
	}
}

// ============================================================
// isProviderConfigured 测试
// ============================================================

func TestIsProviderConfigured_ByAuthProfile(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Auth: &types.AuthConfig{
			Profiles: map[string]*types.AuthProfileConfig{
				"main": {Provider: "google-antigravity", Mode: "oauth"},
			},
		},
	}
	if !isProviderConfigured(cfg, "google-antigravity") {
		t.Error("provider should be configured via auth profile")
	}
}

func TestIsProviderConfigured_ByModelRef(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			Defaults: &types.AgentDefaultsConfig{
				Model: &types.AgentModelListConfig{
					Primary: "anthropic/claude-sonnet-4-5",
				},
			},
		},
	}
	if !isProviderConfigured(cfg, "anthropic") {
		t.Error("provider should be configured via model ref")
	}
}

func TestIsProviderConfigured_NotConfigured(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	if isProviderConfigured(cfg, "nonexistent") {
		t.Error("provider should not be configured")
	}
}

// ============================================================
// ApplyPluginAutoEnable 主函数测试
// ============================================================

func TestApplyPluginAutoEnable_DetectsConfiguredChannels(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Telegram: &types.TelegramConfig{TelegramAccountConfig: types.TelegramAccountConfig{BotToken: "test"}},
		},
	}
	noEnv := func(string) string { return "" }
	result := ApplyPluginAutoEnable(cfg, noEnv)

	if len(result.Changes) == 0 {
		t.Fatal("should detect telegram as configured")
	}

	// 验证插件条目已注册
	if cfg.Plugins == nil || cfg.Plugins.Entries == nil {
		t.Fatal("plugins entries should be created")
	}
	entry := cfg.Plugins.Entries["telegram"]
	if entry == nil {
		t.Fatal("telegram entry should be registered")
	}
}

func TestApplyPluginAutoEnable_SkipsDenied(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Telegram: &types.TelegramConfig{TelegramAccountConfig: types.TelegramAccountConfig{BotToken: "test"}},
		},
		Plugins: &types.PluginsConfig{
			Deny: []string{"telegram"},
		},
	}
	noEnv := func(string) string { return "" }
	result := ApplyPluginAutoEnable(cfg, noEnv)

	for _, change := range result.Changes {
		if change == "telegram configured, not enabled yet." {
			t.Error("should not auto-enable denied plugin")
		}
	}
}

func TestApplyPluginAutoEnable_SkipsExplicitlyDisabled(t *testing.T) {
	disabled := false
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{},
		},
		Plugins: &types.PluginsConfig{
			Entries: map[string]*types.PluginEntryConfig{
				"discord": {Enabled: &disabled},
			},
		},
	}
	noEnv := func(string) string { return "" }
	result := ApplyPluginAutoEnable(cfg, noEnv)

	for _, change := range result.Changes {
		if change == "discord configured, not enabled yet." {
			t.Error("should not auto-enable explicitly disabled plugin")
		}
	}
}

func TestApplyPluginAutoEnable_GlobalDisabled(t *testing.T) {
	disabled := false
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Telegram: &types.TelegramConfig{TelegramAccountConfig: types.TelegramAccountConfig{BotToken: "test"}},
		},
		Plugins: &types.PluginsConfig{
			Enabled: &disabled,
		},
	}
	noEnv := func(string) string { return "" }
	result := ApplyPluginAutoEnable(cfg, noEnv)
	if len(result.Changes) != 0 {
		t.Error("should not auto-enable when plugins globally disabled")
	}
}

func TestApplyPluginAutoEnable_NoConfigured(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	noEnv := func(string) string { return "" }
	result := ApplyPluginAutoEnable(cfg, noEnv)
	if len(result.Changes) != 0 {
		t.Error("should have no changes with empty config")
	}
}

func TestApplyPluginAutoEnable_EnsuresAllowlist(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Telegram: &types.TelegramConfig{TelegramAccountConfig: types.TelegramAccountConfig{BotToken: "test"}},
		},
		Plugins: &types.PluginsConfig{
			Allow: []string{"other-plugin"},
		},
	}
	noEnv := func(string) string { return "" }
	ApplyPluginAutoEnable(cfg, noEnv)

	found := false
	for _, id := range cfg.Plugins.Allow {
		if id == "telegram" {
			found = true
		}
	}
	if !found {
		t.Error("telegram should be added to allow list")
	}
}
