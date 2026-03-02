package reply

import (
	"testing"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

func TestPersistInlineDirectives_Think(t *testing.T) {
	entry := &SessionEntry{SessionKey: "test"}
	PersistInlineDirectives(PersistDirectiveParams{
		Directives:   InlineDirectives{HasThinkDirective: true, ThinkLevel: autoreply.ThinkHigh},
		SessionEntry: entry,
		SessionKey:   "test",
	})
	if entry.ThinkingLevel != "high" {
		t.Errorf("ThinkingLevel = %q, want 'high'", entry.ThinkingLevel)
	}
	if entry.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set")
	}
}

func TestPersistInlineDirectives_ThinkOff(t *testing.T) {
	entry := &SessionEntry{SessionKey: "test", ThinkingLevel: "high"}
	PersistInlineDirectives(PersistDirectiveParams{
		Directives:   InlineDirectives{HasThinkDirective: true, ThinkLevel: autoreply.ThinkOff},
		SessionEntry: entry,
		SessionKey:   "test",
	})
	if entry.ThinkingLevel != "" {
		t.Errorf("ThinkingLevel = %q, want empty (cleared)", entry.ThinkingLevel)
	}
}

func TestPersistInlineDirectives_Verbose(t *testing.T) {
	entry := &SessionEntry{SessionKey: "test"}
	PersistInlineDirectives(PersistDirectiveParams{
		Directives:   InlineDirectives{HasVerboseDirective: true, VerboseLevel: autoreply.VerboseOn},
		SessionEntry: entry,
		SessionKey:   "test",
	})
	if entry.VerboseLevel != "on" {
		t.Errorf("VerboseLevel = %q, want 'on'", entry.VerboseLevel)
	}
}

func TestPersistInlineDirectives_Elevated(t *testing.T) {
	entry := &SessionEntry{SessionKey: "test"}
	PersistInlineDirectives(PersistDirectiveParams{
		Directives:      InlineDirectives{HasElevatedDirective: true, ElevatedLevel: autoreply.ElevatedFull},
		SessionEntry:    entry,
		SessionKey:      "test",
		ElevatedEnabled: true,
		ElevatedAllowed: true,
	})
	if entry.ElevatedLevel != "full" {
		t.Errorf("ElevatedLevel = %q, want 'full'", entry.ElevatedLevel)
	}
}

func TestPersistInlineDirectives_ElevatedNotAllowed(t *testing.T) {
	entry := &SessionEntry{SessionKey: "test"}
	PersistInlineDirectives(PersistDirectiveParams{
		Directives:      InlineDirectives{HasElevatedDirective: true, ElevatedLevel: autoreply.ElevatedFull},
		SessionEntry:    entry,
		SessionKey:      "test",
		ElevatedEnabled: true,
		ElevatedAllowed: false, // Not allowed
	})
	if entry.ElevatedLevel != "" {
		t.Errorf("ElevatedLevel should not be set when not allowed, got %q", entry.ElevatedLevel)
	}
}

func TestPersistInlineDirectives_QueueReset(t *testing.T) {
	entry := &SessionEntry{
		SessionKey:      "test",
		QueueMode:       "followup",
		QueueDebounceMs: 500,
		QueueCap:        3,
		QueueDrop:       "old",
	}
	PersistInlineDirectives(PersistDirectiveParams{
		Directives:   InlineDirectives{HasQueueDirective: true, QueueReset: true},
		SessionEntry: entry,
		SessionKey:   "test",
	})
	if entry.QueueMode != "" || entry.QueueDebounceMs != 0 || entry.QueueCap != 0 || entry.QueueDrop != "" {
		t.Error("queue fields should be cleared on reset")
	}
}

func TestPersistInlineDirectives_ExecHost(t *testing.T) {
	entry := &SessionEntry{SessionKey: "test"}
	PersistInlineDirectives(PersistDirectiveParams{
		Directives:   InlineDirectives{HasExecDirective: true, HasExecOptions: true, ExecHost: ExecHostSandbox},
		SessionEntry: entry,
		SessionKey:   "test",
	})
	if entry.ExecHost != "sandbox" {
		t.Errorf("ExecHost = %q, want 'sandbox'", entry.ExecHost)
	}
}

func TestPersistInlineDirectives_NilEntry(t *testing.T) {
	result := PersistInlineDirectives(PersistDirectiveParams{
		Directives: InlineDirectives{HasThinkDirective: true, ThinkLevel: autoreply.ThinkHigh},
		SessionKey: "test",
	})
	// Should not panic, just return defaults.
	if result.Provider != "" {
		t.Errorf("unexpected provider %q", result.Provider)
	}
}

func TestPersistInlineDirectives_SaveSessionCalled(t *testing.T) {
	entry := &SessionEntry{SessionKey: "test"}
	saveCalled := false
	PersistInlineDirectives(PersistDirectiveParams{
		Directives:   InlineDirectives{HasThinkDirective: true, ThinkLevel: autoreply.ThinkHigh},
		SessionEntry: entry,
		SessionKey:   "test",
		SaveSessionFn: func(e *SessionEntry) {
			saveCalled = true
		},
	})
	if !saveCalled {
		t.Error("SaveSessionFn should have been called")
	}
}
