package autoreply

import "testing"

func TestApplyTemplate(t *testing.T) {
	ctx := &TemplateContext{
		Model:    "gpt-4",
		Provider: "openai",
		Date:     "2024-01-15",
	}
	result := ApplyTemplate("Using {{model}} via {{provider}} on {{date}}", ctx)
	expected := "Using gpt-4 via openai on 2024-01-15"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestApplyTemplate_NoPlaceholders(t *testing.T) {
	result := ApplyTemplate("no placeholders here", &TemplateContext{})
	if result != "no placeholders here" {
		t.Errorf("got %q", result)
	}
}

func TestApplyTemplate_Empty(t *testing.T) {
	result := ApplyTemplate("", nil)
	if result != "" {
		t.Errorf("got %q", result)
	}
}

func TestFormatMediaAttachedLine(t *testing.T) {
	tests := []struct {
		mediaType string
		count     int
		want      string
	}{
		{"image", 1, "[image attached]"},
		{"audio", 3, "[3 audios attached]"},
		{"", 1, "[file attached]"},
		{"video", 0, ""},
	}
	for _, tt := range tests {
		got := FormatMediaAttachedLine(tt.mediaType, tt.count)
		if got != tt.want {
			t.Errorf("FormatMediaAttachedLine(%q, %d) = %q, want %q", tt.mediaType, tt.count, got, tt.want)
		}
	}
}

func TestBuildInboundMediaNote_NoMedia(t *testing.T) {
	ctx := &MsgContext{}
	result := BuildInboundMediaNote(ctx)
	if result != "" {
		t.Errorf("should be empty, got %q", result)
	}
}

func TestBuildInboundMediaNote_WithMedia(t *testing.T) {
	ctx := &MsgContext{HasAttachments: true, MediaType: "image", MediaCount: 2}
	result := BuildInboundMediaNote(ctx)
	if result != "[2 images attached]" {
		t.Errorf("got %q, want %q", result, "[2 images attached]")
	}
}

func TestHasControlCommand(t *testing.T) {
	if !HasControlCommand("/status") {
		t.Error("/status should be a control command")
	}
	if HasControlCommand("hello") {
		t.Error("hello should not be a control command")
	}
}

func TestShouldComputeCommandAuthorized(t *testing.T) {
	if !ShouldComputeCommandAuthorized("/model gpt-4") {
		t.Error("should compute for /model")
	}
	if !ShouldComputeCommandAuthorized("use [[thinking:high]]") {
		t.Error("should compute for [[ tokens")
	}
	if ShouldComputeCommandAuthorized("hello") {
		t.Error("should not compute for plain text")
	}
}

func TestResolveCommandAuthorization_BotOwner(t *testing.T) {
	auth := ResolveCommandAuthorization(&CommandAuthParams{IsBotOwner: true})
	if !auth.Authorized {
		t.Error("bot owner should be authorized")
	}
}

func TestResolveCommandAuthorization_DenyList(t *testing.T) {
	auth := ResolveCommandAuthorization(&CommandAuthParams{
		SenderID:  "user1",
		DenyUsers: []string{"user1"},
	})
	if auth.Authorized {
		t.Error("denied user should not be authorized")
	}
}

func TestResolveCommandAuthorization_AllowList(t *testing.T) {
	auth := ResolveCommandAuthorization(&CommandAuthParams{
		SenderID:     "user2",
		AllowedUsers: []string{"user1"},
	})
	if auth.Authorized {
		t.Error("user not in allow list should not be authorized")
	}
}

func TestResolveInboundDebounceMs(t *testing.T) {
	if got := ResolveInboundDebounceMs(0, 0); got != DefaultDebounceMs {
		t.Errorf("got %d, want %d", got, DefaultDebounceMs)
	}
	if got := ResolveInboundDebounceMs(500, 0); got != 500 {
		t.Errorf("got %d, want 500", got)
	}
}
