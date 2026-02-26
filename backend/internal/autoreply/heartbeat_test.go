package autoreply

import "testing"

func TestIsHeartbeatContentEffectivelyEmpty_EmptyString(t *testing.T) {
	if !IsHeartbeatContentEffectivelyEmpty("") {
		t.Error("empty string should be effectively empty")
	}
}

func TestIsHeartbeatContentEffectivelyEmpty_OnlyHeaders(t *testing.T) {
	content := "# Title\n## Subtitle\n"
	if !IsHeartbeatContentEffectivelyEmpty(content) {
		t.Error("only headers should be effectively empty")
	}
}

func TestIsHeartbeatContentEffectivelyEmpty_WithContent(t *testing.T) {
	content := "# Title\nDo something important"
	if IsHeartbeatContentEffectivelyEmpty(content) {
		t.Error("content after header should not be empty")
	}
}

func TestIsHeartbeatContentEffectivelyEmpty_EmptyCheckbox(t *testing.T) {
	content := "# Tasks\n- [ ]\n* [ ]\n"
	if !IsHeartbeatContentEffectivelyEmpty(content) {
		t.Error("empty checkboxes should be effectively empty")
	}
}

func TestResolveHeartbeatPrompt_Empty(t *testing.T) {
	result := ResolveHeartbeatPrompt("")
	if result != HeartbeatPrompt {
		t.Errorf("empty should return default prompt")
	}
}

func TestResolveHeartbeatPrompt_Custom(t *testing.T) {
	result := ResolveHeartbeatPrompt("custom prompt")
	if result != "custom prompt" {
		t.Errorf("should return custom prompt, got %q", result)
	}
}

func TestStripHeartbeatToken_ExactToken(t *testing.T) {
	result := StripHeartbeatToken("HEARTBEAT_OK", nil)
	if !result.ShouldSkip {
		t.Error("exact token should skip")
	}
	if !result.DidStrip {
		t.Error("should have stripped")
	}
}

func TestStripHeartbeatToken_TokenWithSuffix(t *testing.T) {
	result := StripHeartbeatToken("HEARTBEAT_OK Something extra here that is long enough to exceed the ack limit for sure when we are testing", &StripHeartbeatOpts{Mode: StripModeMessage})
	if result.ShouldSkip {
		t.Error("message mode with long suffix should not skip")
	}
	if !result.DidStrip {
		t.Error("should have stripped")
	}
	if result.Text == "" {
		t.Error("text should have remaining content")
	}
}

func TestStripHeartbeatToken_HeartbeatModeShortAck(t *testing.T) {
	result := StripHeartbeatToken("HEARTBEAT_OK All good", &StripHeartbeatOpts{Mode: StripModeHeartbeat})
	if !result.ShouldSkip {
		t.Error("heartbeat mode with short ack should skip")
	}
}

func TestStripHeartbeatToken_NoToken(t *testing.T) {
	result := StripHeartbeatToken("hello world", nil)
	if result.ShouldSkip {
		t.Error("no token should not skip")
	}
	if result.DidStrip {
		t.Error("should not have stripped")
	}
}

func TestStripHeartbeatToken_Empty(t *testing.T) {
	result := StripHeartbeatToken("", nil)
	if !result.ShouldSkip {
		t.Error("empty should skip")
	}
}

func TestStripHeartbeatToken_MarkdownWrapped(t *testing.T) {
	result := StripHeartbeatToken("**HEARTBEAT_OK**", nil)
	if !result.ShouldSkip {
		t.Error("markdown-wrapped token should skip")
	}
	if !result.DidStrip {
		t.Error("should have stripped")
	}
}
