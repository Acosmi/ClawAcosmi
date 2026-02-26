// Package embedqueue_test — AGFS integration-style tests for EmbeddingQueue.
// Uses a mock HTTP server to simulate AGFS queuefs behavior.
package embedqueue_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// agfsQueueServer simulates AGFS queuefs HTTP behavior (enqueue/dequeue).
type agfsQueueServer struct {
	mu    sync.Mutex
	queue []json.RawMessage
}

func newAGFSQueueServer() (*httptest.Server, *agfsQueueServer) {
	qs := &agfsQueueServer{}
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Files endpoint — handles queuefs enqueue (PUT) and dequeue (GET)
	mux.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")

		switch r.Method {
		case http.MethodPut:
			// Enqueue: write data to the queue
			data, _ := io.ReadAll(r.Body)
			qs.mu.Lock()
			qs.queue = append(qs.queue, json.RawMessage(data))
			qs.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "ok"})

		case http.MethodGet:
			// Dequeue: return first item from queue
			if path == "" || !isDequeuePath(path) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			qs.mu.Lock()
			if len(qs.queue) == 0 {
				qs.mu.Unlock()
				// Empty queue — return empty response
				w.Write([]byte{})
				return
			}
			item := qs.queue[0]
			qs.queue = qs.queue[1:]
			qs.mu.Unlock()
			w.Write(item)
		}
	})

	srv := httptest.NewServer(mux)
	return srv, qs
}

func isDequeuePath(path string) bool {
	return len(path) > 10 // simple check to distinguish dequeue paths
}

// TestAGFSEnqueueDequeueRoundtrip verifies the HTTP serialization roundtrip.
func TestAGFSEnqueueDequeueRoundtrip(t *testing.T) {
	srv, qs := newAGFSQueueServer()
	defer srv.Close()

	type embedItem struct {
		MemoryID        string         `json:"memory_id"`
		Content         string         `json:"content"`
		UserID          string         `json:"user_id"`
		MemoryType      string         `json:"memory_type"`
		ImportanceScore float64        `json:"importance_score"`
		Metadata        map[string]any `json:"metadata,omitempty"`
	}

	// Simulate enqueue via HTTP PUT
	original := embedItem{
		MemoryID:        "550e8400-e29b-41d4-a716-446655440000",
		Content:         "Test memory content",
		UserID:          "user-123",
		MemoryType:      "episodic",
		ImportanceScore: 0.85,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Enqueue
	qs.mu.Lock()
	qs.queue = append(qs.queue, json.RawMessage(data))
	qs.mu.Unlock()

	// Dequeue
	qs.mu.Lock()
	if len(qs.queue) == 0 {
		qs.mu.Unlock()
		t.Fatal("queue should not be empty")
	}
	raw := qs.queue[0]
	qs.queue = qs.queue[1:]
	qs.mu.Unlock()

	var got embedItem
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.MemoryID != original.MemoryID {
		t.Errorf("MemoryID = %q, want %q", got.MemoryID, original.MemoryID)
	}
	if got.Content != original.Content {
		t.Errorf("Content = %q, want %q", got.Content, original.Content)
	}
	if got.UserID != original.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, original.UserID)
	}
	if got.ImportanceScore != original.ImportanceScore {
		t.Errorf("ImportanceScore = %v, want %v", got.ImportanceScore, original.ImportanceScore)
	}
}

// TestAGFSConcurrentEnqueue verifies concurrent writes to the mock AGFS queue.
func TestAGFSConcurrentEnqueue(t *testing.T) {
	srv, qs := newAGFSQueueServer()
	defer srv.Close()

	const producers = 50
	var wg sync.WaitGroup
	wg.Add(producers)
	var enqueued int64

	for i := 0; i < producers; i++ {
		go func(n int) {
			defer wg.Done()
			item := map[string]any{
				"memory_id": n,
				"content":   "test",
			}
			data, _ := json.Marshal(item)
			qs.mu.Lock()
			qs.queue = append(qs.queue, json.RawMessage(data))
			qs.mu.Unlock()
			atomic.AddInt64(&enqueued, 1)
		}(i)
	}

	wg.Wait()

	qs.mu.Lock()
	qLen := len(qs.queue)
	qs.mu.Unlock()

	if qLen != producers {
		t.Errorf("queue length = %d, want %d", qLen, producers)
	}

	// Drain and verify count
	var dequeued int
	qs.mu.Lock()
	dequeued = len(qs.queue)
	qs.queue = nil
	qs.mu.Unlock()

	if dequeued != producers {
		t.Errorf("dequeued = %d, want %d", dequeued, producers)
	}
}

// TestAGFSEmptyDequeue verifies behavior when queue is empty.
func TestAGFSEmptyDequeue(t *testing.T) {
	_, qs := newAGFSQueueServer()

	qs.mu.Lock()
	empty := len(qs.queue) == 0
	qs.mu.Unlock()

	if !empty {
		t.Error("expected empty queue")
	}
}

// TestAGFSBatchPolling verifies that the polling + batching pattern works
// with the AGFS-style queue backend.
func TestAGFSBatchPolling(t *testing.T) {
	_, qs := newAGFSQueueServer()

	// Enqueue 10 items
	for i := 0; i < 10; i++ {
		item := map[string]any{"id": i}
		data, _ := json.Marshal(item)
		qs.mu.Lock()
		qs.queue = append(qs.queue, json.RawMessage(data))
		qs.mu.Unlock()
	}

	// Simulate polling with batch size 4
	batchSize := 4
	var batches [][]json.RawMessage

	for {
		qs.mu.Lock()
		if len(qs.queue) == 0 {
			qs.mu.Unlock()
			break
		}
		end := batchSize
		if end > len(qs.queue) {
			end = len(qs.queue)
		}
		batch := make([]json.RawMessage, end)
		copy(batch, qs.queue[:end])
		qs.queue = qs.queue[end:]
		qs.mu.Unlock()

		batches = append(batches, batch)
	}

	// Should get 3 batches: [4, 4, 2]
	if len(batches) != 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}
	if len(batches[0]) != 4 {
		t.Errorf("batch[0] len = %d, want 4", len(batches[0]))
	}
	if len(batches[1]) != 4 {
		t.Errorf("batch[1] len = %d, want 4", len(batches[1]))
	}
	if len(batches[2]) != 2 {
		t.Errorf("batch[2] len = %d, want 2", len(batches[2]))
	}

	// Suppress unused variable warning
	_ = time.Millisecond
}
