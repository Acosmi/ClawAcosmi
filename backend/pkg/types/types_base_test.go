package types

import "testing"

// === chat-type.ts 等价测试 ===

func TestNormalizeChatType(t *testing.T) {
	tests := []struct {
		input  string
		want   ChatType
		wantOK bool
	}{
		{"direct", ChatDirect, true},
		{"dm", ChatDirect, true}, // "dm" 映射为 "direct"
		{"group", ChatGroup, true},
		{"channel", ChatChannel, true},
		{"unknown", "", false},
		{"", "", false},
		{"Direct", "", false}, // 区分大小写，与原版行为一致
	}

	for _, tt := range tests {
		got, ok := NormalizeChatType(tt.input)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("NormalizeChatType(%q) = (%q, %v), want (%q, %v)",
				tt.input, got, ok, tt.want, tt.wantOK)
		}
	}
}

// === types.base.ts 类型常量验证 ===

func TestReplyModeValues(t *testing.T) {
	if ReplyText != "text" || ReplyCommand != "command" {
		t.Error("ReplyMode 常量值异常")
	}
}

func TestTypingModeValues(t *testing.T) {
	modes := []TypingMode{TypingNever, TypingInstant, TypingThinking, TypingMessage}
	expected := []string{"never", "instant", "thinking", "message"}
	for i, m := range modes {
		if string(m) != expected[i] {
			t.Errorf("TypingMode[%d] = %q, want %q", i, m, expected[i])
		}
	}
}

func TestSessionScopeValues(t *testing.T) {
	if SessionScopePerSender != "per-sender" || SessionScopeGlobal != "global" {
		t.Error("SessionScope 常量值异常")
	}
}

func TestDmScopeValues(t *testing.T) {
	scopes := []DmScope{DmScopeMain, DmScopePerPeer, DmScopePerChannelPeer, DmScopePerAccountChanPeer}
	expected := []string{"main", "per-peer", "per-channel-peer", "per-account-channel-peer"}
	for i, s := range scopes {
		if string(s) != expected[i] {
			t.Errorf("DmScope[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestGroupPolicyValues(t *testing.T) {
	if GroupOpen != "open" || GroupDisabled != "disabled" || GroupAllowlist != "allowlist" {
		t.Error("GroupPolicy 常量值异常")
	}
}

func TestDmPolicyValues(t *testing.T) {
	policies := []DmPolicy{DmPairing, DmAllowlist, DmOpen, DmDisabled}
	expected := []string{"pairing", "allowlist", "open", "disabled"}
	for i, p := range policies {
		if string(p) != expected[i] {
			t.Errorf("DmPolicy[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestLogLevelValues(t *testing.T) {
	levels := []LogLevel{LogSilent, LogFatal, LogError, LogWarn, LogInfo, LogDebug, LogTrace}
	expected := []string{"silent", "fatal", "error", "warn", "info", "debug", "trace"}
	for i, l := range levels {
		if string(l) != expected[i] {
			t.Errorf("LogLevel[%d] = %q, want %q", i, l, expected[i])
		}
	}
}

func TestSessionResetModeValues(t *testing.T) {
	if SessionResetDaily != "daily" || SessionResetIdle != "idle" {
		t.Error("SessionResetMode 常量值异常")
	}
}

func TestMarkdownTableModeValues(t *testing.T) {
	if MarkdownTableOff != "off" || MarkdownTableBullets != "bullets" || MarkdownTableCode != "code" {
		t.Error("MarkdownTableMode 常量值异常")
	}
}

func TestChannelTypeValues(t *testing.T) {
	channels := []ChannelType{ChannelDiscord, ChannelSlack, ChannelTelegram, ChannelWhatsApp, ChannelSignal, ChannelIMessage, ChannelLine, ChannelWeb, ChannelFeishu, ChannelDingTalk, ChannelWeCom}
	expected := []string{"discord", "slack", "telegram", "whatsapp", "signal", "imessage", "line", "web", "feishu", "dingtalk", "wecom"}
	for i, c := range channels {
		if string(c) != expected[i] {
			t.Errorf("ChannelType[%d] = %q, want %q", i, c, expected[i])
		}
	}
}
