package autoreply

import "testing"

func TestNormalizeThinkLevel(t *testing.T) {
	tests := []struct {
		input string
		want  ThinkLevel
		ok    bool
	}{
		{"off", ThinkOff, true},
		{"on", ThinkLow, true},
		{"minimal", ThinkMinimal, true},
		{"think", ThinkMinimal, true},
		{"low", ThinkLow, true},
		{"medium", ThinkMedium, true},
		{"high", ThinkHigh, true},
		{"xhigh", ThinkXHigh, true},
		{"extrahigh", ThinkXHigh, true},
		{"max", ThinkHigh, true},
		{"", "", false},
		{"invalid", "", false},
	}
	for _, tt := range tests {
		got, ok := NormalizeThinkLevel(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf("NormalizeThinkLevel(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestNormalizeVerboseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  VerboseLevel
		ok    bool
	}{
		{"off", VerboseOff, true},
		{"on", VerboseOn, true},
		{"full", VerboseFull, true},
		{"true", VerboseOn, true},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := NormalizeVerboseLevel(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf("NormalizeVerboseLevel(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestNormalizeElevatedLevel(t *testing.T) {
	tests := []struct {
		input string
		want  ElevatedLevel
		ok    bool
	}{
		{"off", ElevatedOff, true},
		{"on", ElevatedOn, true},
		{"ask", ElevatedAsk, true},
		{"full", ElevatedFull, true},
		{"auto-approve", ElevatedFull, true},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := NormalizeElevatedLevel(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf("NormalizeElevatedLevel(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestResolveElevatedMode(t *testing.T) {
	tests := []struct {
		input ElevatedLevel
		want  ElevatedMode
	}{
		{"", ElevatedModeOff},
		{ElevatedOff, ElevatedModeOff},
		{ElevatedFull, ElevatedModeFull},
		{ElevatedOn, ElevatedModeAsk},
		{ElevatedAsk, ElevatedModeAsk},
	}
	for _, tt := range tests {
		got := ResolveElevatedMode(tt.input)
		if got != tt.want {
			t.Errorf("ResolveElevatedMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSupportsXHighThinking(t *testing.T) {
	if !SupportsXHighThinking("openai", "gpt-5.2") {
		t.Error("should support xhigh for openai/gpt-5.2")
	}
	if SupportsXHighThinking("openai", "gpt-4") {
		t.Error("should not support xhigh for openai/gpt-4")
	}
	if !SupportsXHighThinking("", "gpt-5.2") {
		t.Error("should support xhigh by model ID alone")
	}
}

func TestListThinkingLevels(t *testing.T) {
	levels := ListThinkingLevels("", "gpt-4")
	if len(levels) != 5 {
		t.Errorf("expected 5 levels, got %d", len(levels))
	}
	levels = ListThinkingLevels("openai", "gpt-5.2")
	if len(levels) != 6 {
		t.Errorf("expected 6 levels (with xhigh), got %d", len(levels))
	}
}

func TestNormalizeReasoningLevel(t *testing.T) {
	tests := []struct {
		input string
		want  ReasoningLevel
		ok    bool
	}{
		{"off", ReasoningOff, true},
		{"on", ReasoningOn, true},
		{"stream", ReasoningStream, true},
		{"streaming", ReasoningStream, true},
		{"hide", ReasoningOff, true},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := NormalizeReasoningLevel(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf("NormalizeReasoningLevel(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestIsBinaryThinkingProvider(t *testing.T) {
	if !IsBinaryThinkingProvider("z.ai") {
		t.Error("z.ai should be binary")
	}
	if !IsBinaryThinkingProvider("z-ai") {
		t.Error("z-ai should be binary")
	}
	if IsBinaryThinkingProvider("openai") {
		t.Error("openai should not be binary")
	}
}
