// Package services — EmbeddingQueue: async batched embedding pipeline.
//
// Phase A migration: bottom-layer storage moved from Go-native in-memory
// channel to AGFS queuefs (distributed, DB-backed). The workerLoop batch
// logic (batch embedding + segment upsert) is **entirely preserved**.
//
// Architecture:
//
//	Enqueue  → AGFSClient.QueueEnqueueJSON("/queuefs/embedding_tasks/enqueue")
//	workerLoop → polling AGFSClient.QueueDequeue → batch → flushBatch (unchanged)
//
// Benefits:
//  1. Multi-instance: all Go API replicas share the same queue
//  2. Persistence: AGFS queuefs uses DB backend — data survives restarts
//  3. Decoupled producers/consumers: any instance can enqueue, any can process
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/uhms/go-api/internal/agfs"
	"github.com/uhms/go-api/internal/ffi"
)

// queueNameEmbedding is the AGFS queuefs queue name for embedding tasks.
const queueNameEmbedding = "embedding_tasks"

// EmbedItem represents a memory pending embedding + upsert.
// Serialized as JSON for AGFS queuefs transport.
type EmbedItem struct {
	MemoryID        uuid.UUID      `json:"memory_id"`
	Content         string         `json:"content"`
	UserID          string         `json:"user_id"`
	MemoryType      string         `json:"memory_type"`
	ImportanceScore float64        `json:"importance_score"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// EmbeddingQueue manages async batched embedding and segment upsert.
// Phase A: uses AGFS queuefs as distributed backing store.
type EmbeddingQueue struct {
	agfsClient    *agfs.AGFSClient
	batchSize     int
	flushInterval time.Duration
	pollInterval  time.Duration // interval between dequeue attempts
	wg            sync.WaitGroup
	stop          chan struct{}

	// Dependencies injected from VectorStoreService.
	embeddingSvc EmbeddingService
	store        *ffi.SegmentStore

	// Observability counters (atomic).
	enqueued        int64
	processed       int64
	errors          int64
	consecutiveErrs int64 // suppress dequeue log spam when AGFS is unavailable
}

// EmbeddingQueueConfig holds tuning parameters for the queue.
type EmbeddingQueueConfig struct {
	BatchSize     int           // max items per flush (default: 32)
	FlushInterval time.Duration // flush period even if batch not full (default: 500ms)
	PollInterval  time.Duration // dequeue polling interval (default: 100ms)
}

// DefaultEmbeddingQueueConfig returns sensible defaults.
func DefaultEmbeddingQueueConfig() EmbeddingQueueConfig {
	return EmbeddingQueueConfig{
		BatchSize:     32,
		FlushInterval: 500 * time.Millisecond,
		PollInterval:  100 * time.Millisecond,
	}
}

// NewEmbeddingQueue creates and starts a new queue with one background worker.
// The worker polls AGFS queuefs for items, batches them, and flushes.
func NewEmbeddingQueue(
	cfg EmbeddingQueueConfig,
	embeddingSvc EmbeddingService,
	store *ffi.SegmentStore,
	agfsClient *agfs.AGFSClient,
) *EmbeddingQueue {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 32
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 500 * time.Millisecond
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}

	q := &EmbeddingQueue{
		agfsClient:    agfsClient,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		pollInterval:  cfg.PollInterval,
		stop:          make(chan struct{}),
		embeddingSvc:  embeddingSvc,
		store:         store,
	}

	q.wg.Add(1)
	go q.workerLoop()

	slog.Info("EmbeddingQueue started (AGFS queuefs backend)",
		"queue", queueNameEmbedding,
		"batch_size", cfg.BatchSize,
		"flush_interval", cfg.FlushInterval,
		"poll_interval", cfg.PollInterval,
	)
	return q
}

// Enqueue adds an item to the AGFS distributed queue.
// Returns error if serialization or AGFS call fails.
func (q *EmbeddingQueue) Enqueue(item EmbedItem) error {
	select {
	case <-q.stop:
		return fmt.Errorf("embedding queue is closed")
	default:
	}

	if err := q.agfsClient.QueueEnqueueJSON(queueNameEmbedding, item); err != nil {
		atomic.AddInt64(&q.errors, 1)
		return fmt.Errorf("agfs enqueue: %w", err)
	}
	atomic.AddInt64(&q.enqueued, 1)
	return nil
}

// Close signals the worker to stop, drains remaining items, and waits.
func (q *EmbeddingQueue) Close() {
	close(q.stop)
	q.wg.Wait()
	slog.Info("EmbeddingQueue closed",
		"enqueued", atomic.LoadInt64(&q.enqueued),
		"processed", atomic.LoadInt64(&q.processed),
		"errors", atomic.LoadInt64(&q.errors),
	)
}

// Stats returns current queue counters.
func (q *EmbeddingQueue) Stats() (enqueued, processed, errors int64) {
	return atomic.LoadInt64(&q.enqueued),
		atomic.LoadInt64(&q.processed),
		atomic.LoadInt64(&q.errors)
}

// workerLoop is the background goroutine that polls AGFS, batches, and flushes.
// Replaces the channel-based select loop with AGFS queuefs polling.
func (q *EmbeddingQueue) workerLoop() {
	defer q.wg.Done()

	flushTicker := time.NewTicker(q.flushInterval)
	defer flushTicker.Stop()

	pollTicker := time.NewTicker(q.pollInterval)
	defer pollTicker.Stop()

	batch := make([]EmbedItem, 0, q.batchSize)

	for {
		select {
		case <-pollTicker.C:
			// Poll AGFS queue for available items.
			item, ok := q.dequeueOne()
			if ok {
				batch = append(batch, item)
				// Greedily drain more items up to batch size.
				for len(batch) < q.batchSize {
					extra, extraOk := q.dequeueOne()
					if !extraOk {
						break
					}
					batch = append(batch, extra)
				}
			}
			if len(batch) >= q.batchSize {
				q.flushBatch(batch)
				batch = batch[:0]
			}

		case <-flushTicker.C:
			if len(batch) > 0 {
				q.flushBatch(batch)
				batch = batch[:0]
			}

		case <-q.stop:
			// Drain remaining items from AGFS queue before exiting.
			for {
				item, ok := q.dequeueOne()
				if !ok {
					break
				}
				batch = append(batch, item)
			}
			if len(batch) > 0 {
				q.flushBatch(batch)
			}
			return
		}
	}
}

// dequeueOne attempts to dequeue a single item from AGFS queuefs.
// Returns (item, true) on success, or (zero, false) if queue is empty or error.
func (q *EmbeddingQueue) dequeueOne() (EmbedItem, bool) {
	data, err := q.agfsClient.QueueDequeue(queueNameEmbedding)
	if err != nil {
		// Suppress log spam: only log every 300 consecutive failures (~30s at 100ms poll).
		n := atomic.AddInt64(&q.consecutiveErrs, 1)
		if n == 1 || n%300 == 0 {
			slog.Warn("AGFS dequeue unavailable (will retry silently)", "error", err, "consecutive", n)
		}
		return EmbedItem{}, false
	}
	if len(data) == 0 {
		return EmbedItem{}, false
	}
	// Reset consecutive error counter on success.
	atomic.StoreInt64(&q.consecutiveErrs, 0)

	var item EmbedItem
	if err := json.Unmarshal(data, &item); err != nil {
		atomic.AddInt64(&q.errors, 1)
		slog.Error("Failed to unmarshal dequeued item", "error", err, "raw", string(data))
		return EmbedItem{}, false
	}
	return item, true
}

// flushBatch performs batch embedding + per-item segment upsert.
// This method is UNCHANGED from the pre-AGFS implementation.
func (q *EmbeddingQueue) flushBatch(batch []EmbedItem) {
	if len(batch) == 0 {
		return
	}

	ctx := context.Background()

	// 1. Batch dense embedding.
	texts := make([]string, len(batch))
	for i, item := range batch {
		texts[i] = item.Content
	}

	embeddings, err := q.embeddingSvc.EmbedDocuments(ctx, texts)
	if err != nil {
		atomic.AddInt64(&q.errors, int64(len(batch)))
		slog.Error("Batch embedding failed", "count", len(batch), "error", err)
		return
	}

	if len(embeddings) != len(batch) {
		atomic.AddInt64(&q.errors, int64(len(batch)))
		slog.Error("Embedding count mismatch", "expected", len(batch), "got", len(embeddings))
		return
	}

	// 2. Upsert each item via segment store.
	for i, item := range batch {
		payload := map[string]any{
			"content":          item.Content,
			"user_id":          item.UserID,
			"memory_type":      item.MemoryType,
			"importance_score": item.ImportanceScore,
		}
		for k, val := range item.Metadata {
			payload[k] = val
		}
		payloadJSON, jerr := json.Marshal(payload)
		if jerr != nil {
			atomic.AddInt64(&q.errors, 1)
			slog.Error("Payload marshal failed", "memory_id", item.MemoryID, "error", jerr)
			continue
		}

		col := collectionForType(item.MemoryType)
		if uerr := q.store.Upsert(col, item.MemoryID.String(), embeddings[i], payloadJSON); uerr != nil {
			atomic.AddInt64(&q.errors, 1)
			slog.Error("Segment upsert failed", "collection", col, "memory_id", item.MemoryID, "error", uerr)
			continue
		}
		atomic.AddInt64(&q.processed, 1)
	}

	slog.Debug("Flushed embedding batch", "count", len(batch), "first_id", batch[0].MemoryID)
}
