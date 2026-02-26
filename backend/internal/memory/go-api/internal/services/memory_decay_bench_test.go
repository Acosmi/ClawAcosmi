// Package services — P6-1: 记忆衰减算法性能基准测试。
//
// 覆盖指标: ns/op, B/op, allocs/op
// 运行: go test -bench=BenchmarkDecay -benchmem ./internal/services/...
package services

import (
	"testing"
	"time"

	"github.com/uhms/go-api/internal/ffi"
)

// =============================================================================
// BenchmarkComputeEffectiveImportance — 单条衰减计算
// =============================================================================

func BenchmarkComputeEffectiveImportance(b *testing.B) {
	now := time.Now().UTC()
	past := now.Add(-30 * 24 * time.Hour) // 30 天前

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeEffectiveImportance(0.8, 0.9, &past, 5, now, 30.0)
	}
}

func BenchmarkComputeEffectiveImportance_NilAccess(b *testing.B) {
	now := time.Now().UTC()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ComputeEffectiveImportance(0.8, 0.9, nil, 0, now, 30.0)
	}
}

// =============================================================================
// BenchmarkApplyDecayBatch — 批量衰减 (通过 ffi.BatchDecay pure Go fallback)
// =============================================================================

func benchApplyDecayBatch(b *testing.B, n int) {
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

func BenchmarkApplyDecayBatch_100(b *testing.B)   { benchApplyDecayBatch(b, 100) }
func BenchmarkApplyDecayBatch_1000(b *testing.B)  { benchApplyDecayBatch(b, 1000) }
func BenchmarkApplyDecayBatch_10000(b *testing.B) { benchApplyDecayBatch(b, 10000) }

// =============================================================================
// BenchmarkComputeAdaptiveHalfLife — 自适应半衰期公式
// =============================================================================

func BenchmarkComputeAdaptiveHalfLife(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		computeAdaptiveHalfLife("episodic", 7.0)
	}
}
