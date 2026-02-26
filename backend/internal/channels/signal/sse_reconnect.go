package signal

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Signal SSE 重连循环 — 继承自 src/signal/sse-reconnect.ts (81L)

// SSEReconnectConfig 重连配置
type SSEReconnectConfig struct {
	BaseURL    string
	Account    string
	Handler    func(SignalSSEvent)
	LogInfo    func(msg string)
	LogError   func(msg string)
	MinBackoff time.Duration // 默认 1s
	MaxBackoff time.Duration // 默认 30s
}

// RunSignalSseLoop 运行 SSE 事件循环，自动重连
// 通过 ctx 控制生命周期，阻塞直到 ctx 取消
func RunSignalSseLoop(ctx context.Context, cfg SSEReconnectConfig) error {
	minBackoff := cfg.MinBackoff
	if minBackoff <= 0 {
		minBackoff = 1 * time.Second
	}
	maxBackoff := cfg.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 30 * time.Second
	}

	attempt := 0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if cfg.LogInfo != nil {
			if attempt == 0 {
				cfg.LogInfo("signal SSE: connecting...")
			} else {
				cfg.LogInfo("signal SSE: reconnecting...")
			}
		}

		// 对齐 TS: 成功收到事件时重置重连计数器
		err := StreamSignalEvents(ctx, cfg.BaseURL, cfg.Account, func(event SignalSSEvent) {
			attempt = 0
			cfg.Handler(event)
		})

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			if cfg.LogError != nil {
				cfg.LogError("signal SSE error: " + err.Error())
			}
		}

		// 指数退避 + jitter
		attempt++
		backoff := time.Duration(float64(minBackoff) * math.Pow(2, float64(attempt-1)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		// 添加最多 25% 的 jitter
		jitter := time.Duration(rand.Int63n(int64(backoff / 4)))
		backoff += jitter

		if cfg.LogInfo != nil {
			cfg.LogInfo("signal SSE: waiting " + backoff.String() + " before reconnect")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}
}
