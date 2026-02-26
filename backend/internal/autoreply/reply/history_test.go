package reply

import (
	"testing"
)

func TestBuildHistoryContext(t *testing.T) {
	tests := []struct {
		name    string
		history string
		current string
		want    string
	}{
		{"empty history", "", "hello", "hello"},
		{"with history", "Alice: hi", "Bob: hello",
			"[Chat messages since your last reply - for context]\nAlice: hi\n\n[Current message - respond to this]\nBob: hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildHistoryContext(tt.history, tt.current, "\n")
			if result != tt.want {
				t.Errorf("BuildHistoryContext = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestHistoryMapAppendAndEvict(t *testing.T) {
	m := NewHistoryMap()

	// Append entries
	entries := m.AppendHistoryEntry("session1", HistoryEntry{Sender: "Alice", Body: "hi"}, 3)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	m.AppendHistoryEntry("session1", HistoryEntry{Sender: "Bob", Body: "hello"}, 3)
	m.AppendHistoryEntry("session1", HistoryEntry{Sender: "Alice", Body: "how are you"}, 3)
	m.AppendHistoryEntry("session1", HistoryEntry{Sender: "Bob", Body: "good"}, 3) // should evict first

	entries = m.GetEntries("session1")
	if len(entries) != 3 {
		t.Errorf("expected 3 entries after limit, got %d", len(entries))
	}
	if entries[0].Body != "hello" {
		t.Errorf("first entry should be 'hello', got %q", entries[0].Body)
	}
}

func TestHistoryMapClearEntries(t *testing.T) {
	m := NewHistoryMap()
	m.AppendHistoryEntry("key1", HistoryEntry{Sender: "A", Body: "test"}, 10)
	m.ClearEntries("key1")
	entries := m.GetEntries("key1")
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(entries))
	}
}

func TestHistoryMapLRUEviction(t *testing.T) {
	m := NewHistoryMap()
	// Add more than MaxHistoryKeys entries
	for i := 0; i < MaxHistoryKeys+10; i++ {
		key := "key" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		m.AppendHistoryEntry(key, HistoryEntry{Sender: "test", Body: "test"}, 5)
	}
	if len(m.entries) > MaxHistoryKeys {
		t.Errorf("expected at most %d keys, got %d", MaxHistoryKeys, len(m.entries))
	}
}

func TestBuildHistoryContextFromEntries(t *testing.T) {
	entries := []HistoryEntry{
		{Sender: "Alice", Body: "hi"},
		{Sender: "Bob", Body: "hello"},
	}
	format := func(e HistoryEntry) string { return e.Sender + ": " + e.Body }

	// excludeLast = true (default)
	result := BuildHistoryContextFromEntries(entries, "current", format, "\n", true)
	if result == "current" {
		t.Error("should include first entry")
	}

	// excludeLast = false
	result2 := BuildHistoryContextFromEntries(entries, "current", format, "\n", false)
	if result2 == "current" {
		t.Error("should include all entries")
	}
}

func TestBuildHistoryContextFromMap(t *testing.T) {
	m := NewHistoryMap()
	entry := HistoryEntry{Sender: "Test", Body: "msg"}
	format := func(e HistoryEntry) string { return e.Sender + ": " + e.Body }

	// With new entry appended
	result := BuildHistoryContextFromMap(m, "key1", 10, &entry, "current", format, "\n", false)
	if result == "current" {
		t.Error("should include history")
	}
}

func TestHistoryMapLimitZero(t *testing.T) {
	m := NewHistoryMap()
	entries := m.AppendHistoryEntry("key", HistoryEntry{Sender: "A", Body: "test"}, 0)
	if entries != nil {
		t.Errorf("expected nil entries for limit 0, got %v", entries)
	}
}
