package services

import (
	"math"
	"testing"
	"time"
)

// TestComputeEffectiveImportance_WithAdaptiveHalfLife tests the updated function
// with the new halfLifeDays parameter.
func TestComputeEffectiveImportance_WithAdaptiveHalfLife(t *testing.T) {
	now := time.Now().UTC()
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)

	tests := []struct {
		name        string
		base        float64
		decay       float64
		lastAccess  *time.Time
		accessCount int
		halfLife    float64
		wantMin     float64
		wantMax     float64
	}{
		{
			name: "高频用户 - 短半衰期14天, 30天前访问",
			base: 0.8, decay: 0.9,
			lastAccess: &thirtyDaysAgo, accessCount: 5,
			halfLife: 14.0,
			wantMin:  0.01, wantMax: 0.8, // FSRS-6 幂律衰减比指数慢
		},
		{
			name: "低频用户 - 长半衰期60天, 30天前访问",
			base: 0.8, decay: 0.9,
			lastAccess: &thirtyDaysAgo, accessCount: 5,
			halfLife: 60.0,
			wantMin:  0.3, wantMax: 1.0, // 慢衰减 → 高值
		},
		{
			name: "默认半衰期30天, 30天前访问",
			base: 0.8, decay: 0.9,
			lastAccess: &thirtyDaysAgo, accessCount: 5,
			halfLife: 30.0,
			wantMin:  0.5, wantMax: 1.0, // FSRS-6: R(S,S) ≈ 0.9
		},
		{
			name: "零半衰期 - 应回退到默认值",
			base: 0.8, decay: 0.9,
			lastAccess: &thirtyDaysAgo, accessCount: 5,
			halfLife: 0, // 应 fallback 到 DefaultHalfLifeDays
			wantMin:  0.5, wantMax: 1.0,
		},
		{
			name: "nil lastAccess - 无衰减",
			base: 0.8, decay: 0.9,
			lastAccess: nil, accessCount: 0,
			halfLife: 30.0,
			wantMin:  0.5, wantMax: 1.0, // 无时间衰减
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeEffectiveImportance(
				tt.base, tt.decay, tt.lastAccess, tt.accessCount, now, tt.halfLife,
			)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("ComputeEffectiveImportance() = %f, want [%f, %f]",
					got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestComputeAdaptiveHalfLife tests the formula:
// adaptive_half_life = type_base × (1 + log₂(1 + avg_interval / 7))
func TestComputeAdaptiveHalfLife(t *testing.T) {
	tests := []struct {
		name        string
		memType     string
		avgInterval float64
		wantMin     float64
		wantMax     float64
	}{
		{
			name:        "高频用户 episodic (每天访问, interval≈1)",
			memType:     "episodic",
			avgInterval: 1.0,
			wantMin:     14.0, wantMax: 20.0, // 14 × 1.19 ≈ 16.7
		},
		{
			name:        "中频用户 episodic (每周访问, interval≈7)",
			memType:     "episodic",
			avgInterval: 7.0,
			wantMin:     26.0, wantMax: 30.0, // 14 × (1+log₂(2)) = 14 × 2 = 28
		},
		{
			name:        "低频用户 episodic (每月访问, interval≈30)",
			memType:     "episodic",
			avgInterval: 30.0,
			wantMin:     45.0, wantMax: 50.0, // 14 × (1+log₂(5.29)) = 14 × 3.40 ≈ 47.6
		},
		{
			name:        "reflection 类型 (高频访问)",
			memType:     "reflection",
			avgInterval: 1.0,
			wantMin:     60.0, wantMax: 80.0, // 60 × 1.19 ≈ 71.5
		},
		{
			name:        "observation 类型 (中频)",
			memType:     "observation",
			avgInterval: 7.0,
			wantMin:     58.0, wantMax: 62.0, // 30 × (1+log₂(2)) = 30 × 2 = 60
		},
		{
			name:        "未知类型回退默认",
			memType:     "unknown_type",
			avgInterval: 7.0,
			wantMin:     58.0, wantMax: 62.0, // DefaultHalfLifeDays(30) × 2 = 60
		},
		{
			name:        "新用户 (无数据, interval=0)",
			memType:     "episodic",
			avgInterval: 0.0,
			wantMin:     14.0, wantMax: 15.0, // 14 × 1 = 14
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeAdaptiveHalfLife(tt.memType, tt.avgInterval)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("computeAdaptiveHalfLife(%q, %f) = %f, want [%f, %f]",
					tt.memType, tt.avgInterval, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

// TestComputeAdaptiveHalfLife_Bounds tests that the result is clamped to [7, 365].
func TestComputeAdaptiveHalfLife_Bounds(t *testing.T) {
	// Very high interval → should be clamped at 365
	got := computeAdaptiveHalfLife("reflection", 10000.0)
	if got > 365.0 {
		t.Errorf("expected ≤ 365, got %f", got)
	}

	// All valid types should produce ≥ 7
	for memType := range TypeBaseHalfLife {
		got := computeAdaptiveHalfLife(memType, 0.0)
		if got < 7.0 {
			t.Errorf("type %q with interval=0: expected ≥ 7, got %f", memType, got)
		}
	}
}

// TestTypeBaseHalfLife_OrderedCorrectly verifies the design principle:
// episodic < observation < procedural < reflection
func TestTypeBaseHalfLife_OrderedCorrectly(t *testing.T) {
	if TypeBaseHalfLife["episodic"] >= TypeBaseHalfLife["observation"] {
		t.Error("episodic should decay faster than observation")
	}
	if TypeBaseHalfLife["observation"] >= TypeBaseHalfLife["reflection"] {
		t.Error("observation should decay faster than reflection")
	}
}

// TestProtectedMemoryTypes_NotInTypeBase verifies protected types have no decay profile.
func TestProtectedMemoryTypes_NotInTypeBase(t *testing.T) {
	for _, pType := range ProtectedMemoryTypes {
		if _, ok := TypeBaseHalfLife[pType]; ok {
			t.Errorf("protected type %q should NOT be in TypeBaseHalfLife", pType)
		}
	}
}

// TestGetAdaptiveHalfLife_NilDB tests the fallback behavior when DB query fails.
func TestGetAdaptiveHalfLife_NilDB(t *testing.T) {
	// We can't easily pass a nil DB to GORM, so we test the fallback logic directly.
	// Test type-based fallback
	for memType, expected := range TypeBaseHalfLife {
		got := computeAdaptiveHalfLife(memType, 0.0) // interval=0 → factor=1, result=base
		if math.Abs(got-expected) > 0.01 {
			t.Errorf("computeAdaptiveHalfLife(%q, 0) = %f, want %f", memType, got, expected)
		}
	}

	// Unknown type → DefaultHalfLifeDays
	got := computeAdaptiveHalfLife("totally_unknown", 0.0)
	if math.Abs(got-DefaultHalfLifeDays) > 0.01 {
		t.Errorf("unknown type should return DefaultHalfLifeDays=%f, got %f",
			DefaultHalfLifeDays, got)
	}
}

// TestComputeEffectiveImportance_DifferentHalfLives verifies that shorter half-lives
// produce lower effective importance for the same time gap.
func TestComputeEffectiveImportance_DifferentHalfLives(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-30 * 24 * time.Hour) // 30 days ago

	shortHL := ComputeEffectiveImportance(0.8, 1.0, &past, 0, now, 14.0)
	longHL := ComputeEffectiveImportance(0.8, 1.0, &past, 0, now, 60.0)

	if shortHL >= longHL {
		t.Errorf("shorter half-life should produce lower importance: short=%f, long=%f",
			shortHL, longHL)
	}
}

// TestComputeEffectiveImportance_FSRS6_PowerLawDecay verifies FSRS-6 power-law is used.
func TestComputeEffectiveImportance_FSRS6_PowerLawDecay(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-30 * 24 * time.Hour)

	// With FSRS-6 power-law, at t=halfLife (used as stability), R ≈ 0.9 (not 0.5 like exponential)
	result := ComputeEffectiveImportance(1.0, 1.0, &past, 0, now, 30.0)

	// effective = 1.0 * 1.0 * R(30, 30, w20) * 1.0
	// R(30, 30, w20) ≈ 0.9
	if result < 0.85 || result > 0.95 {
		t.Errorf("FSRS-6 at t=halfLife should yield ≈0.9, got %f", result)
	}
}
