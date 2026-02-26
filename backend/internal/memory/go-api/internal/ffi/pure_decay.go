//go:build !cgo

package ffi

import "math"

// Decay 纯 Go 衰减算法 fallback。
func Decay(baseImportance, decayFactor, daysSinceAccess float64, accessCount int) float64 {
	const halfLifeDays = 30.0
	const minDecay = 0.01

	lambda := math.Ln2 / halfLifeDays
	timeFactor := math.Exp(-lambda * daysSinceAccess)
	accessBoost := 1.0 + math.Log1p(float64(accessCount))*0.1
	result := baseImportance * decayFactor * timeFactor * accessBoost

	if result < minDecay {
		return minDecay
	}
	if result > 1.0 {
		return 1.0
	}
	return result
}

// DecayParam 衰减参数（与 cgo 版一致）。
type DecayParam struct {
	BaseImportance  float64
	DecayFactor     float64
	DaysSinceAccess float64
	AccessCount     int
}

// BatchDecay 批量计算衰减（纯 Go）。
func BatchDecay(inputs []DecayParam) []float64 {
	if len(inputs) == 0 {
		return nil
	}
	out := make([]float64, len(inputs))
	for i, inp := range inputs {
		out[i] = Decay(inp.BaseImportance, inp.DecayFactor, inp.DaysSinceAccess, inp.AccessCount)
	}
	return out
}
