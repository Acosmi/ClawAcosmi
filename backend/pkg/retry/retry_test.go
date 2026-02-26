package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond}
	callCount := 0
	err := Do(context.Background(), cfg, func(attempt int) error {
		callCount++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestDo_RetryThenSuccess(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond}
	callCount := 0
	err := Do(context.Background(), cfg, func(attempt int) error {
		callCount++
		if attempt < 3 {
			return fmt.Errorf("fail %d", attempt)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestDo_AllFail(t *testing.T) {
	cfg := Config{MaxAttempts: 2, InitialDelay: 10 * time.Millisecond}
	err := Do(context.Background(), cfg, func(attempt int) error {
		return fmt.Errorf("always fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fmt.Errorf("always fail")) {
		// 检查 wrapped 错误
		if err.Error() == "" {
			t.Error("error should not be empty")
		}
	}
}

func TestDo_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{MaxAttempts: 10, InitialDelay: 100 * time.Millisecond}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Do(ctx, cfg, func(attempt int) error {
		return fmt.Errorf("fail")
	})
	if err == nil {
		t.Fatal("expected error from cancel")
	}
}

func TestDoWithResult_Success(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond}
	result, err := DoWithResult(context.Background(), cfg, func(attempt int) (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestDoWithResult_RetryThenSuccess(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: 10 * time.Millisecond}
	result, err := DoWithResult(context.Background(), cfg, func(attempt int) (int, error) {
		if attempt < 2 {
			return 0, fmt.Errorf("fail")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestShouldRetry_StopsOnFalse(t *testing.T) {
	callCount := 0
	cfg := Config{
		MaxAttempts:  5,
		InitialDelay: 10 * time.Millisecond,
		ShouldRetry: func(err error, attempt int) bool {
			return attempt < 2 // 只重试一次
		},
	}
	err := Do(context.Background(), cfg, func(attempt int) error {
		callCount++
		return fmt.Errorf("fail %d", attempt)
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (original + 1 retry), got %d", callCount)
	}
}

func TestOnRetry_Called(t *testing.T) {
	var infos []RetryInfo
	cfg := Config{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		Label:        "test-op",
		OnRetry: func(info RetryInfo) {
			infos = append(infos, info)
		},
	}
	_ = Do(context.Background(), cfg, func(attempt int) error {
		if attempt < 3 {
			return fmt.Errorf("fail %d", attempt)
		}
		return nil
	})
	if len(infos) != 2 {
		t.Fatalf("expected 2 OnRetry calls, got %d", len(infos))
	}
	if infos[0].Attempt != 1 || infos[0].Label != "test-op" {
		t.Errorf("unexpected info[0]: %+v", infos[0])
	}
	if infos[1].Attempt != 2 {
		t.Errorf("unexpected info[1]: %+v", infos[1])
	}
}

func TestRetryAfterHint(t *testing.T) {
	cfg := Config{
		MaxAttempts:  2,
		InitialDelay: 10 * time.Millisecond,
		RetryAfterHint: func(err error) time.Duration {
			return 20 * time.Millisecond // 服务端建议 20ms
		},
	}

	start := time.Now()
	_ = Do(context.Background(), cfg, func(attempt int) error {
		return fmt.Errorf("rate limited")
	})
	elapsed := time.Since(start)

	// 至少等了 20ms (服务端建议)
	if elapsed < 15*time.Millisecond {
		t.Errorf("expected at least 15ms delay, got %v", elapsed)
	}
}

func TestBackoffPolicy_ComputeBackoff(t *testing.T) {
	policy := BackoffPolicy{
		InitialMs: 100,
		MaxMs:     5000,
		Factor:    2.0,
		Jitter:    0,
	}

	// attempt 1: 100ms
	d1 := ComputeBackoff(policy, 1)
	if d1 != 100*time.Millisecond {
		t.Errorf("attempt 1: expected 100ms, got %v", d1)
	}

	// attempt 2: 200ms
	d2 := ComputeBackoff(policy, 2)
	if d2 != 200*time.Millisecond {
		t.Errorf("attempt 2: expected 200ms, got %v", d2)
	}

	// attempt 3: 400ms
	d3 := ComputeBackoff(policy, 3)
	if d3 != 400*time.Millisecond {
		t.Errorf("attempt 3: expected 400ms, got %v", d3)
	}
}

func TestBackoffPolicy_MaxCap(t *testing.T) {
	policy := BackoffPolicy{
		InitialMs: 1000,
		MaxMs:     2000,
		Factor:    2.0,
		Jitter:    0,
	}

	d := ComputeBackoff(policy, 10) // 1000 * 2^9 = 512000 → capped at 2000
	if d != 2000*time.Millisecond {
		t.Errorf("expected 2000ms cap, got %v", d)
	}
}

func TestBackoffPolicy_WithJitter(t *testing.T) {
	policy := BackoffPolicy{
		InitialMs: 100,
		MaxMs:     5000,
		Factor:    2.0,
		Jitter:    0.5,
	}

	// jitter 使结果不确定，但应在合理范围内
	d := ComputeBackoff(policy, 1)
	if d < 100*time.Millisecond || d > 200*time.Millisecond {
		t.Errorf("attempt 1 with jitter: %v out of expected range [100ms, 200ms]", d)
	}
}

func TestSleepWithContext_Normal(t *testing.T) {
	err := SleepWithContext(context.Background(), 10*time.Millisecond)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSleepWithContext_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	err := SleepWithContext(ctx, time.Second)
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestSleepWithContext_Zero(t *testing.T) {
	err := SleepWithContext(context.Background(), 0)
	if err != nil {
		t.Errorf("unexpected error for zero duration: %v", err)
	}
}

func TestIsRetryable(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("nil error should not be retryable")
	}
	if !IsRetryable(fmt.Errorf("some error")) {
		t.Error("generic error should be retryable")
	}
	if IsRetryable(context.Canceled) {
		t.Error("context.Canceled should not be retryable")
	}
	if IsRetryable(context.DeadlineExceeded) {
		t.Error("context.DeadlineExceeded should not be retryable")
	}
}

func TestResolveConfig_Defaults(t *testing.T) {
	cfg := resolveConfig(Config{})
	if cfg.MaxAttempts != DefaultConfig.MaxAttempts {
		t.Errorf("MaxAttempts = %d, want %d", cfg.MaxAttempts, DefaultConfig.MaxAttempts)
	}
	if cfg.InitialDelay != DefaultConfig.InitialDelay {
		t.Errorf("InitialDelay = %v, want %v", cfg.InitialDelay, DefaultConfig.InitialDelay)
	}
	if cfg.Multiplier != DefaultConfig.Multiplier {
		t.Errorf("Multiplier = %v, want %v", cfg.Multiplier, DefaultConfig.Multiplier)
	}
}
