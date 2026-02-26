package reply

import (
	"strings"
	"testing"
)

func TestMaybeHandleQueueDirective_NoDirective(t *testing.T) {
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{HasQueueDirective: false},
	})
	if result != nil {
		t.Errorf("expected nil for non-queue directive, got %+v", result)
	}
}

func TestMaybeHandleQueueDirective_StatusDisplay(t *testing.T) {
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{HasQueueDirective: true},
		Channel:    "telegram",
	})
	if result == nil {
		t.Fatal("expected status reply, got nil")
	}
	if !strings.Contains(result.Text, "Current queue settings") {
		t.Errorf("expected status text, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "mode=") {
		t.Errorf("expected mode in status, got %q", result.Text)
	}
}

func TestMaybeHandleQueueDirective_ValidMode(t *testing.T) {
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{
			HasQueueDirective: true,
			QueueMode:         QueueModeSteer,
			RawQueueMode:      "steer",
		},
	})
	if result != nil {
		t.Errorf("expected nil for valid mode, got %+v", result)
	}
}

func TestMaybeHandleQueueDirective_InvalidMode(t *testing.T) {
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{
			HasQueueDirective: true,
			RawQueueMode:      "badmode",
		},
	})
	if result == nil {
		t.Fatal("expected error reply, got nil")
	}
	if !strings.Contains(result.Text, "Unrecognized queue mode") {
		t.Errorf("expected mode error, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "badmode") {
		t.Errorf("expected raw value in error, got %q", result.Text)
	}
}

func TestMaybeHandleQueueDirective_InvalidDebounce(t *testing.T) {
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{
			HasQueueDirective: true,
			QueueMode:         QueueModeCollect,
			RawQueueMode:      "collect",
			RawDebounce:       "abc",
		},
	})
	if result == nil {
		t.Fatal("expected error reply, got nil")
	}
	if !strings.Contains(result.Text, "Invalid debounce") {
		t.Errorf("expected debounce error, got %q", result.Text)
	}
}

func TestMaybeHandleQueueDirective_InvalidCap(t *testing.T) {
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{
			HasQueueDirective: true,
			QueueMode:         QueueModeCollect,
			RawQueueMode:      "collect",
			RawCap:            "notanumber",
		},
	})
	if result == nil {
		t.Fatal("expected error reply, got nil")
	}
	if !strings.Contains(result.Text, "Invalid cap") {
		t.Errorf("expected cap error, got %q", result.Text)
	}
}

func TestMaybeHandleQueueDirective_InvalidDrop(t *testing.T) {
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{
			HasQueueDirective: true,
			QueueMode:         QueueModeCollect,
			RawQueueMode:      "collect",
			RawDrop:           "invalid",
		},
	})
	if result == nil {
		t.Fatal("expected error reply, got nil")
	}
	if !strings.Contains(result.Text, "Invalid drop policy") {
		t.Errorf("expected drop error, got %q", result.Text)
	}
}

func TestMaybeHandleQueueDirective_MultipleErrors(t *testing.T) {
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{
			HasQueueDirective: true,
			RawQueueMode:      "badmode",
			RawDebounce:       "xyz",
			RawCap:            "abc",
			RawDrop:           "nope",
		},
	})
	if result == nil {
		t.Fatal("expected error reply, got nil")
	}
	if !strings.Contains(result.Text, "Unrecognized queue mode") {
		t.Errorf("expected mode error in combined text, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "Invalid debounce") {
		t.Errorf("expected debounce error in combined text, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "Invalid cap") {
		t.Errorf("expected cap error in combined text, got %q", result.Text)
	}
	if !strings.Contains(result.Text, "Invalid drop") {
		t.Errorf("expected drop error in combined text, got %q", result.Text)
	}
}

func TestMaybeHandleQueueDirective_QueueReset(t *testing.T) {
	// queue reset 不触发 status，也不触发 validation error → 由持久化层处理
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{
			HasQueueDirective: true,
			QueueReset:        true,
		},
	})
	if result != nil {
		t.Errorf("expected nil for queue reset, got %+v", result)
	}
}

func TestMaybeHandleQueueDirective_ValidDebounce(t *testing.T) {
	debounce := 1500
	result := MaybeHandleQueueDirective(MaybeHandleQueueDirectiveParams{
		Directives: InlineDirectives{
			HasQueueDirective: true,
			QueueMode:         QueueModeCollect,
			RawQueueMode:      "collect",
			DebounceMs:        &debounce,
			RawDebounce:       "1500ms",
		},
	})
	if result != nil {
		t.Errorf("expected nil for valid debounce, got %+v", result)
	}
}
