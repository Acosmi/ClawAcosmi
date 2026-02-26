package reply

import "testing"

func TestShouldRunMemoryFlush_NoTokens(t *testing.T) {
	if ShouldRunMemoryFlush(ShouldRunMemoryFlushParams{
		TotalTokens:         0,
		ContextWindowTokens: 128000,
		ReserveTokensFloor:  4000,
		SoftThresholdTokens: 4000,
	}) {
		t.Error("expected false for zero tokens")
	}
}

func TestShouldRunMemoryFlush_BelowThreshold(t *testing.T) {
	if ShouldRunMemoryFlush(ShouldRunMemoryFlushParams{
		TotalTokens:         100000,
		ContextWindowTokens: 128000,
		ReserveTokensFloor:  4000,
		SoftThresholdTokens: 4000,
	}) {
		t.Error("expected false below threshold (threshold=120000)")
	}
}

func TestShouldRunMemoryFlush_AboveThreshold(t *testing.T) {
	if !ShouldRunMemoryFlush(ShouldRunMemoryFlushParams{
		TotalTokens:                125000,
		CompactionCount:            2,
		MemoryFlushCompactionCount: 1,
		ContextWindowTokens:        128000,
		ReserveTokensFloor:         4000,
		SoftThresholdTokens:        4000,
	}) {
		t.Error("expected true above threshold")
	}
}

func TestShouldRunMemoryFlush_AlreadyFlushed(t *testing.T) {
	if ShouldRunMemoryFlush(ShouldRunMemoryFlushParams{
		TotalTokens:                125000,
		CompactionCount:            2,
		MemoryFlushCompactionCount: 2,
		ContextWindowTokens:        128000,
		ReserveTokensFloor:         4000,
		SoftThresholdTokens:        4000,
	}) {
		t.Error("expected false when already flushed for current compaction")
	}
}

func TestShouldRunMemoryFlush_ZeroThreshold(t *testing.T) {
	// contextWindow - reserve - soft = 1 - 1000 - 1000 < 0
	if ShouldRunMemoryFlush(ShouldRunMemoryFlushParams{
		TotalTokens:         100,
		ContextWindowTokens: 1,
		ReserveTokensFloor:  1000,
		SoftThresholdTokens: 1000,
	}) {
		t.Error("expected false for impossible threshold")
	}
}

func TestResolveMemoryFlushSettings_Default(t *testing.T) {
	s := ResolveMemoryFlushSettings(nil, nil)
	if s == nil {
		t.Fatal("expected non-nil settings by default")
	}
	if s.SoftThresholdTokens != DefaultMemoryFlushSoftTokens {
		t.Errorf("SoftThresholdTokens = %d, want %d", s.SoftThresholdTokens, DefaultMemoryFlushSoftTokens)
	}
	if s.ReserveTokensFloor != DefaultCompactionReserveTokensFloor {
		t.Errorf("ReserveTokensFloor = %d, want %d", s.ReserveTokensFloor, DefaultCompactionReserveTokensFloor)
	}
}

func TestResolveMemoryFlushSettings_Disabled(t *testing.T) {
	disabled := false
	s := ResolveMemoryFlushSettings(&MemoryFlushConfig{Enabled: &disabled}, nil)
	if s != nil {
		t.Error("expected nil when disabled")
	}
}

func TestResolveMemoryFlushContextWindowTokens(t *testing.T) {
	if got := ResolveMemoryFlushContextWindowTokens(0); got != DefaultContextTokens {
		t.Errorf("got %d, want %d for zero agent cfg", got, DefaultContextTokens)
	}
	if got := ResolveMemoryFlushContextWindowTokens(200000); got != 200000 {
		t.Errorf("got %d, want 200000 for explicit value", got)
	}
}
