//go:build !cgo

package ffi

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
)

// SegmentStore is a pure Go fallback for the Rust segment vector store.
// It provides basic in-memory vector storage without the Qdrant segment engine.
// This is used when CGo is not available (e.g., testing, CI).
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

// SegmentSearchHit represents a single search result from the segment store.
type SegmentSearchHit struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// NewSegmentStore creates a new in-memory segment store (pure Go fallback).
func NewSegmentStore(dataDir string) (*SegmentStore, error) {
	return &SegmentStore{
		collections: make(map[string]*pureCollection),
	}, nil
}

// Close is a no-op for the pure Go fallback.
func (s *SegmentStore) Close() {}

// CreateCollection creates a new in-memory collection.
func (s *SegmentStore) CreateCollection(name string, dim int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.collections[name]; ok {
		return nil // already exists
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
		return fmt.Errorf("collection not found: %s", collection)
	}

	var payload map[string]interface{}
	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return fmt.Errorf("payload parse: %w", err)
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
func (s *SegmentStore) Search(collection string, queryVec []float32, limit int) ([]SegmentSearchHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	col, ok := s.collections[collection]
	if !ok {
		return nil, fmt.Errorf("collection not found: %s", collection)
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

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	hits := make([]SegmentSearchHit, len(results))
	for i, r := range results {
		hits[i] = SegmentSearchHit{
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
		return fmt.Errorf("collection not found: %s", collection)
	}
	delete(col.points, pointID)
	return nil
}

// Flush is a no-op for the pure Go fallback.
func (s *SegmentStore) Flush(collection string) error {
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
