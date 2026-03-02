package vectoradapter

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
)

// ============================================================================
// Core search engine functional tests
// Covers: NewSearchEngine, NewSegmentVectorIndex, Upsert, Search,
//         SearchWithParams, UpsertPayload, SearchByPayload, Delete, PointCount
// ============================================================================

// uuid generates a deterministic UUID-like string for test point IDs.
// Qdrant segment requires UUID-format point IDs.
func uuid(n int) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", n)
}

func TestNewSearchEngine_SystemCollections(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewSearchEngine(dir)
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}
	defer idx.Close()

	if idx.Dimension() != DefaultSearchDimension {
		t.Errorf("dimension = %d, want %d", idx.Dimension(), DefaultSearchDimension)
	}

	// All 3 system collections should be accessible
	for _, col := range SystemCollections {
		cnt, err := idx.PointCount(col)
		if err != nil {
			t.Errorf("PointCount(%q): %v", col, err)
		}
		if cnt != 0 {
			t.Errorf("PointCount(%q) = %d, want 0", col, cnt)
		}
	}
}

func TestNewSegmentVectorIndex_AllCollections(t *testing.T) {
	dir := t.TempDir()
	dim := 128
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	if idx.Dimension() != dim {
		t.Errorf("dimension = %d, want %d", idx.Dimension(), dim)
	}

	// Memory collections should exist
	for _, col := range MemoryCollections() {
		cnt, err := idx.PointCount(col)
		if err != nil {
			t.Errorf("PointCount(%q): %v", col, err)
		}
		if cnt != 0 {
			t.Errorf("PointCount(%q) = %d, want 0", col, cnt)
		}
	}

	// System collections should also exist
	for _, col := range SystemCollections {
		cnt, err := idx.PointCount(col)
		if err != nil {
			t.Errorf("PointCount(%q): %v", col, err)
		}
		if cnt != 0 {
			t.Errorf("PointCount(%q) = %d, want 0", col, cnt)
		}
	}
}

func TestUpsertAndSearch(t *testing.T) {
	dir := t.TempDir()
	dim := 4
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "mem_episodic"

	// Insert 3 vectors with UUID IDs
	id1, id2, id3 := uuid(1), uuid(2), uuid(3)
	type vecEntry struct {
		id  string
		vec []float32
	}
	entries := []vecEntry{
		{id1, []float32{1, 0, 0, 0}},
		{id2, []float32{0, 1, 0, 0}},
		{id3, []float32{0.9, 0.1, 0, 0}},
	}
	for _, e := range entries {
		payload := map[string]interface{}{"name": e.id}
		if err := idx.Upsert(ctx, col, e.id, e.vec, payload); err != nil {
			t.Fatalf("Upsert(%q): %v", e.id, err)
		}
	}

	cnt, err := idx.PointCount(col)
	if err != nil {
		t.Fatalf("PointCount: %v", err)
	}
	if cnt != 3 {
		t.Errorf("PointCount = %d, want 3", cnt)
	}

	// Search for [1,0,0,0] — should find id1 first (exact match), id3 second (closest by cosine)
	results, err := idx.Search(ctx, col, []float32{1, 0, 0, 0}, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Search returned %d results, want 3", len(results))
	}
	if results[0].ID != id1 {
		t.Errorf("Search top result = %q, want %q", results[0].ID, id1)
	}
	if results[0].Score < 0.99 {
		t.Errorf("Search top score = %f, want ~1.0", results[0].Score)
	}
	if results[1].ID != id3 {
		t.Errorf("Search second result = %q, want %q", results[1].ID, id3)
	}
}

func TestSearchWithParams(t *testing.T) {
	dir := t.TempDir()
	dim := 4
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "mem_semantic"

	idA, idB := uuid(10), uuid(20)

	if err := idx.Upsert(ctx, col, idA, []float32{1, 0, 0, 0}, nil); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if err := idx.Upsert(ctx, col, idB, []float32{0, 1, 0, 0}, nil); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// SearchWithParams with hnswEf=0 (default)
	results, err := idx.SearchWithParams(ctx, col, []float32{1, 0, 0, 0}, 2, 0)
	if err != nil {
		t.Fatalf("SearchWithParams(ef=0): %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchWithParams returned 0 results")
	}
	if results[0].ID != idA {
		t.Errorf("SearchWithParams top = %q, want %q", results[0].ID, idA)
	}

	// SearchWithParams with hnswEf=128 (higher accuracy)
	results2, err := idx.SearchWithParams(ctx, col, []float32{1, 0, 0, 0}, 2, 128)
	if err != nil {
		t.Fatalf("SearchWithParams(ef=128): %v", err)
	}
	if len(results2) == 0 {
		t.Fatal("SearchWithParams(ef=128) returned 0 results")
	}
	if results2[0].ID != idA {
		t.Errorf("SearchWithParams(ef=128) top = %q, want %q", results2[0].ID, idA)
	}
}

