package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestNewChannelLimiter(t *testing.T) {
	l := NewChannelLimiter(10, 5)
	if l == nil {
		t.Fatal("NewChannelLimiter returned nil")
	}
}

func TestAllow_BurstThenDeny(t *testing.T) {
	// burst=3, rps=1 → 前 3 个 Allow 应返回 true，第 4 个应返回 false
	l := NewChannelLimiter(1, 3)
	for i := 0; i < 3; i++ {
		if !l.Allow() {
			t.Fatalf("Allow() returned false at burst index %d", i)
		}
	}
	if l.Allow() {
		t.Fatal("Allow() should return false after burst exhausted")
	}
}

func TestWait_ContextCancel(t *testing.T) {
	// 先耗尽 burst，然后 Wait 应在 ctx 取消时返回错误
	l := NewChannelLimiter(0.1, 1)
	l.Allow() // 消耗唯一的 burst 令牌

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := l.Wait(ctx)
	if err == nil {
		t.Fatal("Wait should return error when context cancelled")
	}
}

func TestWait_Success(t *testing.T) {
	l := NewChannelLimiter(100, 1)
	ctx := context.Background()
	if err := l.Wait(ctx); err != nil {
		t.Fatalf("Wait should succeed: %v", err)
	}
}

func TestDefaultLimiters(t *testing.T) {
	tg := DefaultTelegramLimiter()
	if tg == nil {
		t.Fatal("DefaultTelegramLimiter returned nil")
	}

	sl := DefaultSlackLimiter()
	if sl == nil {
		t.Fatal("DefaultSlackLimiter returned nil")
	}

	ln := DefaultLineLimiter()
	if ln == nil {
		t.Fatal("DefaultLineLimiter returned nil")
	}
}

func TestGlobalTelegramLimiter_Singleton(t *testing.T) {
	a := GlobalTelegramLimiter()
	b := GlobalTelegramLimiter()
	if a != b {
		t.Fatal("GlobalTelegramLimiter should return same instance")
	}
}

func TestGetSlackLimiter_PerToken(t *testing.T) {
	a := GetSlackLimiter("token-a")
	b := GetSlackLimiter("token-b")
	if a == b {
		t.Fatal("different tokens should get different limiters")
	}
	c := GetSlackLimiter("token-a")
	if a != c {
		t.Fatal("same token should get same limiter")
	}
}
