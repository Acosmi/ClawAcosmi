package runner

import (
	"testing"
)

func TestStripBlockTags_ThinkTag(t *testing.T) {
	state := &TagStripState{}
	out := StripBlockTags("Hello <think>hidden</think> world", state)
	if out != "Hello  world" {
		t.Errorf("expected 'Hello  world', got %q", out)
	}
}

func TestStripBlockTags_UnclosedThink(t *testing.T) {
	state := &TagStripState{}
	out := StripBlockTags("Before <think>inside...", state)
	if out != "Before " {
		t.Errorf("expected 'Before ', got %q", out)
	}
	if !state.Thinking {
		t.Error("state.Thinking should be true")
	}
	// Continue with closing
	out2 := StripBlockTags("...still thinking</think>After", state)
	if out2 != "After" {
		t.Errorf("expected 'After', got %q", out2)
	}
}

func TestStripBlockTags_FinalTag(t *testing.T) {
	state := &TagStripState{}
	out := StripBlockTags("Hello <final>removed</final> kept", state)
	if out != "Hello  kept" {
		t.Errorf("expected 'Hello  kept', got %q", out)
	}
}

func TestStripBlockTags_NoTags(t *testing.T) {
	state := &TagStripState{}
	out := StripBlockTags("plain text no tags", state)
	if out != "plain text no tags" {
		t.Errorf("got %q", out)
	}
}

func TestSubscribeContext_MessageFlow(t *testing.T) {
	var partials []string
	var blockReplies []string

	ctx := NewSubscribeContext(SubscribeParams{
		RunID:          "test-run",
		BlockReplyMode: "text_end",
		OnPartial:      func(s string) { partials = append(partials, s) },
		OnBlockReply:   func(s string) { blockReplies = append(blockReplies, s) },
	})

	ctx.HandleEvent(SubscribeEvent{Type: EventMessageStart})
	ctx.HandleEvent(SubscribeEvent{Type: EventMessageUpdate, Text: "Hello "})
	ctx.HandleEvent(SubscribeEvent{Type: EventMessageUpdate, Text: "world"})
	ctx.HandleEvent(SubscribeEvent{Type: EventMessageEnd})

	if len(partials) != 2 {
		t.Fatalf("expected 2 partials, got %d", len(partials))
	}
	if partials[0] != "Hello " || partials[1] != "world" {
		t.Errorf("partials mismatch: %v", partials)
	}

	result := ctx.State.BuildResult()
	if len(result.AssistantTexts) != 1 || result.AssistantTexts[0] != "Hello world" {
		t.Errorf("assistant text mismatch: %v", result.AssistantTexts)
	}
}

func TestSubscribeContext_ToolTracking(t *testing.T) {
	var toolResults []string
	ctx := NewSubscribeContext(SubscribeParams{
		RunID:        "test-run",
		OnToolResult: func(s string) { toolResults = append(toolResults, s) },
	})

	ctx.HandleEvent(SubscribeEvent{
		Type: EventToolExecStart, ToolName: "read", ToolID: "tool-1",
		Args: map[string]interface{}{"path": "/etc/hosts"},
	})
	ctx.HandleEvent(SubscribeEvent{
		Type: EventToolExecEnd, ToolName: "read", ToolID: "tool-1",
	})

	if len(toolResults) != 1 {
		t.Fatalf("expected 1 tool result, got %d", len(toolResults))
	}
	if toolResults[0] != "read (/etc/hosts)" {
		t.Errorf("tool result mismatch: %s", toolResults[0])
	}

	result := ctx.State.BuildResult()
	if len(result.ToolMetas) != 1 || result.ToolMetas[0].Meta != "/etc/hosts" {
		t.Errorf("tool meta mismatch: %+v", result.ToolMetas)
	}
}

func TestIsMessagingToolName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"discord_send", true},
		{"slack_send", true},
		{"sessions_send", true},
		{"messaging_custom", true},
		{"read", false},
		{"bash", false},
	}
	for _, tt := range tests {
		if got := isMessagingToolName(tt.name); got != tt.want {
			t.Errorf("isMessagingToolName(%q)=%v, want %v", tt.name, got, tt.want)
		}
	}
}
