// Package ratelimit provides a Redis-backed Token Bucket rate limiter.
// Extracted to a standalone package to avoid import cycles between
// middleware and services (services imports middleware for TenantFromCtx).
package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Result holds the outcome of a rate limit check.
type Result struct {
	Allowed   bool
	Remaining int
	ResetTime int64 // Unix timestamp
}

// Headers returns standard rate limit response headers.
func (r Result) Headers() map[string]string {
	return map[string]string{
		"X-RateLimit-Remaining": strconv.Itoa(r.Remaining),
		"X-RateLimit-Reset":     strconv.FormatInt(r.ResetTime, 10),
	}
}

// Token Bucket Lua script for atomic rate limiting.
// Prevents race conditions in distributed environments.
const rateLimitScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local current = redis.call('GET', key)

if current == false then
    redis.call('SET', key, limit - 1, 'EX', window)
    return {1, limit - 1, now + window}
end

current = tonumber(current)

if current > 0 then
    local new_count = redis.call('DECR', key)
    local ttl = redis.call('TTL', key)
    return {1, new_count, now + ttl}
end

local ttl = redis.call('TTL', key)
return {0, 0, now + ttl}
`

// Limiter implements a Token Bucket rate limiter backed by Redis.
// Degrades gracefully when Redis is unavailable (fail-open or fail-closed).
type Limiter struct {
	client        *redis.Client
	failOpen      bool
	defaultLimit  int
	defaultWindow int // seconds
	scriptSHA     string
	mu            sync.Mutex
}

// New creates a new Limiter.
// If client is nil, the limiter degrades based on failOpen setting.
// failOpen=true allows all requests when Redis is unavailable (recommended for production).
func New(client *redis.Client, failOpen bool) *Limiter {
	return &Limiter{
		client:        client,
		failOpen:      failOpen,
		defaultLimit:  100,
		defaultWindow: 60,
	}
}

// ensureScript loads and caches the Lua script SHA.
func (rl *Limiter) ensureScript(ctx context.Context) (string, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.scriptSHA != "" {
		return rl.scriptSHA, nil
	}
	if rl.client == nil {
		return "", fmt.Errorf("redis client is nil")
	}

	sha, err := rl.client.ScriptLoad(ctx, rateLimitScript).Result()
	if err != nil {
		return "", fmt.Errorf("load lua script: %w", err)
	}
	rl.scriptSHA = sha
	return sha, nil
}

// Check determines if a request is allowed under the rate limit.
// key should be in format "rate:user:{id}" or "rate:user:{id}:{endpoint}".
func (rl *Limiter) Check(ctx context.Context, key string, limit, window int) Result {
	if limit <= 0 {
		limit = rl.defaultLimit
	}
	if window <= 0 {
		window = rl.defaultWindow
	}
	now := time.Now().Unix()

	if rl.client == nil {
		return rl.fallback(limit, now, int64(window))
	}

	sha, err := rl.ensureScript(ctx)
	if err != nil {
		slog.Warn("Rate limit script load failed", "error", err)
		return rl.fallback(limit, now, int64(window))
	}

	result, err := rl.client.EvalSha(ctx, sha, []string{key},
		strconv.Itoa(limit),
		strconv.Itoa(window),
		strconv.FormatInt(now, 10),
	).Int64Slice()

	if err != nil {
		slog.Error("Rate limit check failed", "error", err, "key", key)
		return rl.fallback(limit, now, int64(window))
	}

	return Result{
		Allowed:   result[0] == 1,
		Remaining: int(result[1]),
		ResetTime: result[2],
	}
}

// Reset removes a rate limit key (e.g., admin override).
func (rl *Limiter) Reset(ctx context.Context, key string) bool {
	if rl.client == nil {
		return false
	}
	if err := rl.client.Del(ctx, key).Err(); err != nil {
		slog.Error("Failed to reset rate limit", "error", err, "key", key)
		return false
	}
	return true
}

func (rl *Limiter) fallback(limit int, now, window int64) Result {
	if rl.failOpen {
		return Result{Allowed: true, Remaining: limit, ResetTime: now + window}
	}
	return Result{Allowed: false, Remaining: 0, ResetTime: now + window}
}
