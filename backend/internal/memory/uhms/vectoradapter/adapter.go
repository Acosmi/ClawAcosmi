// Package vectoradapter provides uhms.VectorIndex and uhms.EmbeddingProvider
// implementations backed by Qdrant segment (FFI) or pure Go fallback.
package vectoradapter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/anthropic/open-acosmi/internal/memory/uhms"
)

// SystemCollections 系统级 Qdrant collections (非用户记忆)。
var SystemCollections = []string{
	"sys_skills",   // Phase 1: 技能检索
	"sys_plugins",  // Phase 2: 插件检索
	"sys_sessions", // Phase 3: 会话归档检索
}

// SegmentVectorIndex implements uhms.VectorIndex using Qdrant segment engine
// (in-process via Rust FFI, or pure Go fallback when CGO is unavailable).
type SegmentVectorIndex struct {
	store     *SegmentStore
	dimension int
}

// memoryCollections builds collection names from uhms.AllMemoryTypes.
func memoryCollections() []string {
	cols := make([]string, len(uhms.AllMemoryTypes))
	for i, mt := range uhms.AllMemoryTypes {
		cols[i] = "mem_" + string(mt)
	}
	return cols
}

// allCollections returns both memory and system collection names.
func allCollections() []string {
	mem := memoryCollections()
	all := make([]string, 0, len(mem)+len(SystemCollections))
	all = append(all, mem...)
	all = append(all, SystemCollections...)
	return all
}

// NewSegmentVectorIndex creates a new SegmentVectorIndex.
// dataDir is the directory for on-disk storage (CGO mode) or ignored (pure Go mode).
// dim is the expected embedding dimension; all Upsert/Search calls will be validated against it.
func NewSegmentVectorIndex(dataDir string, dim int) (*SegmentVectorIndex, error) {
	if dim <= 0 {
		return nil, fmt.Errorf("vectoradapter: invalid dimension %d", dim)
	}

	store, err := NewSegmentStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter: init segment store: %w", err)
	}

	// Create one collection per memory type + system collections (idempotent).
	cols := allCollections()
	for _, col := range cols {
		if err := store.CreateCollection(col, dim); err != nil {
			store.Close()
			return nil, fmt.Errorf("vectoradapter: create collection %q: %w", col, err)
		}
	}

	slog.Info("vectoradapter: segment vector index initialized",
		"dataDir", dataDir, "dimension", dim,
		"collections", len(cols))

	return &SegmentVectorIndex{store: store, dimension: dim}, nil
}

// Upsert adds or updates a vector in the specified collection.
// Returns an error if the vector dimension does not match the index dimension.
func (s *SegmentVectorIndex) Upsert(ctx context.Context, collection, id string, vector []float32, payload map[string]interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(vector) != s.dimension {
		return fmt.Errorf("vectoradapter: dimension mismatch: got %d, want %d", len(vector), s.dimension)
	}

	var payloadJSON []byte
	if len(payload) > 0 {
		var err error
		payloadJSON, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("vectoradapter: marshal payload: %w", err)
		}
	}

	return s.store.Upsert(collection, id, vector, payloadJSON)
}

// Search finds the top-k nearest vectors by cosine similarity.
// Returns an error if the query vector dimension does not match the index dimension.
func (s *SegmentVectorIndex) Search(ctx context.Context, collection string, query []float32, topK int) ([]uhms.VectorHit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(query) != s.dimension {
		return nil, fmt.Errorf("vectoradapter: dimension mismatch: got %d, want %d", len(query), s.dimension)
	}

	hits, err := s.store.Search(collection, query, topK)
	if err != nil {
		return nil, err
	}

	results := make([]uhms.VectorHit, len(hits))
	for i, h := range hits {
		results[i] = uhms.VectorHit{
			ID:    h.ID,
			Score: float64(h.Score),
		}
	}
	return results, nil
}

// Delete removes a vector from the specified collection.
func (s *SegmentVectorIndex) Delete(ctx context.Context, collection, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.store.Delete(collection, id)
}

// Close releases all segment store resources.
func (s *SegmentVectorIndex) Close() error {
	s.store.Close()
	return nil
}

// Dimension returns the configured vector dimension.
func (s *SegmentVectorIndex) Dimension() int {
	return s.dimension
}

// ============================================================================
// Payload-based operations for system collections (sys_skills, sys_plugins, sys_sessions)
// ============================================================================

// UpsertPayload adds or updates a payload-only entry (no embedding vector).
// Used for system collections (skills, plugins, sessions) where retrieval is by payload filter, not vector similarity.
func (s *SegmentVectorIndex) UpsertPayload(ctx context.Context, collection, id string, payload map[string]interface{}) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var payloadJSON []byte
	if len(payload) > 0 {
		var err error
		payloadJSON, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("vectoradapter: marshal payload: %w", err)
		}
	}
	// Use zero vector for payload-only entries
	zeroVec := make([]float32, s.dimension)
	return s.store.Upsert(collection, id, zeroVec, payloadJSON)
}

// SearchByPayload searches a collection by scanning all points and matching payload fields.
// This is used for system collections where we don't have embedding vectors.
// query is matched against name + description fields in payload.
func (s *SegmentVectorIndex) SearchByPayload(ctx context.Context, collection, query string, topK int) ([]uhms.PayloadHit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Use a zero vector search to get all points, then filter by payload
	zeroVec := make([]float32, s.dimension)
	// Request more results than topK to allow filtering
	limit := topK * 5
	if limit < 50 {
		limit = 50
	}
	hits, err := s.store.Search(collection, zeroVec, limit)
	if err != nil {
		return nil, err
	}

	queryLower := strings.ToLower(query)
	queryTerms := strings.Fields(queryLower)

	results := make([]uhms.PayloadHit, 0, topK)
	for _, h := range hits {
		payload := h.Payload
		if payload == nil {
			continue
		}
		// Score by matching query terms against name, description, tags
		score := payloadMatchScore(payload, queryTerms)
		if score > 0 {
			results = append(results, uhms.PayloadHit{
				ID:      h.ID,
				Payload: payload,
				Score:   score,
			})
		}
	}
	// Sort by score desc
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// payloadMatchScore scores how well payload fields match query terms.
func payloadMatchScore(payload map[string]interface{}, queryTerms []string) float64 {
	if len(queryTerms) == 0 {
		return 1.0 // empty query matches everything
	}

	// Build searchable text from key fields
	searchFields := []string{"name", "description", "tags", "category"}
	var searchText string
	for _, field := range searchFields {
		if v, ok := payload[field]; ok {
			searchText += " " + strings.ToLower(fmt.Sprintf("%v", v))
		}
	}

	if searchText == "" {
		return 0
	}

	matched := 0
	for _, term := range queryTerms {
		if strings.Contains(searchText, term) {
			matched++
		}
	}
	return float64(matched) / float64(len(queryTerms))
}
