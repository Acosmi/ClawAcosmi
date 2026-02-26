//go:build !cgo

package ffi

import "math"

// CosineSimilarity 纯 Go 余弦相似度 fallback。
func CosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
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

// BatchCosine 批量计算余弦相似度（纯 Go）。
func BatchCosine(query []float32, docs [][]float32) ([]float32, error) {
	if len(query) == 0 || len(docs) == 0 {
		return nil, nil
	}
	out := make([]float32, 0, len(docs))
	for _, d := range docs {
		if len(d) != len(query) {
			continue
		}
		out = append(out, CosineSimilarity(query, d))
	}
	return out, nil
}
