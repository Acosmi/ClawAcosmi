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

// CreateCollectionV2 creates a collection from JSON config.
// Pure Go fallback: parses dimension from JSON, ignores HNSW and other advanced settings.
func (s *SegmentStore) CreateCollectionV2(name string, configJSON []byte) error {
	var cfg struct {
		Dimension int `json:"dimension"`
	}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return fmt.Errorf("vectoradapter: parse config JSON: %w", err)
	}
	if cfg.Dimension <= 0 {
		return fmt.Errorf("vectoradapter: invalid dimension %d in config JSON", cfg.Dimension)
	}
	return s.CreateCollection(name, cfg.Dimension)
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

// SearchV2 performs a brute-force search, ignoring search params (pure Go has no HNSW).
func (s *SegmentStore) SearchV2(collection string, queryVec []float32, limit int, _ []byte) ([]segmentSearchHit, error) {
	return s.Search(collection, queryVec, limit)
}

// scrollHit represents a scroll result (no score, just id + payload).
type scrollHit struct {
	ID      string                 `json:"id"`
	Payload map[string]interface{} `json:"payload"`
}

// Scroll enumerates up to limit points in a collection (no vector similarity).
func (s *SegmentStore) Scroll(collection string, limit int) ([]scrollHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	col, ok := s.collections[collection]
	if !ok {
		return nil, fmt.Errorf("vectoradapter: collection not found: %s", collection)
	}

	hits := make([]scrollHit, 0, min(limit, len(col.points)))
	for id, pt := range col.points {
		if len(hits) >= limit {
			break
		}
		hits = append(hits, scrollHit{
			ID:      id,
			Payload: pt.payload,
		})
	}
	return hits, nil
}

// PointCount returns the number of available points in a collection.
func (s *SegmentStore) PointCount(collection string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	col, ok := s.collections[collection]
	if !ok {
		return 0, fmt.Errorf("vectoradapter: collection not found: %s", collection)
	}
	return len(col.points), nil
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

// OptimizeCollection is a no-op for the pure Go fallback (no HNSW support).
func (s *SegmentStore) OptimizeCollection(_ string) (bool, error) {
	return false, nil
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
