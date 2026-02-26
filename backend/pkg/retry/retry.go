// Package retry 提供可重用的重试与退避策略。
//
// 继承自原版 src/infra/retry.ts 和 backoff.ts 的功能。
//
// 功能清单:
//   - Do: 基础重试 (error-only)
//   - DoWithResult[T]: 泛型重试 (返回值 + error)
//   - ShouldRetry: 自定义重试条件判断
//   - OnRetry: 重试事件回调
//   - RetryAfterMs: 服务端建议的重试延迟 (如 HTTP 429)
//   - BackoffPolicy: 退避策略配置
//
// TS 依赖: 无 (仅使用 setTimeout/sleep)
// Go 替代: context + time 标准库
package retry

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"
)

// Config 重试配置
type Config struct {
	// MaxAttempts 最大尝试次数（包含首次）
	MaxAttempts int
	// InitialDelay 初始延迟
	InitialDelay time.Duration
	// MaxDelay 最大延迟上限
	MaxDelay time.Duration
	// Multiplier 延迟倍数（指数退避）
	Multiplier float64
	// JitterFactor 随机抖动系数 (0~1)，0 表示不抖动。
	// 例如 0.1 表示 ±10%，0.25 表示 ±25%。
	// 对应 TS RetryConfig.jitter。
	JitterFactor float64

	// ShouldRetry 自定义重试条件判断。返回 false 则不再重试。
	// 对应原版 RetryOptions.shouldRetry
	ShouldRetry func(err error, attempt int) bool

	// OnRetry 重试事件回调（用于日志记录等）。
	// 对应原版 RetryOptions.onRetry
	OnRetry func(info RetryInfo)

	// RetryAfterHint 从错误中提取服务端建议的重试延迟。
	// 返回 0 表示无建议。对应原版 RetryOptions.retryAfterMs
	RetryAfterHint func(err error) time.Duration

	// Label 操作标签（用于日志）
	Label string
}

// RetryInfo 重试事件信息。
// 对应原版 RetryInfo 类型。
type RetryInfo struct {
	Attempt     int
	MaxAttempts int
	Delay       time.Duration
	Err         error
	Label       string
}

// DefaultConfig 默认重试配置
var DefaultConfig = Config{
	MaxAttempts:  3,
	InitialDelay: 300 * time.Millisecond,
	MaxDelay:     30 * time.Second,
	Multiplier:   2.0,
	JitterFactor: 0.1,
}

// Do 执行带重试的操作
func Do(ctx context.Context, cfg Config, fn func(attempt int) error) error {
	_, err := DoWithResult(ctx, cfg, func(attempt int) (struct{}, error) {
		return struct{}{}, fn(attempt)
	})
	return err
}

// DoWithResult 执行带重试的操作并返回结果。
// 对应原版 retryAsync<T>。
func DoWithResult[T any](ctx context.Context, cfg Config, fn func(attempt int) (T, error)) (T, error) {
	cfg = resolveConfig(cfg)
	var zero T
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err := fn(attempt)
		if err == nil {
			return result, nil
		}
		lastErr = err

		// 最后一次尝试失败不再等待
		if attempt >= cfg.MaxAttempts {
			break
		}

		// 检查自定义重试条件
		if cfg.ShouldRetry != nil && !cfg.ShouldRetry(err, attempt) {
			break
		}

		// 计算延迟
		delay := calculateDelay(cfg, attempt, err)

		// 触发 OnRetry 回调
		if cfg.OnRetry != nil {
			cfg.OnRetry(RetryInfo{
				Attempt:     attempt,
				MaxAttempts: cfg.MaxAttempts,
				Delay:       delay,
				Err:         err,
				Label:       cfg.Label,
			})
		}

		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(delay):
			// 继续下一次重试
		}
	}

	return zero, fmt.Errorf("failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// resolveConfig 填充配置中的默认值。
// 对应原版 resolveRetryConfig。
func resolveConfig(cfg Config) Config {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultConfig.MaxAttempts
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = DefaultConfig.InitialDelay
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = DefaultConfig.MaxDelay
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = DefaultConfig.Multiplier
	}
	if cfg.MaxDelay < cfg.InitialDelay {
		cfg.MaxDelay = cfg.InitialDelay
	}
	return cfg
}

// calculateDelay 计算退避延迟。
// 支持服务端建议延迟 (RetryAfterHint)。
// 对齐 TS: retryAsync 中的 delay 计算逻辑（含 applyJitter + clamp）。
func calculateDelay(cfg Config, attempt int, err error) time.Duration {
	var delay float64

	// 检查服务端建议延迟
	if cfg.RetryAfterHint != nil {
		if hint := cfg.RetryAfterHint(err); hint > 0 {
			delay = math.Max(float64(hint), float64(cfg.InitialDelay))
			// DY-003: 对 hint delay 也应用 jitter，对齐 TS applyJitter(delay, jitter)
			delay = applyJitter(delay, cfg.JitterFactor)
			// clamp
			delay = math.Max(delay, float64(cfg.InitialDelay))
			if delay > float64(cfg.MaxDelay) {
				delay = float64(cfg.MaxDelay)
			}
			return time.Duration(delay)
		}
	}

	// 指数退避
	delay = float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))

	// DY-002: 使用 JitterFactor 系数，对齐 TS applyJitter(delay, jitter)
	delay = applyJitter(delay, cfg.JitterFactor)

	// clamp
	delay = math.Max(delay, float64(cfg.InitialDelay))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	return time.Duration(delay)
}

// applyJitter 对 delay 施加 ±jitterFactor 的随机抖动。
// 对齐 TS: function applyJitter(delayMs, jitter)
func applyJitter(delay float64, jitterFactor float64) float64 {
	if jitterFactor <= 0 {
		return delay
	}
	offset := (rand.Float64()*2 - 1) * jitterFactor
	result := delay * (1 + offset)
	if result < 0 {
		return 0
	}
	return math.Round(result)
}

// ─── BackoffPolicy (backoff.ts) ───

// BackoffPolicy 退避策略配置。
// 对应原版 backoff.ts 的 BackoffPolicy 类型。
type BackoffPolicy struct {
	InitialMs int     // 初始延迟（毫秒）
	MaxMs     int     // 最大延迟（毫秒）
	Factor    float64 // 增长因子
	Jitter    float64 // 抖动系数 (0~1)
}

// DefaultBackoff 默认退避策略
var DefaultBackoff = BackoffPolicy{
	InitialMs: 300,
	MaxMs:     30000,
	Factor:    2.0,
	Jitter:    0.25,
}

// ComputeBackoff 根据策略和尝试次数计算退避延迟。
// 对应原版 computeBackoff()。
func ComputeBackoff(policy BackoffPolicy, attempt int) time.Duration {
	base := float64(policy.InitialMs) * math.Pow(policy.Factor, float64(max(attempt-1, 0)))
	jitter := base * policy.Jitter * rand.Float64()
	delayMs := math.Min(float64(policy.MaxMs), math.Round(base+jitter))
	return time.Duration(delayMs) * time.Millisecond
}

// SleepWithContext 带取消的延迟等待。
// 对应原版 sleepWithAbort()。
func SleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// IsRetryable 检查错误是否可重试（简单实现，可被 ShouldRetry 覆盖）。
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// context 取消不可重试
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	return true
}
