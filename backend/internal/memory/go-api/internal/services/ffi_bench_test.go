// Package services — P6-3: FFI 跨语言调用性能基准测试。
//
// 测量 Go ↔ Rust FFI 层的调用开销。
// 在 !cgo 模式下仍可运行 (pure Go fallback)，提供基线。
// 在 cgo 模式下可对比 Rust 加速效果。
//
// 覆盖指标: ns/op, B/op, allocs/op
// 运行: go test -bench=BenchmarkFFI -benchmem ./internal/services/...
package services

import (
	"math/rand"
	"testing"

	"github.com/uhms/go-api/internal/ffi"
)

// =============================================================================
// BenchmarkFFIDecaySingle — ffi.Decay 单次调用
// =============================================================================

func BenchmarkFFIDecaySingle(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ffi.Decay(0.8, 0.9, 30.0, 5)
	}
}

// =============================================================================
// BenchmarkFFIBatchDecay — ffi.BatchDecay 批量调用
// =============================================================================

func benchFFIBatchDecay(b *testing.B, n int) {
	b.Helper()
	inputs := make([]ffi.DecayParam, n)
	for i := range inputs {
		inputs[i] = ffi.DecayParam{
			BaseImportance:  0.5 + float64(i%50)*0.01,
			DecayFactor:     0.8 + float64(i%20)*0.01,
			DaysSinceAccess: float64(i%365) + 1,
			AccessCount:     i % 100,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ffi.BatchDecay(inputs)
	}
}

func BenchmarkFFIBatchDecay_100(b *testing.B)   { benchFFIBatchDecay(b, 100) }
func BenchmarkFFIBatchDecay_1000(b *testing.B)  { benchFFIBatchDecay(b, 1000) }
func BenchmarkFFIBatchDecay_10000(b *testing.B) { benchFFIBatchDecay(b, 10000) }

// =============================================================================
// BenchmarkFFICosineSimilarity — ffi.CosineSimilarity 单对向量
// =============================================================================

func benchFFICosineSimilarity(b *testing.B, dim int) {
	b.Helper()
	rng := rand.New(rand.NewSource(42))
	a := makeRandomVecFFI(rng, dim)
	vecB := makeRandomVecFFI(rng, dim)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ffi.CosineSimilarity(a, vecB)
	}
}

func BenchmarkFFICosineSimilarity_dim384(b *testing.B) { benchFFICosineSimilarity(b, 384) }
func BenchmarkFFICosineSimilarity_dim768(b *testing.B) { benchFFICosineSimilarity(b, 768) }

// =============================================================================
// BenchmarkFFIBatchCosine — ffi.BatchCosine 批量余弦
// =============================================================================

func benchFFIBatchCosine(b *testing.B, nDocs int) {
	b.Helper()
	const dim = 384
	rng := rand.New(rand.NewSource(42))

	query := makeRandomVecFFI(rng, dim)
	docs := make([][]float32, nDocs)
	for i := range docs {
		docs[i] = makeRandomVecFFI(rng, dim)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ffi.BatchCosine(query, docs)
	}
}

func BenchmarkFFIBatchCosine_10docs(b *testing.B)  { benchFFIBatchCosine(b, 10) }
func BenchmarkFFIBatchCosine_100docs(b *testing.B) { benchFFIBatchCosine(b, 100) }

// =============================================================================
// helpers (避免与 vector_store_bench_test.go 冲突)
// =============================================================================

func makeRandomVecFFI(rng *rand.Rand, dim int) []float32 {
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = rng.Float32()*2 - 1
	}
	return vec
}
