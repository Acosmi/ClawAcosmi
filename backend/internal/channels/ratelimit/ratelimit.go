// Package ratelimit 提供通用的令牌桶限速器封装。
//
// 等价 TS 端 @grammyjs/transformer-throttler（Bottleneck 令牌桶）。
// 使用 golang.org/x/time/rate 实现。
//
// 各通道的默认限速参数：
//   - Telegram: 30 req/s (burst 30) — Bot API 全局限制
//   - Slack:    1 req/s  (burst 3)  — Tier 2 通用保守值
//   - LINE:   100 req/s  (burst 100) — Push API 限制
package ratelimit

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// ChannelLimiter 通道级别的令牌桶限速器。
// 线程安全，可被多个 goroutine 共享。
type ChannelLimiter struct {
	limiter *rate.Limiter
}

// NewChannelLimiter 创建限速器。
//   - rps: 每秒允许的请求数
//   - burst: 突发容量（令牌桶大小）
func NewChannelLimiter(rps float64, burst int) *ChannelLimiter {
	return &ChannelLimiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

// Wait 阻塞等待直到获取一个令牌，或 ctx 被取消。
// 对齐 grammy throttler 的排队等待行为。
func (l *ChannelLimiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

// Allow 非阻塞检查是否允许一次请求（消耗一个令牌）。
// 如果桶满则返回 true 并消耗令牌，否则返回 false。
func (l *ChannelLimiter) Allow() bool {
	return l.limiter.Allow()
}

// ── 各平台预置限速器 ──

// DefaultTelegramLimiter 创建 Telegram 默认限速器。
// Bot API 限制: 约 30 msg/s 全局，单聊 1 msg/s（此处控制全局）。
func DefaultTelegramLimiter() *ChannelLimiter {
	return NewChannelLimiter(30, 30)
}

// DefaultSlackLimiter 创建 Slack 默认限速器。
// Web API Tier 2: ~1 req/s，burst 3 允许短时突发。
func DefaultSlackLimiter() *ChannelLimiter {
	return NewChannelLimiter(1, 3)
}

// DefaultLineLimiter 创建 LINE 默认限速器。
// Messaging API: push 约 100 req/s。
func DefaultLineLimiter() *ChannelLimiter {
	return NewChannelLimiter(100, 100)
}

// ── 全局单例（懒初始化） ──

var (
	telegramLimiter     *ChannelLimiter
	telegramLimiterOnce sync.Once

	slackLimiters   = make(map[string]*ChannelLimiter)
	slackLimitersMu sync.Mutex
)

// GlobalTelegramLimiter 返回全局 Telegram 限速器（懒初始化）。
func GlobalTelegramLimiter() *ChannelLimiter {
	telegramLimiterOnce.Do(func() {
		telegramLimiter = DefaultTelegramLimiter()
	})
	return telegramLimiter
}

// GetSlackLimiter 获取按 token 分隔的 Slack 限速器。
// 每个 SlackWebClient 使用独立限速器，按 token hash 索引。
func GetSlackLimiter(tokenKey string) *ChannelLimiter {
	slackLimitersMu.Lock()
	defer slackLimitersMu.Unlock()
	if l, ok := slackLimiters[tokenKey]; ok {
		return l
	}
	l := DefaultSlackLimiter()
	slackLimiters[tokenKey] = l
	return l
}
