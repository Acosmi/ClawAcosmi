//go:build !cgo

package vectoradapter

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
)

// SegmentStore is a pure Go fallback for the Rust segment vector store.
// It provides basic in-memory vector storage with brute-force cosine similarity search.
// Used when CGO is not available (e.g., CI, testing, CGO_ENABLED=0).
// No disk persistence — data is lost on process exit.
type SegmentStore struct {
	mu          sync.Mutex
	collections map[string]*pureCollection
}

type pureCollection struct {
	dim    int
	points map[string]*purePoint
}

type purePoint struct {
	vector  []float32
	payload map[string]interface{}
}

// segmentSearchHit represents a single search result from the segment store.
type segmentSearchHit struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// NewSegmentStore creates a new in-memory segment store (pure Go fallback).
func NewSegmentStore(_ string) (*SegmentStore, error) {
	return &SegmentStore{
		collections: make(map[string]*pureCollection),
	}, nil
}

// Close is a no-op for the pure Go fallback.
func (s *SegmentStore) Close() {}

// CreateCollection creates a new in-memory collection.
// Idempotent: returns nil if the collection already exists.
func (s *SegmentStore) CreateCollection(name string, dim int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.collections[name]; ok {
		return nil
	}
	s.collections[name] = &pureCollection{
		dim:    dim,
		points: make(map[string]*purePoint),
	}
	return nil
}

// Upsert inserts or updates a point in the specified collection.
func (s *SegmentStore) Upsert(collection, pointID string, denseVec []float32, payloadJSON []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	col, ok := s.collections[collection]
	if !ok {
		return fmt.Errorf("vectoradapter: collection not found: %s", collection)
	}
	if len(denseVec) != col.dim {
		return fmt.Errorf("vectoradapter: dimension mismatch: got %d, want %d", len(denseVec), col.dim)
	}

	var payload map[string]interface{}
	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return fmt.Errorf("vectoradapter: payload parse: %w", err)
		}
	}

	vec := make([]float32, len(denseVec))
	copy(vec, denseVec)

	col.points[pointID] = &purePoint{
		vector:  vec,
		payload: payload,
	}
	return nil
}

// Search performs a brute-force cosine similarity search.
func (s *SegmentStore) Search(collection string, queryVec []float32, limit int) ([]segmentSearchHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	col, ok := s.collections[collection]
	if !ok {
		return nil, fmt.Errorf("vectoradapter: collection not found: %s", collection)
	}

	type scored struct {
		id    string
		score float32
		pt    *purePoint
	}

	var results []scored
	for id, pt := range col.points {
		sim := cosine(queryVec, pt.vector)
		results = append(results, scored{id: id, score: sim, pt: pt})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	hits := make([]segmentSearchHit, len(results))
	for i, r := range results {
		hits[i] = segmentSearchHit{
			ID:      r.id,
			Score:   r.score,
			Payload: r.pt.payload,
		}
	}
	return hits, nil
}

// Delete removes a point from the collection.
func (s *SegmentStore) Delete(collection, pointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	col, ok := s.collections[collection]
	if !ok {
		return fmt.Errorf("vectoradapter: collection not found: %s", collection)
	}
	delete(col.points, pointID)
	return nil
}

// Flush is a no-op for the pure Go fallback.
func (s *SegmentStore) Flush(_ string) error {
	return nil
}

func cosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}
