package sessions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSessionTranscriptPath(t *testing.T) {
	tests := []struct {
		name       string
		sessionID  string
		agentID    string
		topicID    interface{}
		wantSuffix string
	}{
		{"basic", "abc123", "", nil, filepath.Join("sessions", "abc123.jsonl")},
		{"with string topic", "abc123", "", "topic1", filepath.Join("sessions", "abc123-topic-topic1.jsonl")},
		{"with int topic", "abc123", "", 42, filepath.Join("sessions", "abc123-topic-42.jsonl")},
		{"custom agent", "abc123", "bot2", nil, filepath.Join("sessions", "abc123.jsonl")},
		{"empty topic string", "abc123", "", "", filepath.Join("sessions", "abc123.jsonl")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSessionTranscriptPath(tt.sessionID, tt.agentID, tt.topicID)
			if !pathEndsWith(got, tt.wantSuffix) {
				t.Errorf("ResolveSessionTranscriptPath() = %q, want suffix %q", got, tt.wantSuffix)
			}
		})
	}
}

func TestResolveSessionFilePath(t *testing.T) {
	t.Run("nil entry", func(t *testing.T) {
		got := ResolveSessionFilePath("sess1", nil, "")
		if got == "" {
			t.Error("expected non-empty path")
		}
	})
	t.Run("entry with sessionFile", func(t *testing.T) {
		entry := &FullSessionEntry{SessionFile: "/tmp/custom.jsonl"}
		got := ResolveSessionFilePath("sess1", entry, "")
		if got != "/tmp/custom.jsonl" {
			t.Errorf("expected /tmp/custom.jsonl, got %q", got)
		}
	})
	t.Run("entry without sessionFile", func(t *testing.T) {
		entry := &FullSessionEntry{}
		got := ResolveSessionFilePath("sess1", entry, "")
		if got == "" {
			t.Error("expected non-empty fallback path")
		}
	})
}

func TestResolveStorePath(t *testing.T) {
	t.Run("empty store", func(t *testing.T) {
		got := ResolveStorePath("", "")
		if !pathEndsWith(got, "sessions.json") {
			t.Errorf("expected sessions.json suffix, got %q", got)
		}
	})
	t.Run("with agentId template", func(t *testing.T) {
		got := ResolveStorePath("/tmp/{agentId}/store.json", "mybot")
		if got != "/tmp/mybot/store.json" {
			t.Errorf("expected /tmp/mybot/store.json, got %q", got)
		}
	})
	t.Run("tilde expansion", func(t *testing.T) {
		got := ResolveStorePath("~/test/store.json", "")
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, "test", "store.json")
		if got != expected {
			t.Errorf("expected %q, got %q", expected, got)
		}
	})
}

func pathEndsWith(path, suffix string) bool {
	// Normalize both to forward slashes for comparison
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}
