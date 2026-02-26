// Package hybrid_test tests the VFS semantic index + hybrid fusion pipeline.
// Separated from services package to avoid CGO/FFI linkage.
// Replicates core VFSSemanticIndex + fuseSearchResults logic for isolated testing.
package hybrid_test

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"
	"testing"
)

// --- Replicate core types and logic from services/ and ffi/ packages ---

// segmentSearchHit mirrors ffi.SegmentSearchHit.
type segmentSearchHit struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// memFSSearchHit mirrors ffi.MemFSSearchHit.
type memFSSearchHit struct {
	Path       string  `json:"path"`
	MemoryID   string  `json:"memory_id"`
	Category   string  `json:"category"`
	Score      float64 `json:"score"`
	L0Abstract string  `json:"l0_abstract"`
}

// vectorSearchResult mirrors services.VectorSearchResult.
type vectorSearchResult struct {
	MemoryID   string  `json:"memory_id"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
	MemoryType string  `json:"memory_type"`
	Category   string  `json:"category"`
}

// --- In-memory segment store (mirrors ffi/pure_segment.go) ---

type pureSegmentStore struct {
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

func newPureSegmentStore() *pureSegmentStore {
	return &pureSegmentStore{
		collections: make(map[string]*pureCollection),
	}
}

func (s *pureSegmentStore) createCollection(name string, dim int) error {
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

func (s *pureSegmentStore) upsert(collection, pointID string, vec []float32, payloadJSON []byte) error {
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

	v := make([]float32, len(vec))
	copy(v, vec)
	col.points[pointID] = &purePoint{vector: v, payload: payload}
	return nil
}

func (s *pureSegmentStore) search(collection string, queryVec []float32, limit int) ([]segmentSearchHit, error) {
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

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	hits := make([]segmentSearchHit, len(results))
	for i, r := range results {
		hits[i] = segmentSearchHit{ID: r.id, Score: r.score, Payload: r.pt.payload}
	}
	return hits, nil
}

func (s *pureSegmentStore) delete(collection, pointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	col, ok := s.collections[collection]
	if !ok {
		return fmt.Errorf("collection not found: %s", collection)
	}
	delete(col.points, pointID)
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

// payloadStr extracts a string from a map payload.
func payloadStr(payload map[string]interface{}, key string) string {
	if v, ok := payload[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// --- VFS Semantic Index (mirrors services/vfs_semantic_index.go) ---

const vfsSemanticCollection = "vfs_semantic"

type vfsSemanticIndex struct {
	store *pureSegmentStore
}

func newVFSSemanticIndex(store *pureSegmentStore) *vfsSemanticIndex {
	return &vfsSemanticIndex{store: store}
}

func (idx *vfsSemanticIndex) ensureCollection(dim int) error {
	return idx.store.createCollection(vfsSemanticCollection, dim)
}

func (idx *vfsSemanticIndex) indexContent(
	memoryID, tenantID, userID, section, category, content string,
	embedding []float32,
) error {
	vfsURI := fmt.Sprintf("%s/%s/%s", section, category, memoryID)
	payload := map[string]any{
		"content":   content,
		"tenant_id": tenantID,
		"user_id":   userID,
		"section":   section,
		"category":  category,
		"vfs_uri":   vfsURI,
		"memory_id": memoryID,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return idx.store.upsert(vfsSemanticCollection, memoryID, embedding, payloadJSON)
}

func (idx *vfsSemanticIndex) removeIndex(memoryID string) error {
	return idx.store.delete(vfsSemanticCollection, memoryID)
}

func (idx *vfsSemanticIndex) semanticSearchToFSHits(queryVec []float32, limit int) ([]memFSSearchHit, error) {
	hits, err := idx.store.search(vfsSemanticCollection, queryVec, limit)
	if err != nil {
		return nil, err
	}
	fsHits := make([]memFSSearchHit, 0, len(hits))
	for _, h := range hits {
		fsHits = append(fsHits, memFSSearchHit{
			MemoryID:   h.ID,
			Score:      float64(h.Score),
			Path:       payloadStr(h.Payload, "vfs_uri"),
			L0Abstract: payloadStr(h.Payload, "content"),
			Category:   payloadStr(h.Payload, "category"),
		})
	}
	return fsHits, nil
}

// --- RRF Fusion (mirrors services/memory_manager.go fuseSearchResults) ---

func fuseSearchResults(
	vectorResults []vectorSearchResult,
	fsHits []memFSSearchHit,
	limit int,
) []vectorSearchResult {
	const k = 60.0

	type fusedItem struct {
		result   vectorSearchResult
		rrfScore float64
	}

	scoreMap := make(map[string]*fusedItem)

	for i, r := range vectorResults {
		scoreMap[r.MemoryID] = &fusedItem{
			result:   r,
			rrfScore: 1.0 / (k + float64(i+1)),
		}
	}

	for i, hit := range fsHits {
		rrfContrib := 1.0 / (k + float64(i+1))
		if item, ok := scoreMap[hit.MemoryID]; ok {
			item.rrfScore += rrfContrib
		} else {
			scoreMap[hit.MemoryID] = &fusedItem{
				result: vectorSearchResult{
					MemoryID: hit.MemoryID,
					Content:  hit.L0Abstract,
					Score:    hit.Score,
					Category: hit.Category,
				},
				rrfScore: rrfContrib,
			}
		}
	}

	items := make([]fusedItem, 0, len(scoreMap))
	for _, item := range scoreMap {
		items = append(items, *item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].rrfScore > items[j].rrfScore
	})

	if len(items) > limit {
		items = items[:limit]
	}

	results := make([]vectorSearchResult, len(items))
	for i, item := range items {
		results[i] = item.result
	}
	return results
}

// --- Tests ---

// TestVFSSemanticIndex_IndexAndSearch tests the core index→search cycle.
func TestVFSSemanticIndex_IndexAndSearch(t *testing.T) {
	store := newPureSegmentStore()
	idx := newVFSSemanticIndex(store)

	if err := idx.ensureCollection(4); err != nil {
		t.Fatalf("ensureCollection: %v", err)
	}

	// Index three memories with different embeddings
	memories := []struct {
		id       string
		section  string
		category string
		content  string
		vec      []float32
	}{
		{"mem-1", "episodic", "observations", "我喜欢喝咖啡", []float32{0.9, 0.1, 0.0, 0.0}},
		{"mem-2", "semantic", "reflections", "Go 语言的并发模型", []float32{0.0, 0.1, 0.9, 0.0}},
		{"mem-3", "episodic", "dialogues", "今天天气很好", []float32{0.1, 0.9, 0.0, 0.0}},
	}

	for _, m := range memories {
		if err := idx.indexContent(m.id, "t1", "u1", m.section, m.category, m.content, m.vec); err != nil {
			t.Fatalf("indexContent(%s): %v", m.id, err)
		}
	}

	// Search with a vector close to "咖啡" (mem-1)
	queryVec := []float32{0.8, 0.2, 0.0, 0.0}
	hits, err := idx.semanticSearchToFSHits(queryVec, 3)
	if err != nil {
		t.Fatalf("semanticSearch: %v", err)
	}
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d", len(hits))
	}

	// First hit should be mem-1 (highest cosine similarity)
	if hits[0].MemoryID != "mem-1" {
		t.Errorf("expected first hit to be mem-1, got %s", hits[0].MemoryID)
	}

	// Verify VFS URI format
	expectedURI := "episodic/observations/mem-1"
	if hits[0].Path != expectedURI {
		t.Errorf("expected URI %q, got %q", expectedURI, hits[0].Path)
	}

	// Verify content is preserved
	if hits[0].L0Abstract != "我喜欢喝咖啡" {
		t.Errorf("expected content '我喜欢喝咖啡', got %q", hits[0].L0Abstract)
	}
}

// TestHybridFusion_VectorPlusVFSSemantic tests RRF fusion of vector + VFS semantic results.
func TestHybridFusion_VectorPlusVFSSemantic(t *testing.T) {
	// Simulate vector store results (from mem_episodic etc.)
	vectorResults := []vectorSearchResult{
		{MemoryID: "v-1", Content: "向量结果1", Score: 0.95, MemoryType: "episodic"},
		{MemoryID: "v-2", Content: "向量结果2", Score: 0.80, MemoryType: "semantic"},
	}

	// Simulate VFS semantic search results
	fsHits := []memFSSearchHit{
		{MemoryID: "v-1", Score: 0.90, Path: "episodic/observations/v-1", L0Abstract: "VFS结果1", Category: "observations"}, // overlap with vector
		{MemoryID: "fs-1", Score: 0.85, Path: "semantic/knowledge/fs-1", L0Abstract: "仅VFS结果", Category: "knowledge"},
	}

	// Fuse with limit=3
	fused := fuseSearchResults(vectorResults, fsHits, 3)

	if len(fused) != 3 {
		t.Fatalf("expected 3 fused results, got %d", len(fused))
	}

	// v-1 should be first (appears in both lists → higher RRF score)
	if fused[0].MemoryID != "v-1" {
		t.Errorf("expected v-1 to be first (dual appearance), got %s", fused[0].MemoryID)
	}

	// All 3 unique IDs should be present
	ids := map[string]bool{}
	for _, r := range fused {
		ids[r.MemoryID] = true
	}
	for _, expected := range []string{"v-1", "v-2", "fs-1"} {
		if !ids[expected] {
			t.Errorf("missing expected ID: %s", expected)
		}
	}
}

// TestDeleteRemovesIndex verifies that removing an index actually removes the point.
func TestDeleteRemovesIndex(t *testing.T) {
	store := newPureSegmentStore()
	idx := newVFSSemanticIndex(store)

	if err := idx.ensureCollection(3); err != nil {
		t.Fatalf("ensureCollection: %v", err)
	}

	vec := []float32{0.5, 0.5, 0.5}
	if err := idx.indexContent("del-1", "t1", "u1", "episodic", "obs", "to delete", vec); err != nil {
		t.Fatalf("indexContent: %v", err)
	}

	// Verify indexed
	hits, err := idx.semanticSearchToFSHits(vec, 1)
	if err != nil {
		t.Fatalf("search before delete: %v", err)
	}
	if len(hits) != 1 || hits[0].MemoryID != "del-1" {
		t.Fatalf("expected del-1 before delete, got %v", hits)
	}

	// Delete
	if err := idx.removeIndex("del-1"); err != nil {
		t.Fatalf("removeIndex: %v", err)
	}

	// Verify removed
	hits, err = idx.semanticSearchToFSHits(vec, 1)
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits after delete, got %d", len(hits))
	}
}

// TestIndexMultipleUsers verifies that payloads correctly separate users.
func TestIndexMultipleUsers(t *testing.T) {
	store := newPureSegmentStore()
	idx := newVFSSemanticIndex(store)

	if err := idx.ensureCollection(3); err != nil {
		t.Fatalf("ensureCollection: %v", err)
	}

	// Index content for 2 users with same vector
	vec := []float32{0.5, 0.5, 0.5}
	_ = idx.indexContent("m1", "t1", "user-A", "episodic", "obs", "content A", vec)
	_ = idx.indexContent("m2", "t1", "user-B", "episodic", "obs", "content B", vec)

	// Search returns both (segment doesn't filter by user — filtering is done in Go services layer)
	hits, err := idx.semanticSearchToFSHits(vec, 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 hits, got %d", len(hits))
	}
}

// TestSemanticSearchOrdering verifies cosine similarity ordering.
func TestSemanticSearchOrdering(t *testing.T) {
	store := newPureSegmentStore()
	idx := newVFSSemanticIndex(store)

	if err := idx.ensureCollection(3); err != nil {
		t.Fatalf("ensureCollection: %v", err)
	}

	// close = very similar to query, far = very different
	close := []float32{0.9, 0.1, 0.0}
	mid := []float32{0.5, 0.5, 0.0}
	far := []float32{0.0, 0.0, 1.0}

	_ = idx.indexContent("close", "t1", "u1", "s", "c", "close", close)
	_ = idx.indexContent("mid", "t1", "u1", "s", "c", "mid", mid)
	_ = idx.indexContent("far", "t1", "u1", "s", "c", "far", far)

	query := []float32{1.0, 0.0, 0.0}
	hits, _ := idx.semanticSearchToFSHits(query, 3)

	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d", len(hits))
	}
	if hits[0].MemoryID != "close" {
		t.Errorf("first should be 'close', got %s", hits[0].MemoryID)
	}
	if hits[1].MemoryID != "mid" {
		t.Errorf("second should be 'mid', got %s", hits[1].MemoryID)
	}
	if hits[2].MemoryID != "far" {
		t.Errorf("third should be 'far', got %s", hits[2].MemoryID)
	}
}
