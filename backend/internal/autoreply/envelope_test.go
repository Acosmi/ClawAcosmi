package autoreply

import (
	"strings"
	"testing"
	"time"
)

// ---------- F2: FormatTimeAgo ----------

func TestFormatTimeAgo_JustNow(t *testing.T) {
	got := FormatTimeAgo(30 * time.Second)
	if got != "just now" {
		t.Errorf("got %q, want 'just now'", got)
	}
}

func TestFormatTimeAgo_Minutes(t *testing.T) {
	got := FormatTimeAgo(5 * time.Minute)
	if got != "5m ago" {
		t.Errorf("got %q, want '5m ago'", got)
	}
}

func TestFormatTimeAgo_Hours(t *testing.T) {
	got := FormatTimeAgo(3 * time.Hour)
	if got != "3h ago" {
		t.Errorf("got %q, want '3h ago'", got)
	}
}

func TestFormatTimeAgo_Days(t *testing.T) {
	got := FormatTimeAgo(48 * time.Hour)
	if got != "2d ago" {
		t.Errorf("got %q, want '2d ago'", got)
	}
}

// ---------- F2: FormatInboundFromLabel ----------

func TestFormatInboundFromLabel_Direct(t *testing.T) {
	got := FormatInboundFromLabel(false, "", "", "Alice", "", "")
	if got != "Alice" {
		t.Errorf("got %q, want Alice", got)
	}
}

func TestFormatInboundFromLabel_DirectWithId(t *testing.T) {
	got := FormatInboundFromLabel(false, "", "", "Alice", "u123", "")
	if got != "Alice id:u123" {
		t.Errorf("got %q, want 'Alice id:u123'", got)
	}
}

func TestFormatInboundFromLabel_DirectSameId(t *testing.T) {
	got := FormatInboundFromLabel(false, "", "", "Alice", "Alice", "")
	if got != "Alice" {
		t.Errorf("got %q, want Alice (id same as label)", got)
	}
}

func TestFormatInboundFromLabel_Group(t *testing.T) {
	got := FormatInboundFromLabel(true, "Dev Chat", "g123", "", "", "")
	if got != "Dev Chat id:g123" {
		t.Errorf("got %q, want 'Dev Chat id:g123'", got)
	}
}

func TestFormatInboundFromLabel_GroupNoId(t *testing.T) {
	got := FormatInboundFromLabel(true, "Dev Chat", "", "", "", "")
	if got != "Dev Chat" {
		t.Errorf("got %q, want 'Dev Chat'", got)
	}
}

func TestFormatInboundFromLabel_GroupFallback(t *testing.T) {
	got := FormatInboundFromLabel(true, "", "", "", "", "MyGroup")
	if got != "MyGroup" {
		t.Errorf("got %q, want MyGroup", got)
	}
}

func TestFormatInboundFromLabel_GroupDefault(t *testing.T) {
	got := FormatInboundFromLabel(true, "", "", "", "", "")
	if got != "Group" {
		t.Errorf("got %q, want 'Group'", got)
	}
}

// ---------- F2: BuildEnvelopeHeaderWithOptions ----------

func TestBuildEnvelopeHeaderWithOptions_UTC(t *testing.T) {
	ctx := &MsgContext{SenderDisplayName: "Bob", ChannelType: "dm"}
	opts := EnvelopeFormatOptions{
		TimezoneMode:     EnvelopeTZUTC,
		IncludeTimestamp: true,
		IncludeSender:    true,
	}
	header := BuildEnvelopeHeaderWithOptions(ctx, "", opts, time.Time{})
	if header.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
	if header.SenderLabel != "Bob" {
		t.Errorf("sender = %q, want Bob", header.SenderLabel)
	}
}

func TestBuildEnvelopeHeaderWithOptions_Elapsed(t *testing.T) {
	ctx := &MsgContext{SenderDisplayName: "Bob"}
	opts := EnvelopeFormatOptions{
		IncludeTimestamp: true,
		IncludeElapsed:   true,
		IncludeSender:    true,
	}
	msgTime := time.Now().Add(-5 * time.Minute)
	header := BuildEnvelopeHeaderWithOptions(ctx, "", opts, msgTime)
	if header.Elapsed == "" {
		t.Error("expected non-empty elapsed")
	}
}

func TestFormatEnvelopeHeader_WithElapsed(t *testing.T) {
	header := EnvelopeHeader{
		Timestamp:   "2024-01-15 10:30",
		Elapsed:     "5m ago",
		SenderLabel: "Alice",
		ChannelType: "group",
	}
	got := FormatEnvelopeHeader(header)
	if !strings.Contains(got, "5m ago") {
		t.Errorf("expected elapsed in output, got %q", got)
	}
	if !strings.Contains(got, "Alice") {
		t.Errorf("expected sender in output, got %q", got)
	}
}
