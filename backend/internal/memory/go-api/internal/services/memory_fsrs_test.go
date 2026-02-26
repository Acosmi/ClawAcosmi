package services

import (
	"math"
	"testing"
)

// TestRetrievability_AtStability_Returns90Pct verifies R(S, S) ≈ 90%.
func TestRetrievability_AtStability_Returns90Pct(t *testing.T) {
	stability := 30.0
	w20 := DefaultFSRSParams.W[20]
	r := Retrievability(stability, stability, w20)
	// FSRS guarantees R(S, S) ≈ 0.9 (via factor = 19/81)
	if math.Abs(r-0.9) > 0.01 {
		t.Errorf("R(S,S) should ≈ 0.9, got %f", r)
	}
}

// TestRetrievability_AtZero_Returns100Pct verifies R(0, S) = 100%.
func TestRetrievability_AtZero_Returns100Pct(t *testing.T) {
	r := Retrievability(0, 30.0, DefaultFSRSParams.W[20])
	if r != 1.0 {
		t.Errorf("R(0, S) should = 1.0, got %f", r)
	}
}

// TestRetrievability_PowerLaw_vs_Exponential verifies power-law decays slower than exponential at large t.
func TestRetrievability_PowerLaw_vs_Exponential(t *testing.T) {
	stability := 30.0
	w20 := DefaultFSRSParams.W[20]
	days := 90.0 // 3x stability

	// Power-law (FSRS-6)
	rPower := Retrievability(days, stability, w20)

	// Exponential (old formula)
	rExp := math.Exp(-0.693147 * days / stability)

	// Power-law should decay more slowly at large t
	if rPower <= rExp {
		t.Errorf("Power-law R(%f) = %f should > exponential R = %f", days, rPower, rExp)
	}
}

// TestRetrievability_ZeroStability returns 0.
func TestRetrievability_ZeroStability(t *testing.T) {
	r := Retrievability(10, 0, DefaultFSRSParams.W[20])
	if r != 0 {
		t.Errorf("R with zero stability should = 0, got %f", r)
	}
}

// TestRetrievability_NegativeElapsed returns 1.
func TestRetrievability_NegativeElapsed(t *testing.T) {
	r := Retrievability(-5, 30, DefaultFSRSParams.W[20])
	if r != 1.0 {
		t.Errorf("R with negative elapsed should = 1.0, got %f", r)
	}
}

// TestInitialStability_GradeMapping verifies w0-w3 correspond to 4 grades.
func TestInitialStability_GradeMapping(t *testing.T) {
	p := &DefaultFSRSParams
	grades := []FSRSGrade{FSRSAgain, FSRSHard, FSRSGood, FSRSEasy}
	for i, g := range grades {
		s := InitialStability(p, g)
		expected := p.W[i]
		if expected < 0.01 {
			expected = 0.01
		}
		if math.Abs(s-expected) > 1e-6 {
			t.Errorf("InitialStability(grade=%d) = %f, want %f", g, s, expected)
		}
	}

	// Stability should increase with grade
	sAgain := InitialStability(p, FSRSAgain)
	sEasy := InitialStability(p, FSRSEasy)
	if sAgain >= sEasy {
		t.Errorf("S(Again)=%f should < S(Easy)=%f", sAgain, sEasy)
	}
}

// TestStabilityAfterRecall_IncreasesWithGoodGrade verifies successful recall increases stability.
func TestStabilityAfterRecall_IncreasesWithGoodGrade(t *testing.T) {
	p := &DefaultFSRSParams
	d := 5.0  // medium difficulty
	s := 10.0 // current stability
	r := 0.9  // high retrievability

	newS := StabilityAfterRecall(p, d, s, r, FSRSGood)
	if newS <= s {
		t.Errorf("StabilityAfterRecall(Good) = %f should > original S = %f", newS, s)
	}

	// Easy should give even higher stability than Good
	newSEasy := StabilityAfterRecall(p, d, s, r, FSRSEasy)
	if newSEasy <= newS {
		t.Errorf("StabilityAfterRecall(Easy)=%f should > Good=%f", newSEasy, newS)
	}
}

