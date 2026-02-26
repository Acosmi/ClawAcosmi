package scope

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

func TestResolveAgentTimeoutSeconds_Default(t *testing.T) {
	s := ResolveAgentTimeoutSeconds(nil)
	if s != DefaultAgentTimeoutSeconds {
		t.Errorf("default = %d, want %d", s, DefaultAgentTimeoutSeconds)
	}
}

func TestResolveAgentTimeoutSeconds_Configured(t *testing.T) {
	ts := 120
	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			Defaults: &types.AgentDefaultsConfig{
				TimeoutSeconds: &ts,
			},
		},
	}
	s := ResolveAgentTimeoutSeconds(cfg)
	if s != 120 {
		t.Errorf("configured = %d, want 120", s)
	}
}

func TestResolveAgentTimeoutMs(t *testing.T) {
	// Default
	ms := ResolveAgentTimeoutMs(TimeoutOptions{})
	if ms != 600000 {
		t.Errorf("default = %d, want 600000", ms)
	}

	// Override ms = 0 → no timeout
	zero := 0
	ms = ResolveAgentTimeoutMs(TimeoutOptions{OverrideMs: &zero})
	if ms != MaxSafeTimeoutMs {
		t.Errorf("overrideMs=0 = %d, want %d", ms, MaxSafeTimeoutMs)
	}

	// Override seconds
	thirtySeconds := 30
	ms = ResolveAgentTimeoutMs(TimeoutOptions{OverrideSeconds: &thirtySeconds})
	if ms != 30000 {
		t.Errorf("overrideSeconds=30 = %d, want 30000", ms)
	}

	// Negative → fallback
	neg := -1
	ms = ResolveAgentTimeoutMs(TimeoutOptions{OverrideMs: &neg})
	if ms != 600000 {
		t.Errorf("overrideMs=-1 = %d, want 600000", ms)
	}
}

func TestNormalizeUsage(t *testing.T) {
	// nil
	if NormalizeUsage(nil) != nil {
		t.Error("nil should return nil")
	}

	// Standard fields
	inp := 100
	out := 50
	u := NormalizeUsage(&UsageRaw{Input: &inp, Output: &out})
	if u == nil {
		t.Fatal("should not be nil")
	}
	if u.Input != 100 || u.Output != 50 {
		t.Errorf("input=%d, output=%d, want 100,50", u.Input, u.Output)
	}

	// Snake case fields
	pt := 200
	ct := 80
	u = NormalizeUsage(&UsageRaw{PromptTokensSnake: &pt, CompletionTokensSnake: &ct})
	if u == nil {
		t.Fatal("should not be nil")
	}
	if u.Input != 200 || u.Output != 80 {
		t.Errorf("snake: input=%d, output=%d, want 200,80", u.Input, u.Output)
	}

	// All zero → nil
	z := 0
	u = NormalizeUsage(&UsageRaw{Input: &z, Output: &z})
	if u != nil {
		t.Error("all zero should return nil")
	}
}

func TestHasNonzeroUsage(t *testing.T) {
	if HasNonzeroUsage(nil) {
		t.Error("nil = true, want false")
	}
	if HasNonzeroUsage(&NormalizedUsage{}) {
		t.Error("zero = true, want false")
	}
	if !HasNonzeroUsage(&NormalizedUsage{Input: 1}) {
		t.Error("nonzero = false, want true")
	}
}

func TestDeriveSessionTotalTokens(t *testing.T) {
	u := &NormalizedUsage{Input: 100, CacheRead: 50, CacheWrite: 10}
	total := DeriveSessionTotalTokens(u, 0)
	if total != 160 { // 100+50+10
		t.Errorf("total = %d, want 160", total)
	}

	// Capped by context
	total = DeriveSessionTotalTokens(u, 120)
	if total != 120 {
		t.Errorf("capped = %d, want 120", total)
	}
}
