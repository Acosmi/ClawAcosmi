package sessions

import (
	"testing"
)

func TestIsThreadSessionKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"agent:main:telegram:thread:123", true},
		{"agent:main:slack:topic:general", true},
		{"agent:main:telegram:group:mygrp", false},
		{"agent:main:main", false},
		{"", false},
		{":thread:", true},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := IsThreadSessionKey(tt.key); got != tt.want {
				t.Errorf("IsThreadSessionKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestResolveSessionResetType(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		isGroup  bool
		isThread bool
		want     SessionResetType
	}{
		{"thread flag", "", false, true, ResetTypeThread},
		{"thread key", "agent:main:telegram:thread:123", false, false, ResetTypeThread},
		{"group flag", "", true, false, ResetTypeGroup},
		{"group key", "agent:main:telegram:group:abc", false, false, ResetTypeGroup},
		{"direct", "agent:main:main", false, false, ResetTypeDirect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSessionResetType(tt.key, tt.isGroup, tt.isThread)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveThreadFlag(t *testing.T) {
	tests := []struct {
		name   string
		params ThreadFlagParams
		want   bool
	}{
		{"with messageThreadId", ThreadFlagParams{MessageThreadID: "123"}, true},
		{"with threadLabel", ThreadFlagParams{ThreadLabel: "  hello  "}, true},
		{"with threadStarterBody", ThreadFlagParams{ThreadStarterBody: "msg"}, true},
		{"with parentSessionKey", ThreadFlagParams{ParentSessionKey: "parent"}, true},
		{"with thread sessionKey", ThreadFlagParams{SessionKey: "foo:thread:bar"}, true},
		{"none", ThreadFlagParams{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveThreadFlag(tt.params); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveDailyResetAtMs(t *testing.T) {
	// 2026-01-15 10:00:00 UTC → reset at 4:00 → same day 4:00
	nowMs := int64(1768467600000) // approx 2026-01-15 10:00 UTC
	resetAt := ResolveDailyResetAtMs(nowMs, 4)
	if resetAt >= nowMs {
		t.Errorf("expected resetAt < nowMs, got resetAt=%d, nowMs=%d", resetAt, nowMs)
	}
}

func TestEvaluateSessionFreshness(t *testing.T) {
	nowMs := int64(1700000000000) // some timestamp
	t.Run("fresh daily", func(t *testing.T) {
		policy := SessionResetPolicy{Mode: ResetModeDaily, AtHour: 4}
		result := EvaluateSessionFreshness(nowMs-1000, nowMs, policy)
		if !result.Fresh {
			t.Error("expected fresh")
		}
	})
	t.Run("stale idle", func(t *testing.T) {
		idleMin := 5
		policy := SessionResetPolicy{Mode: ResetModeIdle, AtHour: 4, IdleMinutes: &idleMin}
		result := EvaluateSessionFreshness(nowMs-600_000, nowMs, policy) // 10 min idle
		if result.Fresh {
			t.Error("expected stale")
		}
	})
}

func TestResolveSessionResetPolicy(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		policy := ResolveSessionResetPolicy(nil, ResetTypeDirect, nil)
		if policy.Mode != ResetModeDaily {
			t.Errorf("expected daily, got %v", policy.Mode)
		}
		if policy.AtHour != DefaultResetAtHour {
			t.Errorf("expected atHour=%d, got %d", DefaultResetAtHour, policy.AtHour)
		}
	})
	t.Run("idle override", func(t *testing.T) {
		override := &SessionResetConfig{Mode: "idle", IdleMinutes: intPtrSess(30)}
		policy := ResolveSessionResetPolicy(nil, ResetTypeDirect, override)
		if policy.Mode != ResetModeIdle {
			t.Errorf("expected idle, got %v", policy.Mode)
		}
		if policy.IdleMinutes == nil || *policy.IdleMinutes != 30 {
			t.Error("expected idleMinutes=30")
		}
	})
	t.Run("legacy idleMinutes", func(t *testing.T) {
		cfg := &SessionConfig{IdleMinutes: intPtrSess(45)}
		policy := ResolveSessionResetPolicy(cfg, ResetTypeDirect, nil)
		if policy.Mode != ResetModeIdle {
			t.Errorf("expected idle mode for legacy, got %v", policy.Mode)
		}
	})
}

func intPtrSess(v int) *int { return &v }
