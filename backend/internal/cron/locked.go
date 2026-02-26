package cron

import "sync"

// ============================================================================
// 串行化锁 — 确保 cron store 操作互斥
// 对应 TS: cron/service/locked.ts (23L)
// ============================================================================
//
// TS 使用 Promise chain 实现串行化：
//   let pending = Promise.resolve();
//   async function locked(fn) { pending = pending.then(fn); return pending; }
//
// Go 使用 sync.Mutex 实现同等语义：
//   - 同一时刻仅一个 goroutine 可执行 store 操作
//   - 避免竞态条件和数据损坏

// CronStoreLock 用于保护 cron store 的互斥锁
type CronStoreLock struct {
	mu sync.Mutex
}

// Locked 在互斥锁保护下执行 fn
func (l *CronStoreLock) Locked(fn func() error) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return fn()
}

// LockedValue 在互斥锁保护下执行 fn 并返回值
func LockedValue[T any](l *CronStoreLock, fn func() (T, error)) (T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return fn()
}
