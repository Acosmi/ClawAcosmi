package reply

import (
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
)

func TestIsBunFetchSocketError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"", false},
		{"some normal error", false},
		{"socket connection was closed unexpectedly", true},
		{"Error: Socket connection was closed unexpectedly by the server", true},
	}
	for _, tt := range tests {
		if got := IsBunFetchSocketError(tt.msg); got != tt.want {
			t.Errorf("IsBunFetchSocketError(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestFormatBunFetchSocketError(t *testing.T) {
	result := FormatBunFetchSocketError("socket connection was closed unexpectedly")
	if result == "" {
		t.Error("expected non-empty result")
	}
	if !contains(result, "⚠️") {
		t.Error("expected warning emoji")
	}
}

func TestFormatResponseUsageLine(t *testing.T) {
	in, out := 1500, 300
	line := FormatResponseUsageLine(UsageLineParams{
		Usage: &NormalizedUsage{Input: &in, Output: &out},
	})
	if line == "" {
		t.Error("expected non-empty line")
	}
	if !contains(line, "1.5k in") {
		t.Errorf("expected '1.5k in', got %q", line)
	}
	if !contains(line, "300 out") {
		t.Errorf("expected '300 out', got %q", line)
	}
}

func TestFormatResponseUsageLineNil(t *testing.T) {
	line := FormatResponseUsageLine(UsageLineParams{})
	if line != "" {
		t.Errorf("expected empty for nil usage, got %q", line)
	}
}

func TestFormatResponseUsageLineWithCost(t *testing.T) {
	in, out := 1000, 500
	line := FormatResponseUsageLine(UsageLineParams{
		Usage:    &NormalizedUsage{Input: &in, Output: &out},
		ShowCost: true,
		CostConfig: &UsageCostConfig{
			Input:  3.0,
			Output: 15.0,
		},
	})
	if !contains(line, "est $") {
		t.Errorf("expected cost label, got %q", line)
	}
}

func TestAppendUsageLine(t *testing.T) {
	payloads := []autoreply.ReplyPayload{
		{Text: "Hello"},
		{MediaURL: "img.png"},
	}
	result := AppendUsageLine(payloads, "Usage: 1k in / 500 out")
	if len(result) != 2 {
		t.Fatalf("expected 2 payloads, got %d", len(result))
	}
	if !contains(result[0].Text, "Usage:") {
		t.Errorf("expected usage appended to first, got %q", result[0].Text)
	}
}

func TestAppendUsageLineEmpty(t *testing.T) {
	result := AppendUsageLine(nil, "line")
	if len(result) != 1 {
		t.Fatalf("expected 1, got %d", len(result))
	}
	if result[0].Text != "line" {
		t.Errorf("got %q", result[0].Text)
	}
}

func TestIsAudioPayload(t *testing.T) {
	if !IsAudioPayload(autoreply.ReplyPayload{MediaURL: "voice.mp3"}) {
		t.Error("expected true for mp3")
	}
	if IsAudioPayload(autoreply.ReplyPayload{MediaURL: "image.png"}) {
		t.Error("expected false for png")
	}
	if !IsAudioPayload(autoreply.ReplyPayload{MediaURLs: []string{"a.wav"}}) {
		t.Error("expected true for wav in MediaURLs")
	}
}

func TestResolveEnforceFinalTag(t *testing.T) {
	if !ResolveEnforceFinalTag(false, "deepseek") {
		t.Error("deepseek should enforce final tag")
	}
	if !ResolveEnforceFinalTag(true, "anthropic") {
		t.Error("explicit true should enforce")
	}
	if ResolveEnforceFinalTag(false, "openai") {
		t.Error("openai without explicit should not enforce")
	}
}

func TestShouldEmitToolResult(t *testing.T) {
	if ShouldEmitToolResult(autoreply.VerboseOff) {
		t.Error("off should not emit")
	}
	if !ShouldEmitToolResult(autoreply.VerboseOn) {
		t.Error("on should emit")
	}
}

func TestShouldEmitToolOutput(t *testing.T) {
	if ShouldEmitToolOutput(autoreply.VerboseOn) {
		t.Error("on should not emit output")
	}
	if !ShouldEmitToolOutput(autoreply.VerboseFull) {
		t.Error("full should emit output")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
