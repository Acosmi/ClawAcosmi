package pipeline

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

// RustPIIFilterResult mirrors the Rust PIIFilterResult JSON output.
type RustPIIFilterResult struct {
	OriginalText string         `json:"original_text"`
	FilteredText string         `json:"filtered_text"`
	Matches      []RustPIIMatch `json:"matches"`
	PIIDetected  bool           `json:"pii_detected"`
}

// RustPIIMatch mirrors a single PII detection from Rust.
type RustPIIMatch struct {
	EntityType string  `json:"entity_type"`
	Original   string  `json:"original"`
	Masked     string  `json:"masked"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Confidence float64 `json:"confidence"`
}

// RustPIIFilter calls the Rust PII filter and returns the result.
func RustPIIFilter(text string) (RustPIIFilterResult, error) {
	var result RustPIIFilterResult
	if len(text) == 0 {
		result.OriginalText = text
		result.FilteredText = text
		return result, nil
	}

	textBytes := []byte(text)
	var outPtr *C.uint8_t
	var outLen C.size_t

	rc := C.argus_pii_filter(
		(*C.uint8_t)(unsafe.Pointer(&textBytes[0])),
		C.size_t(len(textBytes)),
		&outPtr,
		&outLen,
	)
	if rc != 0 {
		return result, fmt.Errorf("argus_pii_filter failed: rc=%d", rc)
	}
	defer C.argus_free_buffer(outPtr, outLen)

	jsonBytes := C.GoBytes(unsafe.Pointer(outPtr), C.int(outLen))
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return result, fmt.Errorf("failed to parse PII result JSON: %w", err)
	}

	return result, nil
}

// RustPIIIsSafe returns true if no PII is detected in the text.
func RustPIIIsSafe(text string) bool {
	if len(text) == 0 {
		return true
	}

	textBytes := []byte(text)
	rc := C.argus_pii_is_safe(
		(*C.uint8_t)(unsafe.Pointer(&textBytes[0])),
		C.size_t(len(textBytes)),
	)
	return rc == 0
}
