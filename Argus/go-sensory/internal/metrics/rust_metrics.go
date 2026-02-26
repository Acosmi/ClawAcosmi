package metrics

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust-core/include
#cgo LDFLAGS: -L${SRCDIR}/../../../rust-core/target/release -largus_core -Wl,-rpath,/usr/lib/swift
#include "argus_core.h"
#include <stdlib.h>
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// RustMetrics holds the Rust-side counter values.
type RustMetrics struct {
	FramesCaptured uint64 `json:"rust_frames_captured_total"`
	ResizesTotal   uint64 `json:"rust_resizes_total"`
	ShmWritesTotal uint64 `json:"rust_shm_writes_total"`
	KeyframeDiffs  uint64 `json:"rust_keyframe_diffs_total"`
	PIIScansTotal  uint64 `json:"rust_pii_scans_total"`
	CryptoOpsTotal uint64 `json:"rust_crypto_ops_total"`
}

// GetRustMetrics fetches all Rust-side metrics as a struct.
func GetRustMetrics() (RustMetrics, error) {
	var outPtr *C.uint8_t
	var outLen C.size_t

	rc := C.argus_metrics_get(&outPtr, &outLen)
	if rc != 0 {
		return RustMetrics{}, fmt.Errorf("argus_metrics_get failed: rc=%d", rc)
	}
	defer C.argus_free_buffer(outPtr, outLen)

	jsonBytes := C.GoBytes(unsafe.Pointer(outPtr), C.int(outLen))
	var m RustMetrics
	if err := json.Unmarshal(jsonBytes, &m); err != nil {
		return RustMetrics{}, fmt.Errorf("failed to parse rust metrics JSON: %w", err)
	}
	return m, nil
}

// ResetRustMetrics resets all Rust-side counters to zero.
func ResetRustMetrics() {
	C.argus_metrics_reset()
}

// RenderRustMetrics returns Prometheus-format text for Rust-side counters.
func RenderRustMetrics() string {
	m, err := GetRustMetrics()
	if err != nil {
		return "" // silently skip if Rust metrics unavailable
	}
	return fmt.Sprintf(
		"# HELP rust_frames_captured_total Total frames captured by Rust\n"+
			"# TYPE rust_frames_captured_total counter\n"+
			"rust_frames_captured_total %d\n"+
			"# HELP rust_resizes_total Total image resizes by Rust\n"+
			"# TYPE rust_resizes_total counter\n"+
			"rust_resizes_total %d\n"+
			"# HELP rust_shm_writes_total Total SHM writes by Rust\n"+
			"# TYPE rust_shm_writes_total counter\n"+
			"rust_shm_writes_total %d\n"+
			"# HELP rust_keyframe_diffs_total Total keyframe diffs by Rust\n"+
			"# TYPE rust_keyframe_diffs_total counter\n"+
			"rust_keyframe_diffs_total %d\n"+
			"# HELP rust_pii_scans_total Total PII scans by Rust\n"+
			"# TYPE rust_pii_scans_total counter\n"+
			"rust_pii_scans_total %d\n"+
			"# HELP rust_crypto_ops_total Total crypto operations by Rust\n"+
			"# TYPE rust_crypto_ops_total counter\n"+
			"rust_crypto_ops_total %d\n",
		m.FramesCaptured, m.ResizesTotal, m.ShmWritesTotal,
		m.KeyframeDiffs, m.PIIScansTotal, m.CryptoOpsTotal,
	)
}
