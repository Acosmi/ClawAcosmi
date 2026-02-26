package autoreply

import (
	"sync"
	"time"
)

// TS 对照: auto-reply/inbound-debounce.ts (111L)

// DefaultDebounceMs 默认防抖毫秒数。
const DefaultDebounceMs = 750

// ResolveInboundDebounceMs 解析入站防抖毫秒数。
// TS 对照: inbound-debounce.ts L5-20
func ResolveInboundDebounceMs(configured int, defaultMs int) int {
	if defaultMs <= 0 {
		defaultMs = DefaultDebounceMs
	}
	if configured > 0 {
		return configured
	}
	return defaultMs
}

// InboundDebouncer 入站防抖器。
// 在指定延迟内收到新消息时重置定时器，到期后执行回调。
// TS 对照: inbound-debounce.ts InboundDebouncer class
type InboundDebouncer struct {
	mu       sync.Mutex
	delayMs  int
	timer    *time.Timer
	pending  []string
	callback func(messages []string)
}

// NewInboundDebouncer 创建入站防抖器。
func NewInboundDebouncer(delayMs int, callback func(messages []string)) *InboundDebouncer {
	if delayMs <= 0 {
		delayMs = DefaultDebounceMs
	}
	return &InboundDebouncer{
		delayMs:  delayMs,
		callback: callback,
	}
}

// Push 推入新消息并重置防抖定时器。
// TS 对照: inbound-debounce.ts L40-75
func (d *InboundDebouncer) Push(message string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.pending = append(d.pending, message)

	if d.timer != nil {
		d.timer.Stop()
	}

	d.timer = time.AfterFunc(time.Duration(d.delayMs)*time.Millisecond, func() {
		d.mu.Lock()
		messages := d.pending
		d.pending = nil
		d.mu.Unlock()

		if d.callback != nil && len(messages) > 0 {
			d.callback(messages)
		}
	})
}

// Cancel 取消待执行的回调。
// TS 对照: inbound-debounce.ts L77-90
func (d *InboundDebouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.pending = nil
}

// PendingCount 返回待处理消息数。
func (d *InboundDebouncer) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}