func TestUpsertPayloadAndSearchByPayload(t *testing.T) {
	dir := t.TempDir()
	idx, err := NewSearchEngine(dir)
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "sys_skills"

	// Insert skills with UUID IDs
	skills := []struct {
		id   string
		name string
		desc string
	}{
		{uuid(100), "web_search", "Search the web for information"},
		{uuid(101), "code_review", "Review code for bugs and style issues"},
		{uuid(102), "translate", "Translate text between languages"},
	}
	for _, s := range skills {
		payload := map[string]interface{}{
			"name":        s.name,
			"description": s.desc,
		}
		if err := idx.UpsertPayload(ctx, col, s.id, payload); err != nil {
			t.Fatalf("UpsertPayload(%q): %v", s.id, err)
		}
	}

	cnt, err := idx.PointCount(col)
	if err != nil {
		t.Fatalf("PointCount: %v", err)
	}
	if cnt != 3 {
		t.Errorf("PointCount = %d, want 3", cnt)
	}

	// Search by keyword "code"
	results, err := idx.SearchByPayload(ctx, col, "code", 10)
	if err != nil {
		t.Fatalf("SearchByPayload: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchByPayload returned 0 results for 'code'")
	}
	if results[0].ID != uuid(101) {
		t.Errorf("SearchByPayload top = %q, want %q (code_review)", results[0].ID, uuid(101))
	}

	// Search by keyword "web search"
	results2, err := idx.SearchByPayload(ctx, col, "web search", 10)
	if err != nil {
		t.Fatalf("SearchByPayload: %v", err)
	}
	if len(results2) == 0 {
		t.Fatal("SearchByPayload returned 0 results for 'web search'")
	}
	// skills[0] should match both terms
	if results2[0].ID != uuid(100) {
		t.Errorf("SearchByPayload top = %q, want %q (web_search)", results2[0].ID, uuid(100))
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	dim := 4
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "mem_episodic"
	id := uuid(50)

	if err := idx.Upsert(ctx, col, id, []float32{1, 0, 0, 0}, nil); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	cnt, _ := idx.PointCount(col)
	if cnt != 1 {
		t.Fatalf("PointCount after insert = %d, want 1", cnt)
	}

	if err := idx.Delete(ctx, col, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	cnt, _ = idx.PointCount(col)
	if cnt != 0 {
		t.Errorf("PointCount after delete = %d, want 0", cnt)
	}
}

func TestDimensionMismatch(t *testing.T) {
	dir := t.TempDir()
	dim := 4
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "mem_episodic"

	// Wrong dimension on Upsert
	err = idx.Upsert(ctx, col, uuid(60), []float32{1, 0}, nil)
	if err == nil {
		t.Error("Upsert with wrong dimension should fail")
	}

	// Wrong dimension on Search
	_, err = idx.Search(ctx, col, []float32{1, 0}, 1)
	if err == nil {
		t.Error("Search with wrong dimension should fail")
	}

	// Wrong dimension on SearchWithParams
	_, err = idx.SearchWithParams(ctx, col, []float32{1, 0}, 1, 0)
	if err == nil {
		t.Error("SearchWithParams with wrong dimension should fail")
	}
}

func TestBuildConfigJSON(t *testing.T) {
	// Memory profile: should include HNSW
	memProfile := DefaultMemoryProfile
	memProfile.Dimension = 768
	data := buildConfigJSON(memProfile)
	if len(data) == 0 {
		t.Fatal("buildConfigJSON returned empty")
	}
	s := string(data)
	if !containsStr(s, "hnsw") {
		t.Errorf("memory config should contain hnsw: %s", s)
	}
	if !containsStr(s, `"m":16`) {
		t.Errorf("memory config should contain m=16: %s", s)
	}
	if !containsStr(s, `"ef_construct":200`) {
		t.Errorf("memory config should contain ef_construct=200: %s", s)
	}

	// System profile: no HNSW
	sysProfile := DefaultSystemProfile
	sysProfile.Dimension = 768
	data2 := buildConfigJSON(sysProfile)
	s2 := string(data2)
	if containsStr(s2, "hnsw") {
		t.Errorf("system config should NOT contain hnsw: %s", s2)
	}
}

func TestSearchEmptyCollection(t *testing.T) {
	dir := t.TempDir()
	dim := 4
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "mem_episodic"

	results, err := idx.Search(ctx, col, []float32{1, 0, 0, 0}, 10)
	if err != nil {
		t.Fatalf("Search on empty collection: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search on empty collection returned %d results, want 0", len(results))
	}
}

// TestCGOvsEnvironment verifies the engine works in the current CGO mode.
func TestCGOvsEnvironment(t *testing.T) {
	cgoEnabled := os.Getenv("CGO_ENABLED")
	t.Logf("CGO_ENABLED=%q (empty means default for platform)", cgoEnabled)

	dir := t.TempDir()
	idx, err := NewSearchEngine(dir)
	if err != nil {
		t.Fatalf("NewSearchEngine: %v", err)
	}
	defer idx.Close()

	// Basic smoke test: insert and retrieve
	ctx := context.Background()
	id := uuid(200)
	payload := map[string]interface{}{"name": "test_skill", "description": "a test skill"}
	if err := idx.UpsertPayload(ctx, "sys_skills", id, payload); err != nil {
		t.Fatalf("UpsertPayload: %v", err)
	}
	results, err := idx.SearchByPayload(ctx, "sys_skills", "test", 5)
	if err != nil {
		t.Fatalf("SearchByPayload: %v", err)
	}
	if len(results) == 0 {
		t.Error("SearchByPayload returned 0 results for inserted data")
	}
}

// TestSearchScoreOrdering verifies that cosine similarity scores are correctly ordered.
func TestSearchScoreOrdering(t *testing.T) {
	dir := t.TempDir()
	dim := 3
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "mem_semantic"

	idSame, idClose, idOrtho := uuid(300), uuid(301), uuid(302)

	// Insert vectors at known angles to query [1,0,0]
	//   same:       [1,0,0]     → cosine=1.0
	//   close:      [0.9,0.4,0] → cosine≈0.914
	//   orthogonal: [0,1,0]     → cosine=0.0
	if err := idx.Upsert(ctx, col, idSame, []float32{1, 0, 0}, nil); err != nil {
		t.Fatal(err)
	}
	if err := idx.Upsert(ctx, col, idClose, []float32{0.9, 0.4, 0}, nil); err != nil {
		t.Fatal(err)
	}
	if err := idx.Upsert(ctx, col, idOrtho, []float32{0, 1, 0}, nil); err != nil {
		t.Fatal(err)
	}

	results, err := idx.Search(ctx, col, []float32{1, 0, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify ordering: same > close > orthogonal
	if results[0].ID != idSame {
		t.Errorf("rank 0: got %q, want %q (same)", results[0].ID, idSame)
	}
	if results[1].ID != idClose {
		t.Errorf("rank 1: got %q, want %q (close)", results[1].ID, idClose)
	}

	// Verify score values are reasonable
	if math.Abs(results[0].Score-1.0) > 0.01 {
		t.Errorf("score[same] = %f, want ~1.0", results[0].Score)
	}
	if results[1].Score < 0.8 || results[1].Score > 0.95 {
		t.Errorf("score[close] = %f, want ~0.91", results[1].Score)
	}
}

// ============================================================================
// Optimization tests
// ============================================================================

func TestOptimizeMemoryCollections(t *testing.T) {
	dir := t.TempDir()
	dim := 4
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "mem_episodic"

	// Insert data into one memory collection.
	for i := 1; i <= 5; i++ {
		vec := make([]float32, dim)
		vec[i%dim] = 1.0
		payload := map[string]interface{}{"idx": i}
		if err := idx.Upsert(ctx, col, uuid(400+i), vec, payload); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
	}

	// Search before optimization.
	preResults, err := idx.Search(ctx, col, []float32{1, 0, 0, 0}, 5)
	if err != nil {
		t.Fatalf("Search pre-optimize: %v", err)
	}
	if len(preResults) == 0 {
		t.Fatal("Search pre-optimize returned 0 results")
	}

	// Optimize all memory collections.
	optimized, names, err := idx.OptimizeMemoryCollections(ctx)
	if err != nil {
		t.Fatalf("OptimizeMemoryCollections: %v", err)
	}
	// At least the collection with data should be optimized (CGO mode) or 0 (pure Go).
	t.Logf("OptimizeMemoryCollections: %d collections optimized (%v)", optimized, names)

	// Point count should be preserved.
	cnt, err := idx.PointCount(col)
	if err != nil {
		t.Fatalf("PointCount after optimize: %v", err)
	}
	if cnt != 5 {
		t.Errorf("PointCount after optimize = %d, want 5", cnt)
	}

	// Search after optimization should return consistent results.
	postResults, err := idx.Search(ctx, col, []float32{1, 0, 0, 0}, 5)
	if err != nil {
		t.Fatalf("Search post-optimize: %v", err)
	}
	if len(postResults) == 0 {
		t.Fatal("Search post-optimize returned 0 results")
	}
	// Top result should match.
	if preResults[0].ID != postResults[0].ID {
		t.Errorf("Top result changed after optimize: %q → %q", preResults[0].ID, postResults[0].ID)
	}
}

func TestOptimizeEmptyCollection(t *testing.T) {
	dir := t.TempDir()
	dim := 4
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	// Optimize with no data — should return 0 optimized.
	optimized, names, err := idx.OptimizeMemoryCollections(context.Background())
	if err != nil {
		t.Fatalf("OptimizeMemoryCollections: %v", err)
	}
	if optimized != 0 {
		t.Errorf("OptimizeMemoryCollections on empty = %d, want 0", optimized)
	}
	if len(names) != 0 {
		t.Errorf("OptimizeMemoryCollections names on empty = %v, want empty", names)
	}
}

func TestOptimizeSystemCollectionSkipped(t *testing.T) {
	dir := t.TempDir()
	dim := 4
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()

	// Insert into system collection.
	zeroVec := make([]float32, dim)
	payload := map[string]interface{}{"name": "test"}
	if err := idx.Upsert(ctx, "sys_skills", uuid(500), zeroVec, payload); err != nil {
		t.Fatalf("Upsert sys_skills: %v", err)
	}

	// Optimize single system collection should return false (no HNSW config).
	ok, err := idx.Optimize(ctx, "sys_skills")
	if err != nil {
		t.Fatalf("Optimize sys_skills: %v", err)
	}
	if ok {
		t.Error("Optimize sys_skills should return false (Plain index, no HNSW)")
	}
}

func TestSearchAfterOptimize(t *testing.T) {
	dir := t.TempDir()
	dim := 3
	idx, err := NewSegmentVectorIndex(dir, dim)
	if err != nil {
		t.Fatalf("NewSegmentVectorIndex: %v", err)
	}
	defer idx.Close()

	ctx := context.Background()
	col := "mem_semantic"

	// Insert vectors at known angles.
	idSame, idClose, idOrtho := uuid(600), uuid(601), uuid(602)
	if err := idx.Upsert(ctx, col, idSame, []float32{1, 0, 0}, nil); err != nil {
		t.Fatal(err)
	}
	if err := idx.Upsert(ctx, col, idClose, []float32{0.9, 0.4, 0}, nil); err != nil {
		t.Fatal(err)
	}
	if err := idx.Upsert(ctx, col, idOrtho, []float32{0, 1, 0}, nil); err != nil {
		t.Fatal(err)
	}

	// Optimize.
	_, err = idx.Optimize(ctx, col)
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}

	// Search should still return correct ordering.
	results, err := idx.Search(ctx, col, []float32{1, 0, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].ID != idSame {
		t.Errorf("rank 0: got %q, want %q", results[0].ID, idSame)
	}
	if results[1].ID != idClose {
		t.Errorf("rank 1: got %q, want %q", results[1].ID, idClose)
	}
	if math.Abs(results[0].Score-1.0) > 0.01 {
		t.Errorf("score[same] = %f, want ~1.0", results[0].Score)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