// TestStabilityAfterLapse_DecreasesButNotBelowMin verifies lapse reduces stability but has a floor.
func TestStabilityAfterLapse_DecreasesButNotBelowMin(t *testing.T) {
	p := &DefaultFSRSParams
	d := 5.0  // medium difficulty
	s := 30.0 // current stability
	r := 0.5  // low retrievability

	newS := StabilityAfterLapse(p, d, s, r)
	if newS >= s {
		t.Errorf("StabilityAfterLapse = %f should < original S = %f", newS, s)
	}
	if newS < 0.01 {
		t.Errorf("StabilityAfterLapse = %f should >= 0.01", newS)
	}
}

// TestSameDayStability_Properties verifies same-day review constraints.
func TestSameDayStability_Properties(t *testing.T) {
	p := &DefaultFSRSParams
	s := 10.0

	// Good grade should increase or maintain
	newS := SameDayStability(p, s, FSRSGood)
	if newS < s {
		t.Errorf("SameDayStability(Good) = %f should >= S = %f", newS, s)
	}

	// Easy should give >= Good
	newSEasy := SameDayStability(p, s, FSRSEasy)
	if newSEasy < newS {
		t.Errorf("SameDayStability(Easy)=%f should >= Good=%f", newSEasy, newS)
	}
}

// TestDefaultFSRSParams_Length21 verifies parameter count.
func TestDefaultFSRSParams_Length21(t *testing.T) {
	if len(DefaultFSRSParams.W) != 21 {
		t.Errorf("DefaultFSRSParams should have 21 parameters, got %d", len(DefaultFSRSParams.W))
	}

	// All parameters should be non-negative
	for i, w := range DefaultFSRSParams.W {
		if w < 0 {
			t.Errorf("w[%d] = %f should be non-negative", i, w)
		}
	}
}

// TestInitialDifficulty_Bounds verifies difficulty is clamped to [1, 10].
func TestInitialDifficulty_Bounds(t *testing.T) {
	p := &DefaultFSRSParams
	for _, g := range []FSRSGrade{FSRSAgain, FSRSHard, FSRSGood, FSRSEasy} {
		d := InitialDifficulty(p, g)
		if d < 1 || d > 10 {
			t.Errorf("InitialDifficulty(grade=%d) = %f, should be in [1, 10]", g, d)
		}
	}

	// Again should be harder than Easy
	dAgain := InitialDifficulty(p, FSRSAgain)
	dEasy := InitialDifficulty(p, FSRSEasy)
	if dAgain <= dEasy {
		t.Errorf("D(Again)=%f should > D(Easy)=%f", dAgain, dEasy)
	}
}

// TestNextDifficulty_MeanReversion verifies difficulty converges toward D0(Easy).
func TestNextDifficulty_MeanReversion(t *testing.T) {
	p := &DefaultFSRSParams
	d := 9.0 // very hard

	// After many Good reviews, difficulty should decrease toward equilibrium
	for i := 0; i < 10; i++ {
		d = NextDifficulty(p, d, FSRSGood)
	}
	if d >= 9.0 {
		t.Errorf("After 10 Good reviews, difficulty should decrease from 9.0, got %f", d)
	}
}

// TestRetrievability_DefaultW20_Fallback verifies w20 <= 0 uses default.
func TestRetrievability_DefaultW20_Fallback(t *testing.T) {
	r1 := Retrievability(30, 30, 0)
	r2 := Retrievability(30, 30, DefaultFSRSParams.W[20])
	if math.Abs(r1-r2) > 1e-10 {
		t.Errorf("w20=0 should fallback to default: %f vs %f", r1, r2)
	}
}
