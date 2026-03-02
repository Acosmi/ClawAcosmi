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

	"github.com/openacosmi/claw-acismi/internal/memory/uhms"
)

// SystemCollections 系统级 Qdrant collections (非用户记忆)。
var SystemCollections = []string{
	"sys_skills",   // Phase 1: 技能检索
	"sys_plugins",  // Phase 2: 插件检索
	"sys_sessions", // Phase 3: 会话归档检索
}

// CollectionProfile defines creation configuration for a collection.
type CollectionProfile struct {
	Dimension    int    // embedding dimension
	Distance     string // "Cosine" | "Euclid" | "Dot" | "Manhattan"
	UseHNSW      bool   // true = HNSW index, false = Plain brute-force
	HnswM        int    // HNSW m parameter, 0 = default 16
	HnswEfConst  int    // HNSW ef_construct, 0 = default 200
	Quantization string // "" | "scalar_int8" | "binary_1bit" (reserved for future use)
}

// DefaultMemoryProfile is the default for memory collections (HNSW enabled).
var DefaultMemoryProfile = CollectionProfile{
	Distance:    "Cosine",
	UseHNSW:     true,
	HnswM:       16,
	HnswEfConst: 200,
}

// DefaultSystemProfile is the default for system collections (Plain brute-force, small data).
var DefaultSystemProfile = CollectionProfile{
	Distance: "Cosine",
	UseHNSW:  false,
}

// buildConfigJSON constructs the JSON config for CreateCollectionV2.
func buildConfigJSON(p CollectionProfile) []byte {
	cfg := map[string]interface{}{
		"dimension":    p.Dimension,
		"distance":     p.Distance,
		"storage_type": "InRamChunkedMmap",
	}
	if p.UseHNSW {
		m := p.HnswM
		if m == 0 {
			m = 16
		}
		ef := p.HnswEfConst
		if ef == 0 {
			ef = 200
		}
		cfg["hnsw"] = map[string]interface{}{
			"m":                    m,
			"ef_construct":         ef,
			"full_scan_threshold":  10000,
			"max_indexing_threads": 0,
		}
	}
	// Quantization reserved for future use — only include if set.
	data, _ := json.Marshal(cfg)
	return data
}

// SegmentVectorIndex implements uhms.VectorIndex using Qdrant segment engine
// (in-process via Rust FFI, or pure Go fallback when CGO is unavailable).
type SegmentVectorIndex struct {
	store     *SegmentStore
	dimension int
}

// MemoryCollections builds collection names from uhms.AllMemoryTypes.
func MemoryCollections() []string {
	cols := make([]string, len(uhms.AllMemoryTypes))
	for i, mt := range uhms.AllMemoryTypes {
		cols[i] = "mem_" + string(mt)
	}
	return cols
}

// DefaultSearchDimension is the default dimension for payload-only search (system collections).
// Actual vector values are zero; this only affects storage layout, not search quality.
const DefaultSearchDimension = 768

// NewSearchEngine creates a SegmentVectorIndex for system collections only (skills, plugins, sessions).
// Always call this to ensure the core search engine is available regardless of vectorMode.
// Embedding-based memory collections are NOT created; use NewSegmentVectorIndex for full mode.
func NewSearchEngine(dataDir string) (*SegmentVectorIndex, error) {
	store, err := NewSegmentStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter: init segment store: %w", err)
	}

	for _, col := range SystemCollections {
		if err := store.CreateCollection(col, DefaultSearchDimension); err != nil {
			store.Close()
			return nil, fmt.Errorf("vectoradapter: create collection %q: %w", col, err)
		}
	}

	slog.Info("vectoradapter: search engine initialized (system collections only)",
		"dataDir", dataDir, "dimension", DefaultSearchDimension,
		"collections", len(SystemCollections))

	return &SegmentVectorIndex{store: store, dimension: DefaultSearchDimension}, nil
}

