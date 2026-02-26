package autoreply

import (
	"strings"
	"testing"
)

func TestFormatTokenCount(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{0, "0"},
		{500, "500"},
		{999, "999"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{9999, "10.0k"},
		{10000, "10k"},
		{128000, "128k"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
		{-1, "0"},
	}
	for _, tt := range tests {
		got := FormatTokenCount(tt.count)
		if got != tt.want {
			t.Errorf("FormatTokenCount(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestFormatContextUsageShort(t *testing.T) {
	tests := []struct {
		used, max int
		want      string
	}{
		{12000, 128000, "Context 12k/128k (9%)"},
		{0, 128000, "Context 0/128k (0%)"},
		{128000, 128000, "Context 128k/128k (100%)"},
		{5000, 0, "Context 5.0k"},
	}
	for _, tt := range tests {
		got := FormatContextUsageShort(tt.used, tt.max)
		if got != tt.want {
			t.Errorf("FormatContextUsageShort(%d, %d) = %q, want %q",
				tt.used, tt.max, got, tt.want)
		}
	}
}

func TestFormatQueueDetails(t *testing.T) {
	got := FormatQueueDetails(nil)
	if got != "" {
		t.Errorf("nil queue should return empty, got %q", got)
	}

	q := &QueueStatus{Mode: "serial", Depth: 3, Cap: 10, DebounceMs: 500}
	got = FormatQueueDetails(q)
	if !strings.Contains(got, "mode: serial") {
		t.Errorf("expected 'mode: serial' in %q", got)
	}
	if !strings.Contains(got, "depth: 3") {
		t.Errorf("expected 'depth: 3' in %q", got)
	}
}

func TestFormatMediaDecisionsLine(t *testing.T) {
	got := FormatMediaDecisionsLine(nil)
	if got != "" {
		t.Errorf("nil decisions should return empty, got %q", got)
	}

	decisions := []MediaDecision{
		{Kind: "image", Provider: "openai", Accepted: true},
		{Kind: "audio", Accepted: false},
	}
	got = FormatMediaDecisionsLine(decisions)
	if !strings.Contains(got, "✅ image") {
		t.Errorf("expected accepted image marker in %q", got)
	}
	if !strings.Contains(got, "❌ audio") {
		t.Errorf("expected rejected audio marker in %q", got)
	}
}

func TestFormatUsagePair(t *testing.T) {
	got := FormatUsagePair(5000, 3000)
	if got != "↑5.0k ↓3.0k" {
		t.Errorf("FormatUsagePair(5000,3000) = %q, want %q", got, "↑5.0k ↓3.0k")
	}
}

func TestBuildStatusMessage_Minimal(t *testing.T) {
	args := &StatusArgs{AgentLabel: "TestBot"}
	msg := BuildStatusMessage(args, nil)
	if !strings.Contains(msg, "Status") {
		t.Errorf("missing Status header in %q", msg)
	}
	if !strings.Contains(msg, "TestBot") {
		t.Errorf("missing agent label in %q", msg)
	}
}

func TestBuildHelpMessage(t *testing.T) {
	msg := BuildHelpMessage(nil)
	if !strings.Contains(msg, "Help") {
		t.Errorf("missing Help header in %q", msg)
	}
	if !strings.Contains(msg, "/commands") {
		t.Errorf("missing /commands reference in %q", msg)
	}
}

func TestBuildCommandList(t *testing.T) {
	entries := BuildCommandList()
	// 至少应有注册的命令
	if len(entries) == 0 {
		t.Log("no commands registered; skipping detailed check")
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name, "/") {
			t.Errorf("command name %q should start with /", e.Name)
		}
	}
}
