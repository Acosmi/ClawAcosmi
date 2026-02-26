package routing

import (
	"strings"
	"testing"
)

// ---------- ParseAgentSessionKey ----------

func TestParseAgentSessionKey_Valid(t *testing.T) {
	p := ParseAgentSessionKey("agent:mybot:main")
	if p == nil {
		t.Fatal("expected non-nil parsed key")
	}
	if p.AgentID != "mybot" {
		t.Errorf("agentID = %q, want 'mybot'", p.AgentID)
	}
	if p.Rest != "main" {
		t.Errorf("rest = %q, want 'main'", p.Rest)
	}
}

func TestParseAgentSessionKey_NoPrefix(t *testing.T) {
	p := ParseAgentSessionKey("legacy-key-123")
	if p != nil {
		t.Errorf("expected nil for non-agent key, got %+v", p)
	}
}

func TestParseAgentSessionKey_MinParts(t *testing.T) {
	p := ParseAgentSessionKey("agent:")
	if p != nil {
		t.Errorf("expected nil for incomplete agent key, got %+v", p)
	}
}

// ---------- ClassifySessionKeyShape ----------

func TestClassifySessionKeyShape(t *testing.T) {
	tests := []struct {
		key  string
		want SessionKeyShape
	}{
		{"", SessionKeyMissing},
		{"agent:bot:main", SessionKeyAgent},
		{"legacy-key", SessionKeyLegacyOrAlias},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := ClassifySessionKeyShape(tt.key)
			if got != tt.want {
				t.Errorf("ClassifySessionKeyShape(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

// ---------- NormalizeAgentID ----------

func TestNormalizeAgentID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MyBot", "mybot"},
		{"my bot!", "my-bot"},
		{"", DefaultAgentID},
		{"  trimmed  ", "trimmed"},
		{"UPPER_case", "upper_case"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeAgentID(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeAgentID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- NormalizeMainKey ----------

func TestNormalizeMainKey(t *testing.T) {
	if got := NormalizeMainKey(""); got != DefaultMainKey {
		t.Errorf("expected default main key, got %q", got)
	}
	if got := NormalizeMainKey("  custom  "); got != "custom" {
		t.Errorf("expected 'custom', got %q", got)
	}
}

// ---------- BuildAgentMainSessionKey ----------

func TestBuildAgentMainSessionKey(t *testing.T) {
	key := BuildAgentMainSessionKey("mybot", "main")
	if !strings.HasPrefix(key, "agent:") {
		t.Errorf("expected 'agent:' prefix, got %q", key)
	}
	if !strings.Contains(key, "mybot") {
		t.Errorf("expected 'mybot' in key, got %q", key)
	}
}

// ---------- ToAgentRequestSessionKey / ToAgentStoreSessionKey ----------

func TestSessionKeyRoundTrip(t *testing.T) {
	store := BuildAgentMainSessionKey("mybot", "main")
	request := ToAgentRequestSessionKey(store)
	rebuilt := ToAgentStoreSessionKey("mybot", request, "main")
	if rebuilt != store {
		t.Errorf("round-trip failed: %q -> %q -> %q", store, request, rebuilt)
	}
}

// ---------- ResolveAgentIDFromSessionKey ----------

func TestResolveAgentIDFromSessionKey(t *testing.T) {
	id := ResolveAgentIDFromSessionKey("agent:mybot:main")
	if id != "mybot" {
		t.Errorf("expected 'mybot', got %q", id)
	}
	id = ResolveAgentIDFromSessionKey("legacy-key")
	if id != DefaultAgentID {
		t.Errorf("expected default for legacy key, got %q", id)
	}
}

// ---------- IsSubagentSessionKey ----------

func TestIsSubagentSessionKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"agent:main:main", false},
		{"agent:bot:subagent:task1", true},
		{"legacy-key", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := IsSubagentSessionKey(tt.key)
			if got != tt.want {
				t.Errorf("IsSubagentSessionKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

// ---------- BuildGroupHistoryKey ----------

func TestBuildGroupHistoryKey(t *testing.T) {
	key := BuildGroupHistoryKey("discord", "default", "guild", "12345")
	if key == "" {
		t.Error("expected non-empty group history key")
	}
	if !strings.Contains(key, "discord") {
		t.Errorf("expected 'discord' in key, got %q", key)
	}
}

// ---------- NormalizeAccountID ----------

func TestNormalizeAccountID(t *testing.T) {
	if got := NormalizeAccountID(""); got != DefaultAccountID {
		t.Errorf("expected default, got %q", got)
	}
	if got := NormalizeAccountID("MyAccount"); strings.Contains(got, "MyAccount") {
		t.Logf("normalized: %q", got)
	}
}
