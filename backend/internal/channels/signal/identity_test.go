package signal

// identity 测试 — 对齐 src/signal/identity.ts 相关逻辑

import (
	"testing"
)

func TestResolveSignalSender_Phone(t *testing.T) {
	sender := ResolveSignalSender("+15550001111", "")
	if sender == nil {
		t.Fatal("expected non-nil sender")
	}
	if sender.Kind != SignalSenderPhone {
		t.Errorf("kind = %s, want phone", sender.Kind)
	}
	if sender.E164 != "+15550001111" {
		t.Errorf("e164 = %q, want +15550001111", sender.E164)
	}
}

func TestResolveSignalSender_UUID(t *testing.T) {
	sender := ResolveSignalSender("", "123e4567-e89b-12d3-a456-426614174000")
	if sender == nil {
		t.Fatal("expected non-nil sender")
	}
	if sender.Kind != SignalSenderUUID {
		t.Errorf("kind = %s, want uuid", sender.Kind)
	}
	if sender.Raw != "123e4567-e89b-12d3-a456-426614174000" {
		t.Errorf("raw = %q", sender.Raw)
	}
}

func TestResolveSignalSender_PhonePriority(t *testing.T) {
	// 当 phone 和 UUID 都有时，优先 phone
	sender := ResolveSignalSender("+15550001111", "123e4567-e89b-12d3-a456-426614174000")
	if sender == nil {
		t.Fatal("expected non-nil sender")
	}
	if sender.Kind != SignalSenderPhone {
		t.Errorf("kind = %s, want phone (phone takes priority)", sender.Kind)
	}
}

func TestResolveSignalSender_Empty(t *testing.T) {
	sender := ResolveSignalSender("", "")
	if sender != nil {
		t.Errorf("expected nil sender, got %v", sender)
	}
}

func TestFormatSignalSenderId(t *testing.T) {
	tests := []struct {
		sender *SignalSender
		want   string
	}{
		{&SignalSender{Kind: SignalSenderPhone, E164: "+15550001111"}, "+15550001111"},
		{&SignalSender{Kind: SignalSenderUUID, Raw: "abc-123"}, "uuid:abc-123"},
	}
	for _, tt := range tests {
		got := FormatSignalSenderId(tt.sender)
		if got != tt.want {
			t.Errorf("FormatSignalSenderId(%v) = %q, want %q", tt.sender, got, tt.want)
		}
	}
}

func TestFormatSignalPairingIdLine(t *testing.T) {
	phone := &SignalSender{Kind: SignalSenderPhone, E164: "+15550001111"}
	got := FormatSignalPairingIdLine(phone)
	want := "Your Signal number: +15550001111"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	uuid := &SignalSender{Kind: SignalSenderUUID, Raw: "abc-123"}
	got2 := FormatSignalPairingIdLine(uuid)
	want2 := "Your Signal sender id: uuid:abc-123"
	if got2 != want2 {
		t.Errorf("got %q, want %q", got2, want2)
	}
}

func TestIsSignalSenderAllowed_Wildcard(t *testing.T) {
	// 对齐 TS: "allows allowlist wildcard"
	sender := &SignalSender{Kind: SignalSenderPhone, E164: "+15550002222"}
	if !IsSignalSenderAllowed(sender, []string{"*"}) {
		t.Error("wildcard should allow any sender")
	}
}

func TestIsSignalSenderAllowed_PhoneMatch(t *testing.T) {
	sender := &SignalSender{Kind: SignalSenderPhone, E164: "+15550001111"}
	if !IsSignalSenderAllowed(sender, []string{"+15550001111"}) {
		t.Error("matching phone should be allowed")
	}
}

func TestIsSignalSenderAllowed_PhoneMismatch(t *testing.T) {
	sender := &SignalSender{Kind: SignalSenderPhone, E164: "+15550001111"}
	if IsSignalSenderAllowed(sender, []string{"+15550009999"}) {
		t.Error("non-matching phone should not be allowed")
	}
}

func TestIsSignalSenderAllowed_UUIDMatch(t *testing.T) {
	// 对齐 TS: "allows allowlist when uuid sender matches"
	sender := &SignalSender{Kind: SignalSenderUUID, Raw: "123e4567-e89b-12d3-a456-426614174000"}
	if !IsSignalSenderAllowed(sender, []string{"uuid:123e4567-e89b-12d3-a456-426614174000"}) {
		t.Error("matching uuid should be allowed")
	}
}

func TestIsSignalSenderAllowed_EmptyList(t *testing.T) {
	sender := &SignalSender{Kind: SignalSenderPhone, E164: "+15550001111"}
	if IsSignalSenderAllowed(sender, nil) {
		t.Error("nil allowlist should deny")
	}
	if IsSignalSenderAllowed(sender, []string{}) {
		t.Error("empty allowlist should deny")
	}
}

func TestIsSignalGroupAllowed(t *testing.T) {
	sender := &SignalSender{Kind: SignalSenderPhone, E164: "+15550001111"}

	// 对齐 TS monitor.test.ts: "allows when policy is open"
	if !IsSignalGroupAllowed("open", nil, sender) {
		t.Error("open policy should allow")
	}

	// 对齐 TS: "blocks when policy is disabled"
	if IsSignalGroupAllowed("disabled", []string{"+15550001111"}, sender) {
		t.Error("disabled policy should block")
	}

	// 对齐 TS: "blocks allowlist when empty"
	if IsSignalGroupAllowed("allowlist", nil, sender) {
		t.Error("empty allowlist should block")
	}

	// 对齐 TS: "allows allowlist when sender matches"
	if !IsSignalGroupAllowed("allowlist", []string{"+15550001111"}, sender) {
		t.Error("matching allowlist should allow")
	}

	// 对齐 TS: "allows allowlist wildcard"
	if !IsSignalGroupAllowed("allowlist", []string{"*"}, &SignalSender{Kind: SignalSenderPhone, E164: "+15550002222"}) {
		t.Error("wildcard allowlist should allow")
	}

	// 对齐 TS: "allows allowlist when uuid sender matches"
	uuidSender := &SignalSender{Kind: SignalSenderUUID, Raw: "123e4567-e89b-12d3-a456-426614174000"}
	if !IsSignalGroupAllowed("allowlist", []string{"uuid:123e4567-e89b-12d3-a456-426614174000"}, uuidSender) {
		t.Error("uuid allowlist should allow matching sender")
	}
}

func TestParseSignalAllowEntry(t *testing.T) {
	tests := []struct {
		name  string
		input string
		kind  string
	}{
		{"wildcard", "*", "any"},
		{"phone", "+15550001111", "phone"},
		{"uuid prefix", "uuid:abc-123", "uuid"},
		{"signal prefix stripped", "signal:+15550001111", "phone"},
		{"empty", "", ""},
		{"whitespace", "  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := parseSignalAllowEntry(tt.input)
			if tt.kind == "" {
				if entry != nil {
					t.Errorf("expected nil, got %v", entry)
				}
				return
			}
			if entry == nil {
				t.Fatal("expected non-nil entry")
			}
			if entry.kind != tt.kind {
				t.Errorf("kind = %q, want %q", entry.kind, tt.kind)
			}
		})
	}
}
