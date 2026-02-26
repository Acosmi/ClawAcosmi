package signal

// send_reactions 测试 — 对齐 src/signal/send-reactions.test.ts

import (
	"testing"
)

func TestNormalizeSignalUUID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"uuid:123e4567-e89b-12d3-a456-426614174000", "123e4567-e89b-12d3-a456-426614174000"},
		{"UUID:ABC-123", "abc-123"},
		{"plain-uuid", "plain-uuid"},
		{"", ""},
		{"  uuid:  spaced  ", "spaced"},
	}
	for _, tt := range tests {
		got := normalizeSignalUUID(tt.input)
		if got != tt.want {
			t.Errorf("normalizeSignalUUID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeSignalRecipient(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"+15550001111", "+15550001111"},
		{"signal:+15550001111", "+15550001111"},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeSignalRecipient(tt.input)
		if got != tt.want {
			t.Errorf("normalizeSignalRecipient(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveReactionTargetAuthor(t *testing.T) {
	// 对齐 TS: "uses recipients array and targetAuthor for uuid dms"
	// targetAuthorUUID 优先
	got := resolveReactionTargetAuthor("", "uuid:123e4567-e89b-12d3-a456-426614174000", "+15550001111")
	if got != "123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("uuid priority: got %q", got)
	}

	// 回退到 targetAuthor
	got2 := resolveReactionTargetAuthor("+15550002222", "", "+15550001111")
	if got2 != "+15550002222" {
		t.Errorf("author fallback: got %q", got2)
	}

	// 对齐 TS: "defaults targetAuthor to recipient for removals"
	got3 := resolveReactionTargetAuthor("", "", "+15550001111")
	if got3 != "+15550001111" {
		t.Errorf("recipient fallback: got %q", got3)
	}
}

func TestResolveReactionTargets(t *testing.T) {
	targets := ResolveReactionTargets("+15550001111", "abc-uuid")
	if len(targets) != 2 {
		t.Fatalf("targets count = %d, want 2", len(targets))
	}
	// UUID 排在前面
	if targets[0].Kind != "uuid" || targets[0].ID != "abc-uuid" {
		t.Errorf("targets[0] = %v", targets[0])
	}
	if targets[1].Kind != "phone" {
		t.Errorf("targets[1] = %v", targets[1])
	}
}

func TestResolveReactionTargets_Empty(t *testing.T) {
	targets := ResolveReactionTargets("", "")
	if len(targets) != 0 {
		t.Errorf("expected empty targets, got %v", targets)
	}
}

func TestIsSignalReactionMessage(t *testing.T) {
	if !IsSignalReactionMessage("🔥", true) {
		t.Error("valid reaction should return true")
	}
	if IsSignalReactionMessage("", true) {
		t.Error("empty emoji should return false")
	}
	if IsSignalReactionMessage("🔥", false) {
		t.Error("no timestamp should return false")
	}
}

func TestShouldEmitSignalReactionNotification(t *testing.T) {
	sender := &SignalSender{Kind: SignalSenderPhone, E164: "+15550001111"}
	targets := []SignalReactionTarget{
		{Kind: "phone", ID: "+15550009999", Display: "+15550009999"},
	}

	// off
	if ShouldEmitSignalReactionNotification("off", "", nil, nil, nil) {
		t.Error("off mode should return false")
	}
	if ShouldEmitSignalReactionNotification("", "", nil, nil, nil) {
		t.Error("empty mode should return false")
	}

	// all
	if !ShouldEmitSignalReactionNotification("all", "", nil, nil, nil) {
		t.Error("all mode should return true")
	}

	// own — target matches account
	ownTargets := []SignalReactionTarget{{Kind: "phone", ID: "+15550009999"}}
	if !ShouldEmitSignalReactionNotification("own", "+15550009999", ownTargets, sender, nil) {
		t.Error("own mode with matching target should return true")
	}
	if ShouldEmitSignalReactionNotification("own", "+15550008888", ownTargets, sender, nil) {
		t.Error("own mode with non-matching target should return false")
	}

	// allowlist
	if !ShouldEmitSignalReactionNotification("allowlist", "", targets, sender, []string{"+15550001111"}) {
		t.Error("allowlist with matching sender should return true")
	}
	if ShouldEmitSignalReactionNotification("allowlist", "", targets, sender, []string{"+15550009999"}) {
		t.Error("allowlist with non-matching sender should return false")
	}
}

func TestBuildSignalReactionSystemEventText(t *testing.T) {
	// 对齐 TS: 构建反应事件文本
	got := BuildSignalReactionSystemEventText("🔥", "Alice", "123", "Bob", "Test Group")
	want := "Alice reacted 🔥 to message 123 by Bob in Test Group"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// 无 target 和 group
	got2 := BuildSignalReactionSystemEventText("👍", "Alice", "456", "", "")
	want2 := "Alice reacted 👍 to message 456"
	if got2 != want2 {
		t.Errorf("got %q, want %q", got2, want2)
	}
}
