//go:build cgo

package ffi

// CGO 绑定：nexus-decay — 记忆衰减算法。
//
// 替换 Go 侧 memory_decay.go 中的 ComputeEffectiveImportance（批量场景）。

/*
#cgo LDFLAGS: -L${SRCDIR}/../../../../libs/nexus-core/target/release -lnexus_unified -lm
#cgo CFLAGS: -I${SRCDIR}/../../../../libs/nexus-core/include
#include "nexus_decay.h"
*/
import "C"

import "unsafe"

// Decay 计算单条记忆的有效重要性（Rust 加速版）。
func Decay(baseImportance, decayFactor, daysSinceAccess float64, accessCount int) float64 {
	return float64(C.nexus_decay(
		C.double(baseImportance),
		C.double(decayFactor),
		C.double(daysSinceAccess),
		C.int(accessCount),
	))
}

// BatchDecay 批量计算记忆衰减。
func BatchDecay(inputs []DecayParam) []float64 {
	if len(inputs) == 0 {
		return nil
	}

	cInputs := make([]C.DecayInput, len(inputs))
	for i, inp := range inputs {
		cInputs[i] = C.DecayInput{
			base_importance:   C.double(inp.BaseImportance),
			decay_factor:      C.double(inp.DecayFactor),
			days_since_access: C.double(inp.DaysSinceAccess),
			access_count:      C.int(inp.AccessCount),
		}
	}

	out := make([]float64, len(inputs))
	C.nexus_batch_decay(
		&cInputs[0],
		C.size_t(len(inputs)),
		(*C.double)(unsafe.Pointer(&out[0])),
	)
	return out
}

// DecayParam 衰减参数（Go 侧结构体）。
type DecayParam struct {
	BaseImportance  float64
	DecayFactor     float64
	DaysSinceAccess float64
	AccessCount     int
}
