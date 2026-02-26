// Package embedqueue_test tests the EmbeddingQueue logic in isolation.
// Separated from services package to avoid CGO/FFI linkage.
//
// Phase A: Updated to reflect AGFS queuefs polling architecture.
// Uses an in-memory queue to simulate AGFS queuefs enqueue/dequeue behavior.
package embedqueue_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// embedItem mirrors services.EmbedItem for testing.
type embedItem struct {
	ID      string
	Content string
}

// agfsMockQueue simulates the AGFS queuefs distributed queue in-memory.
// Thread-safe: multiple goroutines can enqueue/dequeue concurrently.
type agfsMockQueue struct {
	mu    sync.Mutex
	items []embedItem
}

func (q *agfsMockQueue) enqueue(item embedItem) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, item)
}

func (q *agfsMockQueue) dequeue() (embedItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return embedItem{}, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

// mockPollingQueue replicates the EmbeddingQueue AGFS polling + batching logic.
type mockPollingQueue struct {
	backend       *agfsMockQueue
	batchSize     int
	flushInterval time.Duration
	pollInterval  time.Duration
	wg            sync.WaitGroup
	stop          chan struct{}

	// Test hooks.
	flushed   [][]embedItem
	flushMu   sync.Mutex
	processed int64
}

func newMockPollingQueue(batchSize int, flushInterval, pollInterval time.Duration) *mockPollingQueue {
	q := &mockPollingQueue{
		backend:       &agfsMockQueue{},
		batchSize:     batchSize,
		flushInterval: flushInterval,
		pollInterval:  pollInterval,
		stop:          make(chan struct{}),
	}
	q.wg.Add(1)
	go q.workerLoop()
	return q
}

func (q *mockPollingQueue) enqueue(item embedItem) {
	q.backend.enqueue(item)
}

func (q *mockPollingQueue) close() {
	close(q.stop)
	q.wg.Wait()
}

// workerLoop mirrors the AGFS polling-based EmbeddingQueue.workerLoop.
func (q *mockPollingQueue) workerLoop() {
	defer q.wg.Done()

	flushTicker := time.NewTicker(q.flushInterval)
	defer flushTicker.Stop()

	pollTicker := time.NewTicker(q.pollInterval)
	defer pollTicker.Stop()

	batch := make([]embedItem, 0, q.batchSize)

	for {
		select {
		case <-pollTicker.C:
			item, ok := q.backend.dequeue()
			if ok {
				batch = append(batch, item)
				for len(batch) < q.batchSize {
					extra, extraOk := q.backend.dequeue()
					if !extraOk {
						break
					}
					batch = append(batch, extra)
				}
			}
			if len(batch) >= q.batchSize {
				q.flush(batch)
				batch = make([]embedItem, 0, q.batchSize)
			}

		case <-flushTicker.C:
			if len(batch) > 0 {
				q.flush(batch)
				batch = make([]embedItem, 0, q.batchSize)
			}

		case <-q.stop:
			for {
				item, ok := q.backend.dequeue()
				if !ok {
					break
				}
				batch = append(batch, item)
			}
			if len(batch) > 0 {
				q.flush(batch)
			}
			return
		}
	}
}

func (q *mockPollingQueue) flush(batch []embedItem) {
	q.flushMu.Lock()
	defer q.flushMu.Unlock()
	cp := make([]embedItem, len(batch))
	copy(cp, batch)
	q.flushed = append(q.flushed, cp)
	atomic.AddInt64(&q.processed, int64(len(batch)))
}

func (q *mockPollingQueue) getFlushed() [][]embedItem {
	q.flushMu.Lock()
	defer q.flushMu.Unlock()
	cp := make([][]embedItem, len(q.flushed))
	copy(cp, q.flushed)
	return cp
}

// TestBatchFlushOnFullBatch — batch is flushed when batchSize is reached.
func TestBatchFlushOnFullBatch(t *testing.T) {
	q := newMockPollingQueue(4, 10*time.Second, 10*time.Millisecond)

	for i := 0; i < 4; i++ {
		q.enqueue(embedItem{ID: string(rune('A' + i))})
	}

	// Wait for poll + flush.
	time.Sleep(100 * time.Millisecond)
	q.close()

	if p := atomic.LoadInt64(&q.processed); p != 4 {
		t.Errorf("expected 4 processed, got %d", p)
	}
	batches := q.getFlushed()
	if len(batches) == 0 {
		t.Fatal("expected at least one flush")
	}
	if len(batches[0]) != 4 {
		t.Errorf("first batch should have 4 items, got %d", len(batches[0]))
	}
}

// TestTimerFlush — items are flushed by timer even if batch is not full.
func TestTimerFlush(t *testing.T) {
	q := newMockPollingQueue(100, 50*time.Millisecond, 10*time.Millisecond)

	q.enqueue(embedItem{ID: "X"})
	q.enqueue(embedItem{ID: "Y"})

	// Wait for poll to pick up + flush timer to fire.
	time.Sleep(200 * time.Millisecond)
	q.close()

	if p := atomic.LoadInt64(&q.processed); p != 2 {
		t.Errorf("expected 2 processed, got %d", p)
	}
}

// TestCloseFlushesRemaining — remaining items are drained on Close.
func TestCloseFlushesRemaining(t *testing.T) {
	q := newMockPollingQueue(100, 10*time.Second, 10*time.Second)

	for i := 0; i < 7; i++ {
		q.enqueue(embedItem{ID: string(rune('0' + i))})
	}

	// Close immediately — should drain the 7 items from AGFS backend.
	q.close()

	if p := atomic.LoadInt64(&q.processed); p != 7 {
		t.Errorf("expected 7 processed after close, got %d", p)
	}
}

// TestConcurrentEnqueue — many goroutines enqueue without data loss.
func TestConcurrentEnqueue(t *testing.T) {
	q := newMockPollingQueue(32, 50*time.Millisecond, 10*time.Millisecond)

	const producers = 100
	var wg sync.WaitGroup
	wg.Add(producers)

	for i := 0; i < producers; i++ {
		go func(n int) {
			defer wg.Done()
			q.enqueue(embedItem{ID: string(rune(n))})
		}(i)
	}

	wg.Wait()
	q.close()

	if p := atomic.LoadInt64(&q.processed); p != producers {
		t.Errorf("expected %d processed, got %d", producers, p)
	}
}
