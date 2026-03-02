package main

import (
	"testing"

	"github.com/openacosmi/claw-acismi/internal/channels"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- CollectChannelStatus 测试 ----------

func TestCollectChannelStatus_Empty(t *testing.T) {
	statuses := CollectChannelStatus(nil, nil)
	if len(statuses) != 9 { // 9 known channels
		t.Errorf("expected 9 statuses, got %d", len(statuses))
	}
	for _, s := range statuses {
		if s.Configured {
			t.Errorf("%s should not be configured", s.Channel)
		}
	}
}

func TestCollectChannelStatus_WithDiscord(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{},
		},
	}
	opts := &channels.SetupChannelsOptions{}
	statuses := CollectChannelStatus(cfg, opts)

	var discordStatus *ChannelStatusEntry
	for i := range statuses {
		if statuses[i].Channel == channels.ChannelDiscord {
			discordStatus = &statuses[i]
			break
		}
	}
	if discordStatus == nil {
		t.Fatal("discord status not found")
	}
	if !discordStatus.Configured {
		t.Error("discord should be configured")
	}
}

// ---------- isChannelConfigured 测试 ----------

func TestIsChannelConfigured_NilConfig(t *testing.T) {
	if isChannelConfigured(nil, channels.ChannelDiscord) {
		t.Error("nil config should return false")
	}
}

func TestIsChannelConfigured_AllChannelTypes(t *testing.T) {
	// 通过先创建空配置再设置 Enabled，而非在 struct literal 中赋值
	// 因为 Enabled 字段在嵌入的 AccountConfig 中
	dcfg := &types.DiscordConfig{}
	tcfg := &types.TelegramConfig{}
	scfg := &types.SlackConfig{}
	wcfg := &types.WhatsAppConfig{}
	sigcfg := &types.SignalConfig{}
	imcfg := &types.IMessageConfig{}
	gcfg := &types.GoogleChatConfig{}
	mcfg := &types.MSTeamsConfig{}

	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord:    dcfg,
			Telegram:   tcfg,
			Slack:      scfg,
			WhatsApp:   wcfg,
			Signal:     sigcfg,
			IMessage:   imcfg,
			GoogleChat: gcfg,
			MSTeams:    mcfg,
		},
	}
	knownChannels := []channels.ChannelID{
		channels.ChannelDiscord,
		channels.ChannelTelegram,
		channels.ChannelSlack,
		channels.ChannelWhatsApp,
		channels.ChannelSignal,
		channels.ChannelIMessage,
		channels.ChannelGoogleChat,
		channels.ChannelMSTeams,
	}
	for _, ch := range knownChannels {
		if !isChannelConfigured(cfg, ch) {
			t.Errorf("%s should be configured", ch)
		}
	}
}

// ---------- channelQuickstartScore 测试 ----------

func TestChannelQuickstartScore(t *testing.T) {
	discordScore := channelQuickstartScore(channels.ChannelDiscord)
	telegramScore := channelQuickstartScore(channels.ChannelTelegram)
	unknownScore := channelQuickstartScore("unknown")

	if discordScore <= telegramScore {
		t.Error("discord should score higher than telegram")
	}
	if unknownScore >= telegramScore {
		t.Error("unknown should score lower than telegram")
	}
}

// ---------- ResolveQuickstartDefault 测试 ----------

func TestResolveQuickstartDefault_NotEnabled(t *testing.T) {
	opts := &channels.SetupChannelsOptions{QuickstartDefaults: false}
	statuses := CollectChannelStatus(nil, opts)
	defaults := ResolveQuickstartDefault(statuses, opts)
	if len(defaults) != 0 {
		t.Errorf("expected 0 defaults, got %d", len(defaults))
	}
}

func TestResolveQuickstartDefault_Enabled(t *testing.T) {
	opts := &channels.SetupChannelsOptions{QuickstartDefaults: true}
	statuses := CollectChannelStatus(nil, opts)
	defaults := ResolveQuickstartDefault(statuses, opts)
	if len(defaults) != 2 {
		t.Errorf("expected 2 defaults, got %d", len(defaults))
	}
	// Top 2 should be discord and slack
	if defaults[0] != channels.ChannelDiscord {
		t.Errorf("expected discord first, got %s", defaults[0])
	}
}

