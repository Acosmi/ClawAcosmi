package imaging

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

// Resize algorithm constants matching Rust side.
const (
	AlgoLanczos3 = 0 // High-quality (default)
	AlgoBilinear = 1 // Fast
	AlgoNearest  = 2 // Fastest
)

// RustResizeBGRA resizes BGRA pixel data using Rust SIMD.
//
// Parameters:
//   - pixels: BGRA pixel data
//   - w, h: source dimensions
//   - stride: source row stride in bytes (0 = w*4)
//   - dstW, dstH: target dimensions
//   - algo: AlgoLanczos3, AlgoBilinear, or AlgoNearest
//
// Returns resized BGRA pixel data. The returned slice is a Go-managed
// copy; no manual memory management needed.
func RustResizeBGRA(pixels []byte, w, h, stride, dstW, dstH, algo int) ([]byte, error) {
	if len(pixels) == 0 || w <= 0 || h <= 0 || dstW <= 0 || dstH <= 0 {
		return nil, fmt.Errorf("invalid resize parameters: src=%dx%d dst=%dx%d len=%d",
			w, h, dstW, dstH, len(pixels))
	}

	var outPtr *C.uint8_t
	var outLen C.size_t

	rc := C.argus_image_resize(
		(*C.uint8_t)(unsafe.Pointer(&pixels[0])),
		C.int32_t(w), C.int32_t(h), C.int32_t(stride),
		C.int32_t(dstW), C.int32_t(dstH), C.int32_t(algo),
		&outPtr, &outLen,
	)
	if rc != 0 {
		return nil, fmt.Errorf("argus_image_resize failed: error code %d", rc)
	}
	defer C.argus_free_buffer(outPtr, outLen)

	// Copy to Go-managed memory.
	result := C.GoBytes(unsafe.Pointer(outPtr), C.int(outLen))
	return result, nil
}

// RustCalcFitSize calculates target dimensions that fit within maxDim
// on the longest edge while preserving aspect ratio.
func RustCalcFitSize(w, h, maxDim int) (int, int, error) {
	if w <= 0 || h <= 0 || maxDim <= 0 {
		return 0, 0, fmt.Errorf("invalid parameters: w=%d h=%d maxDim=%d", w, h, maxDim)
	}

	var outW, outH C.int32_t
	rc := C.argus_image_calc_fit_size(
		C.int32_t(w), C.int32_t(h), C.int32_t(maxDim),
		&outW, &outH,
	)
	if rc != 0 {
		return 0, 0, fmt.Errorf("argus_image_calc_fit_size failed: error code %d", rc)
	}
	return int(outW), int(outH), nil
}