// NewSegmentVectorIndex creates a new SegmentVectorIndex with ALL collections (memory + system).
// dataDir is the directory for on-disk storage (CGO mode) or ignored (pure Go mode).
// dim is the expected embedding dimension; all Upsert/Search calls will be validated against it.
// Use this when vectorMode is enabled and an embedding provider is available.
//
// Memory collections use HNSW index for O(log n) approximate search.
// System collections use Plain index (brute-force) — data volume is small enough.
func NewSegmentVectorIndex(dataDir string, dim int) (*SegmentVectorIndex, error) {
	if dim <= 0 {
		return nil, fmt.Errorf("vectoradapter: invalid dimension %d", dim)
	}

	store, err := NewSegmentStore(dataDir)
	if err != nil {
		return nil, fmt.Errorf("vectoradapter: init segment store: %w", err)
	}

	// Memory collections: HNSW index for scalable search.
	memCols := MemoryCollections()
	for _, col := range memCols {
		profile := DefaultMemoryProfile
		profile.Dimension = dim
		configJSON := buildConfigJSON(profile)
		if err := store.CreateCollectionV2(col, configJSON); err != nil {
			store.Close()
			return nil, fmt.Errorf("vectoradapter: create memory collection %q: %w", col, err)
		}
	}

	// System collections: Plain index (scroll + keyword, small data volume).
	for _, col := range SystemCollections {
		if err := store.CreateCollection(col, dim); err != nil {
			store.Close()
			return nil, fmt.Errorf("vectoradapter: create system collection %q: %w", col, err)
		}
	}

	totalCols := len(memCols) + len(SystemCollections)
	slog.Info("vectoradapter: segment vector index initialized",
		"dataDir", dataDir, "dimension", dim,
		"memoryCollections", len(memCols), "systemCollections", len(SystemCollections),
		"indexMode", "Plain (HNSW optimization pending)",
		"collections", totalCols)

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

// SearchWithParams finds the top-k nearest vectors with configurable HNSW ef parameter.
// hnswEf controls search accuracy: higher = more accurate but slower. 0 = default.
func (s *SegmentVectorIndex) SearchWithParams(ctx context.Context, collection string, query []float32, topK int, hnswEf int) ([]uhms.VectorHit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(query) != s.dimension {
		return nil, fmt.Errorf("vectoradapter: dimension mismatch: got %d, want %d", len(query), s.dimension)
	}

	var paramsJSON []byte
	if hnswEf > 0 {
		paramsJSON, _ = json.Marshal(map[string]interface{}{
			"hnsw_ef": hnswEf,
		})
	}

	hits, err := s.store.SearchV2(collection, query, topK, paramsJSON)
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
// Segment optimization (HNSW index build)
// ============================================================================

// Optimize triggers HNSW index build for a single collection.
// Returns (true, nil) if optimized, (false, nil) if skipped (no HNSW config, empty, or system collection).
func (s *SegmentVectorIndex) Optimize(ctx context.Context, collection string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return s.store.OptimizeCollection(collection)
}

// OptimizeMemoryCollections optimizes all memory collections (mem_*) by building HNSW indexes.
// Returns the number of collections optimized and their names.
//
// NOTE: After optimization, a collection's segment becomes non-appendable (immutable HNSW index).
// Subsequent Upsert calls to an optimized collection will fail. This is by design for the
// current usage pattern: batch write → optimize → read-only search.
func (s *SegmentVectorIndex) OptimizeMemoryCollections(ctx context.Context) (int, []string, error) {
	optimized := 0
	var optimizedNames []string
	for _, col := range MemoryCollections() {
		if err := ctx.Err(); err != nil {
			return optimized, optimizedNames, err
		}
		ok, err := s.store.OptimizeCollection(col)
		if err != nil {
			return optimized, optimizedNames, fmt.Errorf("optimize %q: %w", col, err)
		}
		if ok {
			optimized++
			optimizedNames = append(optimizedNames, col)
		}
	}
	if optimized > 0 {
		slog.Info("vectoradapter: HNSW optimization complete",
			"optimized", optimized, "collections", optimizedNames)
	}
	return optimized, optimizedNames, nil
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

// SearchByPayload searches a collection by scrolling all points and matching payload fields.
// Uses scroll API (no vector similarity) to avoid the zero-vector cosine(0,0)=NaN problem.
// query is matched against name + description fields in payload.
func (s *SegmentVectorIndex) SearchByPayload(ctx context.Context, collection, query string, topK int) ([]uhms.PayloadHit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Scroll more points than topK to allow keyword filtering to select the best matches.
	scrollLimit := topK * 10
	if scrollLimit < 200 {
		scrollLimit = 200
	}

	hits, err := s.store.Scroll(collection, scrollLimit)
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
		score := payloadMatchScore(payload, queryTerms)
		if score > 0 {
			results = append(results, uhms.PayloadHit{
				ID:      h.ID,
				Payload: payload,
				Score:   score,
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// PointCount returns the number of available points in a collection.
func (s *SegmentVectorIndex) PointCount(collection string) (int, error) {
	return s.store.PointCount(collection)
}

// payloadMatchScore scores how well payload fields match query terms.
func payloadMatchScore(payload map[string]interface{}, queryTerms []string) float64 {
	if len(queryTerms) == 0 {
		return 1.0 // empty query matches everything
	}

	// Build searchable text from key fields
	searchFields := []string{"name", "description", "tags", "category", "l0_abstract"}
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
