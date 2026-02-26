package runner

// ============================================================================
// CompactionFailureEmitter — compaction 失败事件分发器
// TS 对照: google.ts → compactionFailureEmitter (EventEmitter)
//
// Go 实现: sync.Mutex + []CompactionFailureCallback 回调列表
// ============================================================================

import (
	"log/slog"
	"sync"
)

// CompactionFailureEmitter compaction 失败事件分发器。
// 线程安全；支持注册/注销回调。
// TS 对照: google.ts → compactionFailureEmitter = new EventEmitter()
type CompactionFailureEmitter struct {
	mu        sync.Mutex
	listeners []compactionListener
	nextID    uint64
}

type compactionListener struct {
	id uint64
	cb CompactionFailureCallback
}

// OnCompactionFailure 注册 compaction 失败回调，返回取消函数。
// TS 对照: google.ts → onUnhandledCompactionFailure(cb)
func (e *CompactionFailureEmitter) OnCompactionFailure(cb CompactionFailureCallback) func() {
	e.mu.Lock()
	defer e.mu.Unlock()

	id := e.nextID
	e.nextID++
	e.listeners = append(e.listeners, compactionListener{id: id, cb: cb})

	return func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		for i, l := range e.listeners {
			if l.id == id {
				e.listeners = append(e.listeners[:i], e.listeners[i+1:]...)
				return
			}
		}
	}
}

// EmitCompactionFailure 触发 compaction 失败事件，通知所有已注册的回调。
// 每个回调在 defer/recover 中执行，防止 panic 传播。
// TS 对照: compactionFailureEmitter.emit("failure", reason)
func (e *CompactionFailureEmitter) EmitCompactionFailure(reason string) {
	e.mu.Lock()
	snapshot := make([]CompactionFailureCallback, len(e.listeners))
	for i, l := range e.listeners {
		snapshot[i] = l.cb
	}
	e.mu.Unlock()

	for _, cb := range snapshot {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("compaction failure callback panicked",
						"reason", reason,
						"panic", r)
				}
			}()
			cb(reason)
		}()
	}
}

// DefaultCompactionFailureEmitter 全局默认 compaction 失败分发器。
var DefaultCompactionFailureEmitter = &CompactionFailureEmitter{}
