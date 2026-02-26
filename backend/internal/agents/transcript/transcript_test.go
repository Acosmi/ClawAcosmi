package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeToolCallId(t *testing.T) {
	// Empty → default
	if got := SanitizeToolCallId("", ToolIdStrict); got != "defaulttoolid" {
		t.Errorf("empty strict = %q", got)
	}
	if got := SanitizeToolCallId("", ToolIdStrict9); got != "defaultid" {
		t.Errorf("empty strict9 = %q", got)
	}

	// Normal
	if got := SanitizeToolCallId("abc-123", ToolIdStrict); got != "abc123" {
		t.Errorf("abc-123 strict = %q, want abc123", got)
	}

	// Strict9 truncation
	got := SanitizeToolCallId("abcdefghijklmnop", ToolIdStrict9)
	if len(got) != 9 {
		t.Errorf("strict9 len = %d, want 9", len(got))
	}
}

func TestIsValidToolId(t *testing.T) {
	if IsValidToolId("", ToolIdStrict) {
		t.Error("empty should be invalid")
	}
	if !IsValidToolId("abc123", ToolIdStrict) {
		t.Error("abc123 should be valid")
	}
	if IsValidToolId("abc-123", ToolIdStrict) {
		t.Error("abc-123 should be invalid in strict")
	}
	if !IsValidToolId("abcdefghi", ToolIdStrict9) {
		t.Error("abcdefghi should be valid in strict9")
	}
}

func TestMakeUniqueToolId(t *testing.T) {
	used := make(map[string]bool)
	id1 := MakeUniqueToolId("test-id", used, ToolIdStrict)
	used[id1] = true
	id2 := MakeUniqueToolId("test-id", used, ToolIdStrict)
	if id1 == id2 {
		t.Errorf("should generate unique IDs: %q == %q", id1, id2)
	}
}

func TestResolveTranscriptPolicy(t *testing.T) {
	// Google
	p := ResolveTranscriptPolicy("google-genai", "google", "gemini-pro")
	if p.SanitizeMode != SanitizeFull {
		t.Errorf("google sanitize = %q, want full", p.SanitizeMode)
	}
	if !p.ApplyGoogleTurnOrdering {
		t.Error("google should apply turn ordering")
	}

	// OpenAI
	p = ResolveTranscriptPolicy("openai", "openai", "gpt-4")
	if p.SanitizeMode != SanitizeImagesOnly {
		t.Errorf("openai sanitize = %q, want images-only", p.SanitizeMode)
	}
	if p.SanitizeToolCallIds {
		t.Error("openai should not sanitize tool call IDs")
	}

	// Anthropic
	p = ResolveTranscriptPolicy("anthropic-messages", "anthropic", "claude-3")
	if !p.ValidateAnthropicTurns {
		t.Error("anthropic should validate turns")
	}

	// Mistral
	p = ResolveTranscriptPolicy("", "mistral", "mistral-large")
	if p.ToolCallIdMode != ToolIdStrict9 {
		t.Errorf("mistral tool id mode = %q, want strict9", p.ToolCallIdMode)
	}
}

func TestRepairSessionFileIfNeeded_MissingFile(t *testing.T) {
	report := RepairSessionFileIfNeeded("/nonexistent/file.json", nil)
	if report.Repaired {
		t.Error("missing file should not be repaired")
	}
	if report.Reason != "missing session file" {
		t.Errorf("reason = %q", report.Reason)
	}
}

func TestRepairSessionFileIfNeeded_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.ndjson")

	header := map[string]interface{}{"type": "session", "id": "test-123"}
	msg := map[string]interface{}{"type": "message", "content": "hello"}

	h, _ := json.Marshal(header)
	m, _ := json.Marshal(msg)
	os.WriteFile(path, []byte(string(h)+"\n"+string(m)+"\n"), 0644)

	report := RepairSessionFileIfNeeded(path, nil)
	if report.Repaired {
		t.Error("valid file should not need repair")
	}
	if report.DroppedLines != 0 {
		t.Errorf("dropped = %d", report.DroppedLines)
	}
}

func TestRepairSessionFileIfNeeded_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.ndjson")

	header := map[string]interface{}{"type": "session", "id": "test-123"}
	h, _ := json.Marshal(header)
	content := string(h) + "\nbadline\n{\"type\":\"message\"}\n"
	os.WriteFile(path, []byte(content), 0644)

	var warnings []string
	warn := func(msg string) { warnings = append(warnings, msg) }

	report := RepairSessionFileIfNeeded(path, warn)
	if !report.Repaired {
		t.Error("corrupted file should be repaired")
	}
	if report.DroppedLines != 1 {
		t.Errorf("dropped = %d, want 1", report.DroppedLines)
	}
	if report.BackupPath == "" {
		t.Error("should have backup path")
	}
}