func TestResolveQuickstartDefault_InitialSelection(t *testing.T) {
	opts := &channels.SetupChannelsOptions{
		InitialSelection: []channels.ChannelID{channels.ChannelTelegram},
	}
	statuses := CollectChannelStatus(nil, opts)
	defaults := ResolveQuickstartDefault(statuses, opts)
	if len(defaults) != 1 || defaults[0] != channels.ChannelTelegram {
		t.Errorf("expected telegram, got %v", defaults)
	}
}

// ---------- BuildSelectionOptions 测试 ----------

func TestBuildSelectionOptions(t *testing.T) {
	statuses := CollectChannelStatus(nil, nil)
	opts := &channels.SetupChannelsOptions{}
	options := BuildSelectionOptions(statuses, opts)
	if len(options) != 9 {
		t.Errorf("expected 9 options, got %d", len(options))
	}
}

// ---------- HandleChannelChoice 测试 ----------

func TestHandleChannelChoice_Discord(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	opts := &channels.SetupChannelsOptions{}
	result, err := HandleChannelChoice(cfg, nil, channels.ChannelDiscord, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Channels == nil || result.Channels.Discord == nil {
		t.Fatal("discord config should be set")
	}
	if result.Channels.Discord.Enabled == nil || !*result.Channels.Discord.Enabled {
		t.Error("discord should be enabled")
	}
}

func TestHandleChannelChoice_UnknownChannel(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result, err := HandleChannelChoice(cfg, nil, "custom-channel", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Channels.Extra == nil {
		t.Fatal("extra should be set")
	}
	if _, ok := result.Channels.Extra["custom-channel"]; !ok {
		t.Error("custom-channel should be in extra")
	}
}

func TestHandleChannelChoice_WithAccountID(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	opts := &channels.SetupChannelsOptions{
		AccountIDs: map[channels.ChannelID]string{
			channels.ChannelTelegram: "mybot",
		},
	}
	result, err := HandleChannelChoice(cfg, nil, channels.ChannelTelegram, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Channels.Telegram == nil {
		t.Fatal("telegram should be set")
	}
}

// ---------- containsChannelID 测试 ----------

func TestContainsChannelID(t *testing.T) {
	ids := []channels.ChannelID{channels.ChannelDiscord, channels.ChannelSlack}
	if !containsChannelID(ids, channels.ChannelDiscord) {
		t.Error("should contain discord")
	}
	if containsChannelID(ids, channels.ChannelTelegram) {
		t.Error("should not contain telegram")
	}
	if containsChannelID(nil, channels.ChannelDiscord) {
		t.Error("nil slice should not contain anything")
	}
}

// ---------- disableChannel 测试 ----------

func TestDisableChannel_Discord(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{},
		},
	}
	result, err := disableChannel(cfg, channels.ChannelDiscord)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Channels.Discord == nil {
		t.Fatal("discord config should not be nil after disable")
	}
	if result.Channels.Discord.Enabled == nil || *result.Channels.Discord.Enabled {
		t.Error("discord should be disabled")
	}
}

func TestDisableChannel_NilChannels(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result, err := disableChannel(cfg, channels.ChannelDiscord)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Channels != nil {
		t.Error("channels should remain nil")
	}
}

// ---------- deleteChannelConfig 测试 ----------

func TestDeleteChannelConfig_Discord(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Discord: &types.DiscordConfig{},
		},
	}
	result, err := deleteChannelConfig(cfg, channels.ChannelDiscord)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Channels.Discord != nil {
		t.Error("discord should be nil after delete")
	}
}

func TestDeleteChannelConfig_ExtraChannel(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Channels: &types.ChannelsConfig{
			Extra: map[string]interface{}{
				"custom": map[string]interface{}{"enabled": true},
			},
		},
	}
	result, err := deleteChannelConfig(cfg, "custom")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Channels.Extra["custom"]; ok {
		t.Error("custom channel should be deleted from extra")
	}
}

// ---------- channelLabel 测试 ----------

func TestChannelLabel(t *testing.T) {
	if channelLabel(channels.ChannelDiscord) != "Discord" {
		t.Error("discord label mismatch")
	}
	if channelLabel("unknown-chan") != "unknown-chan" {
		t.Error("unknown channel should return raw ID")
	}
}

// ---------- configureNewChannel 测试 ----------

func TestConfigureNewChannel_Telegram(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	result, err := configureNewChannel(cfg, nil, channels.ChannelTelegram, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Channels == nil || result.Channels.Telegram == nil {
		t.Fatal("telegram should be configured")
	}
	if result.Channels.Telegram.Enabled == nil || !*result.Channels.Telegram.Enabled {
		t.Error("telegram should be enabled")
	}
}
