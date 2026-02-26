package gateway

// idempotency.go — 请求幂等去重缓存
// 对应 TS src/gateway/server-methods 中 idempotencyKey 逻辑。
//
// 使用 sync.Map + TTL 实现：
// - CheckOrRegister: 原子查重+注册
// - Complete: 标记完成+缓存结果
// - 后台 reaper goroutine 定期清理过期条目

import (
	"sync"
	"time"
)

// IdempotencyState 标识请求所处阶段。
type IdempotencyState int

const (
	// IdempotencyInFlight 请求正在处理中。
	IdempotencyInFlight IdempotencyState = iota
	// IdempotencyCompleted 请求已完成，有缓存结果。
	IdempotencyCompleted
)

// IdempotencyEntry 缓存条目。
type IdempotencyEntry struct {
	State     IdempotencyState
	Result    interface{} // 完成后的缓存结果
	CreatedAt time.Time
}

// IdempotencyCache 基于 sync.Map 的幂等去重缓存。
type IdempotencyCache struct {
	entries sync.Map // key → *IdempotencyEntry
	ttl     time.Duration
	done    chan struct{} // 关闭信号
}

// DefaultIdempotencyTTL 默认 TTL: 5 分钟。
const DefaultIdempotencyTTL = 5 * time.Minute

// NewIdempotencyCache 创建幂等缓存。ttl <= 0 使用默认值。
func NewIdempotencyCache(ttl time.Duration) *IdempotencyCache {
	if ttl <= 0 {
		ttl = DefaultIdempotencyTTL
	}
	c := &IdempotencyCache{
		ttl:  ttl,
		done: make(chan struct{}),
	}
	go c.reaper()
	return c
}

// CheckResult 查重结果。
type CheckResult struct {
	// IsDuplicate 为 true 表示 key 已存在。
	IsDuplicate bool
	// State 仅当 IsDuplicate=true 时有效：InFlight 或 Completed。
	State IdempotencyState
	// CachedResult 仅当 State=Completed 时有效。
	CachedResult interface{}
}

// CheckOrRegister 原子查重+注册。
// 如果 key 为空，直接返回 not duplicate（跳过幂等检查）。
// 如果 key 不存在，注册为 InFlight 并返回 IsDuplicate=false。
// 如果 key 已存在，返回 IsDuplicate=true + 当前状态。
func (c *IdempotencyCache) CheckOrRegister(key string) CheckResult {
	if key == "" {
		return CheckResult{IsDuplicate: false}
	}

	now := time.Now()
	newEntry := &IdempotencyEntry{
		State:     IdempotencyInFlight,
		CreatedAt: now,
	}

	existing, loaded := c.entries.LoadOrStore(key, newEntry)
	if !loaded {
		// 新注册成功
		return CheckResult{IsDuplicate: false}
	}

	// key 已存在，检查是否过期
	entry := existing.(*IdempotencyEntry)
	if now.Sub(entry.CreatedAt) > c.ttl {
		// 已过期，覆盖为新条目
		c.entries.Store(key, newEntry)
		return CheckResult{IsDuplicate: false}
	}

	return CheckResult{
		IsDuplicate:  true,
		State:        entry.State,
		CachedResult: entry.Result,
	}
}

// Complete 将 InFlight 条目标记为完成，缓存结果。
func (c *IdempotencyCache) Complete(key string, result interface{}) {
	if key == "" {
		return
	}
	val, ok := c.entries.Load(key)
	if !ok {
		return
	}
	entry := val.(*IdempotencyEntry)
	entry.State = IdempotencyCompleted
	entry.Result = result
}

// Remove 移除指定 key。
func (c *IdempotencyCache) Remove(key string) {
	c.entries.Delete(key)
}

// Close 停止后台 reaper。
func (c *IdempotencyCache) Close() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

// reaper 定期清理过期条目。
func (c *IdempotencyCache) reaper() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			now := time.Now()
			c.entries.Range(func(key, value interface{}) bool {
				entry := value.(*IdempotencyEntry)
				if now.Sub(entry.CreatedAt) > c.ttl {
					c.entries.Delete(key)
				}
				return true
			})
		}
	}
}
