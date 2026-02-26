package tui

import (
	"testing"
)

func TestResolveFinalAssistantText(t *testing.T) {
	tests := []struct {
		name      string
		finalText string
		streamed  string
		want      string
	}{
		{"both empty", "", "", "(no output)"},
		{"whitespace only", "   ", "   ", "(no output)"},
		{"finalText present", "hello", "streamed", "hello"},
		{"finalText empty, streamed present", "", "streamed", "streamed"},
		{"finalText whitespace, streamed present", "   ", "world", "world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveFinalAssistantText(tt.finalText, tt.streamed)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestComposeThinkingAndContent(t *testing.T) {
	tests := []struct {
		name         string
		thinking     string
		content      string
		showThinking bool
		want         string
	}{
		{"both empty", "", "", false, ""},
		{"content only", "", "hello", false, "hello"},
		{"thinking hidden", "deep thought", "hello", false, "hello"},
		{"thinking shown", "deep thought", "hello", true, "[thinking]\ndeep thought\n\nhello"},
		{"thinking only shown", "deep thought", "", true, "[thinking]\ndeep thought"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComposeThinkingAndContent(tt.thinking, tt.content, tt.showThinking)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractThinkingFromMessage(t *testing.T) {
	tests := []struct {
		name    string
		message interface{}
		want    string
	}{
		{"nil", nil, ""},
		{"non-object", "string", ""},
		{"string content", map[string]interface{}{"content": "hello"}, ""},
		{"no thinking blocks", map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "hello"},
			},
		}, ""},
		{"one thinking block", map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "thinking", "thinking": "deep thought"},
			},
		}, "deep thought"},
		{"multiple thinking blocks", map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "thinking", "thinking": "thought 1"},
				map[string]interface{}{"type": "text", "text": "hello"},
				map[string]interface{}{"type": "thinking", "thinking": "thought 2"},
			},
		}, "thought 1\nthought 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractThinkingFromMessage(tt.message)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractContentFromMessage(t *testing.T) {
	tests := []struct {
		name    string
		message interface{}
		want    string
	}{
		{"nil", nil, ""},
		{"string content", map[string]interface{}{"content": "hello"}, "hello"},
		{"text blocks", map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "hello"},
				map[string]interface{}{"type": "text", "text": "world"},
			},
		}, "hello\nworld"},
		{"error fallback no content", map[string]interface{}{
			"stopReason":   "error",
			"errorMessage": "rate limit exceeded",
		}, "rate limit exceeded"},
		{"error fallback empty text blocks", map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "thinking", "thinking": "hmm"},
			},
			"stopReason":   "error",
			"errorMessage": "something went wrong",
		}, "something went wrong"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractContentFromMessage(tt.message)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractTextFromMessage(t *testing.T) {
	tests := []struct {
		name            string
		message         interface{}
		includeThinking bool
		want            string
	}{
		{"nil", nil, false, ""},
		{"text only", map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "hello"},
			},
		}, false, "hello"},
		{"with thinking excluded", map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "thinking", "thinking": "hmm"},
				map[string]interface{}{"type": "text", "text": "hello"},
			},
		}, false, "hello"},
		{"with thinking included", map[string]interface{}{
			"content": []interface{}{
				map[string]interface{}{"type": "thinking", "thinking": "hmm"},
				map[string]interface{}{"type": "text", "text": "hello"},
			},
		}, true, "[thinking]\nhmm\n\nhello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractTextFromMessage(tt.message, tt.includeThinking)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsCommandMessage(t *testing.T) {
	tests := []struct {
		name    string
		message interface{}
		want    bool
	}{
		{"nil", nil, false},
		{"no command field", map[string]interface{}{"text": "hi"}, false},
		{"command true", map[string]interface{}{"command": true}, true},
		{"command false", map[string]interface{}{"command": false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCommandMessage(tt.message)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	intPtr := func(v int) *int { return &v }
	tests := []struct {
		name    string
		total   *int
		context *int
		want    string
	}{
		{"both nil", nil, nil, "tokens ?"},
		{"total only", intPtr(1500), nil, "tokens 1.5k"},
		{"both present", intPtr(1500), intPtr(10000), "tokens 1.5k/10k (15%)"},
		{"zero context", intPtr(100), intPtr(0), "tokens 100/0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTokens(tt.total, tt.context)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatContextUsageLine(t *testing.T) {
	intPtr := func(v int) *int { return &v }
	tests := []struct {
		name      string
		total     *int
		context   *int
		remaining *int
		percent   *int
		wantEmpty bool
	}{
		{"all nil", nil, nil, nil, nil, false},
		{"total only", intPtr(1500), nil, nil, nil, false},
		{"total and context", intPtr(1500), intPtr(10000), nil, nil, false},
		{"all fields", intPtr(1500), intPtr(10000), intPtr(8500), intPtr(15), false},
		{"zero values", intPtr(0), intPtr(0), intPtr(0), intPtr(0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatContextUsageLine(tt.total, tt.context, tt.remaining, tt.percent)
			if tt.wantEmpty && got != "" {
				t.Errorf("got %q, want empty", got)
			}
			if !tt.wantEmpty && got == "" {
				t.Error("got empty, want non-empty")
			}
		})
	}
}

func TestAsString(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		fallback string
		want     string
	}{
		{"string", "hello", "", "hello"},
		{"int", 42, "", "42"},
		{"bool true", true, "", "true"},
		{"bool false", false, "", "false"},
		{"nil", nil, "default", "default"},
		{"slice", []int{1}, "fallback", "fallback"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AsString(tt.value, tt.fallback)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
