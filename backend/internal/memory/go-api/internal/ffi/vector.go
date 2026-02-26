//go:build cgo

package ffi

// CGO 绑定：nexus-vector — 向量余弦相似度计算。
//
// 替换 Go 侧 tree_algorithm.go 中的 cosineSimilarity32。

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../libs/nexus-core/target/release -lnexus_unified -lm
#cgo CFLAGS: -I${SRCDIR}/../../../../libs/nexus-core/include
#include "nexus_vector.h"
*/
import "C"

import "unsafe"

// CosineSimilarity 计算两个 float32 向量的余弦相似度。
func CosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	return float32(C.nexus_cosine_similarity(
		(*C.float)(unsafe.Pointer(&a[0])),
		(*C.float)(unsafe.Pointer(&b[0])),
		C.size_t(len(a)),
	))
}

// BatchCosine 批量计算 query 与多个 doc 向量的余弦相似度。
// docs 为行主序矩阵 (nDocs × dim)。
func BatchCosine(query []float32, docs [][]float32) ([]float32, error) {
	if len(query) == 0 || len(docs) == 0 {
		return nil, nil
	}
	dim := len(query)
	nDocs := len(docs)

	// 展平 docs 为连续内存
	flat := make([]float32, 0, nDocs*dim)
	for _, d := range docs {
		if len(d) != dim {
			continue
		}
		flat = append(flat, d...)
	}
	actualDocs := len(flat) / dim
	if actualDocs == 0 {
		return nil, nil
	}

	out := make([]float32, actualDocs)
	C.nexus_batch_cosine(
		(*C.float)(unsafe.Pointer(&query[0])),
		(*C.float)(unsafe.Pointer(&flat[0])),
		C.size_t(dim),
		C.size_t(actualDocs),
		(*C.float)(unsafe.Pointer(&out[0])),
	)
	return out, nil
}
