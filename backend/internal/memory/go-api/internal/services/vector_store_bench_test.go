// Package services — P6-2: 向量搜索性能基准测试。
//
// 覆盖指标: ns/op, B/op, allocs/op
// 运行: go test -bench=BenchmarkVector -benchmem ./internal/services/...
package services

import (
	"math/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/uhms/go-api/internal/ffi"
)

// =============================================================================
// BenchmarkSegmentSearch — Segment Store 搜索 (pure Go brute-force)
// =============================================================================

func benchSegmentSearch(b *testing.B, numVectors int) {
	b.Helper()
	const dim = 384
	const collection = "bench_test"

	store, err := ffi.NewSegmentStore(b.TempDir())
	if err != nil {
		b.Fatalf("创建 SegmentStore 失败: %v", err)
	}
	defer store.Close()

	if err := store.CreateCollection(collection, dim); err != nil {
		b.Fatalf("创建集合失败: %v", err)
	}

	// 预灌入随机向量
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < numVectors; i++ {
		vec := makeRandomVec(rng, dim)
		id := uuid.New().String()
		if err := store.Upsert(collection, id, vec, nil); err != nil {
			b.Fatalf("Upsert 失败: %v", err)
		}
	}

	// 查询向量
	queryVec := makeRandomVec(rng, dim)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := store.Search(collection, queryVec, 10)
		if err != nil {
			b.Fatalf("Search 失败: %v", err)
		}
	}
}

func BenchmarkSegmentSearch_100(b *testing.B)  { benchSegmentSearch(b, 100) }
func BenchmarkSegmentSearch_500(b *testing.B)  { benchSegmentSearch(b, 500) }
func BenchmarkSegmentSearch_1000(b *testing.B) { benchSegmentSearch(b, 1000) }

// =============================================================================
// BenchmarkCosineSimilarity — 纯余弦相似度计算
// =============================================================================

func benchCosineSimilarity(b *testing.B, dim int) {
	b.Helper()
	rng := rand.New(rand.NewSource(42))
	a := makeRandomVec(rng, dim)
	vecB := makeRandomVec(rng, dim)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ffi.CosineSimilarity(a, vecB)
	}
}

func BenchmarkCosineSimilarity_dim384(b *testing.B)  { benchCosineSimilarity(b, 384) }
func BenchmarkCosineSimilarity_dim768(b *testing.B)  { benchCosineSimilarity(b, 768) }
func BenchmarkCosineSimilarity_dim1536(b *testing.B) { benchCosineSimilarity(b, 1536) }

// =============================================================================
// BenchmarkSegmentUpsert — 向量插入吞吐量
// =============================================================================

func BenchmarkSegmentUpsert(b *testing.B) {
	const dim = 384
	const collection = "bench_upsert"

	store, err := ffi.NewSegmentStore(b.TempDir())
	if err != nil {
		b.Fatalf("创建 SegmentStore 失败: %v", err)
	}
	defer store.Close()

	if err := store.CreateCollection(collection, dim); err != nil {
		b.Fatalf("创建集合失败: %v", err)
	}

	rng := rand.New(rand.NewSource(42))
	vecs := make([][]float32, b.N)
	ids := make([]string, b.N)
	for i := range vecs {
		vecs[i] = makeRandomVec(rng, dim)
		ids[i] = uuid.New().String()
	}

	payload := []byte(`{"user_id":"bench-user","type":"episodic"}`)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := store.Upsert(collection, ids[i], vecs[i], payload); err != nil {
			b.Fatalf("Upsert 失败: %v", err)
		}
	}
}

// =============================================================================
// helpers
// =============================================================================

func makeRandomVec(rng *rand.Rand, dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = rng.Float32()*2 - 1 // [-1, 1]
	}
	return vec
}
