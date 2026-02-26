package reply

import (
	"testing"
)

func TestResolveTypingMode(t *testing.T) {
	tests := []struct {
		name   string
		ctx    TypingModeContext
		expect TypingMode
	}{
		{"heartbeat -> never", TypingModeContext{IsHeartbeat: true}, TypingModeNever},
		{"configured instant", TypingModeContext{Configured: TypingModeInstant}, TypingModeInstant},
		{"configured never", TypingModeContext{Configured: TypingModeNever}, TypingModeNever},
		{"DM -> instant", TypingModeContext{}, TypingModeInstant},
		{"DM mentioned -> instant", TypingModeContext{WasMentioned: true}, TypingModeInstant},
		{"group not mentioned -> message", TypingModeContext{IsGroupChat: true}, TypingModeMessage},
		{"group mentioned -> instant", TypingModeContext{IsGroupChat: true, WasMentioned: true}, TypingModeInstant},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveTypingMode(tt.ctx)
			if result != tt.expect {
				t.Errorf("ResolveTypingMode = %q, want %q", result, tt.expect)
			}
		})
	}
}

func TestTypingSignalerFlags(t *testing.T) {
	tc := NewTypingController(TypingControllerParams{})
	defer tc.Cleanup()

	s := NewTypingSignaler(tc, TypingModeInstant, false)
	if !s.ShouldStartImmediately {
		t.Error("instant mode should start immediately")
	}
	if s.ShouldStartOnMessageStart {
		t.Error("instant mode should not start on message start")
	}

	s2 := NewTypingSignaler(tc, TypingModeMessage, false)
	if s2.ShouldStartImmediately {
		t.Error("message mode should not start immediately")
	}
	if !s2.ShouldStartOnMessageStart {
		t.Error("message mode should start on message start")
	}
	if !s2.ShouldStartOnText {
		t.Error("message mode should start on text")
	}

	s3 := NewTypingSignaler(tc, TypingModeThinking, false)
	if !s3.ShouldStartOnReasoning {
		t.Error("thinking mode should start on reasoning")
	}
}

func TestTypingSignalerDisabled(t *testing.T) {
	tc := NewTypingController(TypingControllerParams{})
	defer tc.Cleanup()

	// Never mode
	s := NewTypingSignaler(tc, TypingModeNever, false)
	if err := s.SignalRunStart(); err != nil {
		t.Errorf("SignalRunStart should not error: %v", err)
	}

	// Heartbeat
	s2 := NewTypingSignaler(tc, TypingModeInstant, true)
	if err := s2.SignalRunStart(); err != nil {
		t.Errorf("SignalRunStart should not error: %v", err)
	}
}

func TestFormatDirectiveAck(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"", ""},
		{"hello", "⚙️ hello"},
		{"⚙️ already", "⚙️ already"},
	}

	for _, tt := range tests {
		result := FormatDirectiveAck(tt.input)
		if result != tt.expect {
			t.Errorf("FormatDirectiveAck(%q) = %q, want %q", tt.input, result, tt.expect)
		}
	}
}
