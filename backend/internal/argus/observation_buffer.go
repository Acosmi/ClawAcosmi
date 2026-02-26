package argus

// observation_buffer.go — ObservationBuffer ring buffer 实现
//
// 线程安全的 ring buffer，存储最近 N 帧 VisionObservation。
// 主 Agent 通过查询接口获取视觉状态，0ms 延迟。

import (
	"sync"
	"sync/atomic"
)

// ObservationBuffer 观测帧环形缓冲区。
type ObservationBuffer struct {
	mu       sync.RWMutex
	frames   []*VisionObservation
	capacity int
	head     int // 下一个写入位置
	count    int // 当前帧数

	// 订阅者
	subMu   sync.RWMutex
	subs    map[uint64]chan *VisionObservation
	nextSub uint64

	// 最新关键帧缓存
	latestKeyframe atomic.Pointer[VisionObservation]
}

// NewObservationBuffer 创建环形缓冲区。capacity 为最大帧数。
func NewObservationBuffer(capacity int) *ObservationBuffer {
	if capacity <= 0 {
		capacity = 500
	}
	return &ObservationBuffer{
		frames:   make([]*VisionObservation, capacity),
		capacity: capacity,
		subs:     make(map[uint64]chan *VisionObservation),
	}
}

// Push 写入一帧（非阻塞）。
func (b *ObservationBuffer) Push(obs *VisionObservation) {
	b.mu.Lock()
	b.frames[b.head] = obs
	b.head = (b.head + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
	b.mu.Unlock()

	// 更新关键帧缓存
	if obs.IsKeyframe {
		b.latestKeyframe.Store(obs)
	}

	// 广播到订阅者（非阻塞）
	b.subMu.RLock()
	for _, ch := range b.subs {
		select {
		case ch <- obs:
		default:
			// 订阅者消费太慢，丢弃
		}
	}
	b.subMu.RUnlock()
}

// Last 返回最新 n 帧（按时间倒序）。
func (b *ObservationBuffer) Last(n int) []*VisionObservation {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if n > b.count {
		n = b.count
	}
	result := make([]*VisionObservation, n)
	for i := 0; i < n; i++ {
		idx := (b.head - 1 - i + b.capacity) % b.capacity
		result[i] = b.frames[idx]
	}
	return result
}

// Since 返回指定时间戳（Unix ms）之后的所有帧。
func (b *ObservationBuffer) Since(sinceMs int64) []*VisionObservation {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []*VisionObservation
	for i := 0; i < b.count; i++ {
		idx := (b.head - 1 - i + b.capacity) % b.capacity
		f := b.frames[idx]
		if f.CapturedAt <= sinceMs {
			break
		}
		result = append(result, f)
	}
	return result
}

// LatestKeyframe 返回最新关键帧（含 VLM 分析）。
func (b *ObservationBuffer) LatestKeyframe() *VisionObservation {
	return b.latestKeyframe.Load()
}

// Count 返回当前缓冲区中的帧数。
func (b *ObservationBuffer) Count() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Subscribe 订阅新帧通知。返回订阅 ID 和 channel。
func (b *ObservationBuffer) Subscribe(bufSize int) (uint64, <-chan *VisionObservation) {
	if bufSize <= 0 {
		bufSize = 16
	}
	ch := make(chan *VisionObservation, bufSize)

	b.subMu.Lock()
	b.nextSub++
	id := b.nextSub
	b.subs[id] = ch
	b.subMu.Unlock()

	return id, ch
}

// Unsubscribe 取消订阅。
func (b *ObservationBuffer) Unsubscribe(id uint64) {
	b.subMu.Lock()
	if ch, ok := b.subs[id]; ok {
		close(ch)
		delete(b.subs, id)
	}
	b.subMu.Unlock()
}
