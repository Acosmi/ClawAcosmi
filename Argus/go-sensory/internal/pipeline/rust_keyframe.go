package pipeline

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust-core/include
#cgo LDFLAGS: -L${SRCDIR}/../../../rust-core/target/release -largus_core -Wl,-rpath,/usr/lib/swift
#include "argus_core.h"
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// RustCalcChangeRatio computes pixel change ratio between two BGRA frames
// using Rust SIMD. A pixel is "changed" if any RGB channel differs by more
// than threshold (typically 20).
//
// Returns a ratio in [0.0, 1.0] where 1.0 = all pixels changed.
func RustCalcChangeRatio(prev, curr []byte, w, h, stride, threshold int) (float64, error) {
	if len(prev) == 0 || len(curr) == 0 {
		return 0, fmt.Errorf("empty frame data: prev=%d curr=%d", len(prev), len(curr))
	}
	if w <= 0 || h <= 0 {
		return 0, fmt.Errorf("invalid dimensions: %dx%d", w, h)
	}

	var ratio C.double
	rc := C.argus_keyframe_diff(
		(*C.uint8_t)(unsafe.Pointer(&prev[0])),
		(*C.uint8_t)(unsafe.Pointer(&curr[0])),
		C.int32_t(w), C.int32_t(h), C.int32_t(stride),
		C.int32_t(threshold),
		&ratio,
	)
	if rc != 0 {
		return 0, fmt.Errorf("argus_keyframe_diff failed: error code %d", rc)
	}
	return float64(ratio), nil
}

// RustFrameHash computes a 64-bit perceptual hash (dHash) for a BGRA frame.
// Two frames with hamming distance < 10 are visually similar.
func RustFrameHash(pixels []byte, w, h, stride int) (uint64, error) {
	if len(pixels) == 0 {
		return 0, fmt.Errorf("empty frame data")
	}
	if w <= 0 || h <= 0 {
		return 0, fmt.Errorf("invalid dimensions: %dx%d", w, h)
	}

	var hash C.uint64_t
	rc := C.argus_keyframe_hash(
		(*C.uint8_t)(unsafe.Pointer(&pixels[0])),
		C.int32_t(w), C.int32_t(h), C.int32_t(stride),
		&hash,
	)
	if rc != 0 {
		return 0, fmt.Errorf("argus_keyframe_hash failed: error code %d", rc)
	}
	return uint64(hash), nil
}
