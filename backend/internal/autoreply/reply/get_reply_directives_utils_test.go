package reply

import "testing"

func TestClearInlineDirectives(t *testing.T) {
	d := ClearInlineDirectives("hello world")
	if d.Cleaned != "hello world" {
		t.Errorf("Cleaned = %q, want %q", d.Cleaned, "hello world")
	}
	if d.HasThinkDirective || d.HasVerboseDirective || d.HasElevatedDirective {
		t.Error("expected all directive flags to be false")
	}
	if d.HasModelDirective || d.HasQueueDirective || d.HasStatusDirective {
		t.Error("expected all directive flags to be false")
	}
}

func TestClearInlineDirectivesEmpty(t *testing.T) {
	d := ClearInlineDirectives("")
	if d.Cleaned != "" {
		t.Errorf("Cleaned = %q, want empty", d.Cleaned)
	}
}
