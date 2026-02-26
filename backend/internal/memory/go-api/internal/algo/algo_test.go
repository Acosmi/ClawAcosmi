package algo

import (
	"testing"
)

func TestExtractCategoryFromJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"valid category", `{"category": "preference"}`, "preference"},
		{"with markdown fences", "```json\n{\"category\": \"habit\"}\n```", "habit"},
		{"uppercase", `{"category": "GOAL"}`, "goal"},
		{"empty", `{"category": ""}`, "fact"},
		{"invalid category", `{"category": "unknown_type"}`, "fact"},
		{"invalid JSON", `not json at all`, "fact"},
		{"missing field", `{"other": "value"}`, "fact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCategoryFromJSON(tt.input)
			if result != tt.expected {
				t.Errorf("extractCategoryFromJSON(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractCoreMemoryEdits(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNil   bool
		wantCount int
	}{
		{
			"with edits",
			`{"reflection": "User likes tea.", "core_memory_edits": [{"section": "preferences", "content": "prefers tea", "mode": "append"}]}`,
			false,
			1,
		},
		{
			"no edits",
			`{"reflection": "User likes tea.", "core_memory_edits": []}`,
			true,
			0,
		},
		{
			"plain text",
			"This is just a reflection without JSON.",
			true,
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCoreMemoryEdits(tt.input)
			if tt.wantNil && result != nil {
				t.Errorf("expected nil, got %v", result)
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result")
			}
			if !tt.wantNil && len(result) != tt.wantCount {
				t.Errorf("expected %d edits, got %d", tt.wantCount, len(result))
			}
		})
	}
}

func TestExtractReflectionText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"JSON with reflection field",
			`{"reflection": "User enjoys coding in Go."}`,
			"User enjoys coding in Go.",
		},
		{
			"plain text",
			"User enjoys coding in Go.",
			"User enjoys coding in Go.",
		},
		{
			"JSON with empty reflection",
			`{"reflection": ""}`,
			`{"reflection": ""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractReflectionText(tt.input)
			if result != tt.expected {
				t.Errorf("extractReflectionText(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHealthNoServices(t *testing.T) {
	svc := NewService(nil, nil, nil)
	health := svc.Health()

	if health.Status != "degraded" {
		t.Errorf("expected 'degraded', got %q", health.Status)
	}
	if health.Embedding {
		t.Error("expected Embedding=false when no service")
	}
	if health.LLM {
		t.Error("expected LLM=false when no service")
	}
	if health.Rerank {
		t.Error("expected Rerank=false when no service")
	}
}
