package agent

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust-core/include
#cgo LDFLAGS: -L${SRCDIR}/../../../rust-core/target/release -largus_core -Wl,-rpath,/usr/lib/swift
#include "argus_core.h"
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"log"
	"unsafe"
)

// RustAccessibility provides Go bindings to the Rust AXUIElement module.
// It exposes macOS Accessibility API via FFI for zero-latency UI element detection.
type RustAccessibility struct{}

// NewRustAccessibility creates a new Rust-backed accessibility client.
func NewRustAccessibility() *RustAccessibility {
	return &RustAccessibility{}
}

// axRawElement mirrors the JSON output from Rust accessibility module.
type axRawElement struct {
	Role         string `json:"role"`
	Label        string `json:"label"`
	X1           int    `json:"x1"`
	Y1           int    `json:"y1"`
	X2           int    `json:"x2"`
	Y2           int    `json:"y2"`
	Interactable bool   `json:"interactable"`
}

// parseAXJSON deserializes Rust JSON output into UIElement slice.
func parseAXJSON(jsonBytes []byte) ([]UIElement, error) {
	var raw []axRawElement
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		return nil, fmt.Errorf("parse AX JSON: %w", err)
	}

	elements := make([]UIElement, 0, len(raw))
	for i, el := range raw {
		elements = append(elements, UIElement{
			ID:           i,
			Type:         ParseElementType(el.Role),
			Label:        el.Label,
			X1:           el.X1,
			Y1:           el.Y1,
			X2:           el.X2,
			Y2:           el.Y2,
			Confidence:   1.0, // AX provides exact coordinates
			Interactable: el.Interactable,
		})
	}
	return elements, nil
}

// ListElements returns all UI elements for the given process ID.
func (r *RustAccessibility) ListElements(pid int) ([]UIElement, error) {
	var outPtr *C.uint8_t
	var outLen C.size_t

	rc := C.argus_ax_list_elements(C.int32_t(pid), &outPtr, &outLen)
	if rc != 0 {
		return nil, fmt.Errorf("argus_ax_list_elements failed: error code %d", rc)
	}
	defer C.argus_free_buffer((*C.uint8_t)(outPtr), outLen)

	jsonBytes := C.GoBytes(unsafe.Pointer(outPtr), C.int(outLen))
	elements, err := parseAXJSON(jsonBytes)
	if err != nil {
		return nil, err
	}

	log.Printf("[AX] ListElements(pid=%d): %d elements", pid, len(elements))
	return elements, nil
}

// FocusedAppElements returns all UI elements of the currently focused app.
func (r *RustAccessibility) FocusedAppElements() ([]UIElement, error) {
	var outPtr *C.uint8_t
	var outLen C.size_t

	rc := C.argus_ax_focused_app(&outPtr, &outLen)
	if rc != 0 {
		return nil, fmt.Errorf("argus_ax_focused_app failed: error code %d", rc)
	}
	defer C.argus_free_buffer((*C.uint8_t)(outPtr), outLen)

	jsonBytes := C.GoBytes(unsafe.Pointer(outPtr), C.int(outLen))
	elements, err := parseAXJSON(jsonBytes)
	if err != nil {
		return nil, err
	}

	log.Printf("[AX] FocusedAppElements: %d elements", len(elements))
	return elements, nil
}

// ElementAtPosition returns the UI element at the given screen coordinates.
func (r *RustAccessibility) ElementAtPosition(x, y int) (*UIElement, error) {
	var outPtr *C.uint8_t
	var outLen C.size_t

	rc := C.argus_ax_element_at_position(C.float(x), C.float(y), &outPtr, &outLen)
	if rc != 0 {
		return nil, fmt.Errorf("argus_ax_element_at_position failed: error code %d", rc)
	}
	defer C.argus_free_buffer((*C.uint8_t)(outPtr), outLen)

	jsonBytes := C.GoBytes(unsafe.Pointer(outPtr), C.int(outLen))
	elements, err := parseAXJSON(jsonBytes)
	if err != nil {
		return nil, err
	}

	if len(elements) == 0 {
		return nil, nil
	}
	return &elements[0], nil
}

// PermissionStatus holds macOS permission check results.
type PermissionStatus struct {
	Accessibility   bool `json:"accessibility"`
	ScreenRecording bool `json:"screen_recording"`
}

// CheckPermissions checks Accessibility and Screen Recording permissions.
func (r *RustAccessibility) CheckPermissions() PermissionStatus {
	var outPtr *C.uint8_t
	var outLen C.size_t

	rc := C.argus_check_permissions(&outPtr, &outLen)
	if rc != 0 {
		log.Printf("[AX] argus_check_permissions failed: error code %d", rc)
		return PermissionStatus{}
	}
	defer C.argus_free_buffer((*C.uint8_t)(outPtr), outLen)

	jsonBytes := C.GoBytes(unsafe.Pointer(outPtr), C.int(outLen))
	var status PermissionStatus
	if err := json.Unmarshal(jsonBytes, &status); err != nil {
		log.Printf("[AX] parse permissions JSON: %v", err)
		return PermissionStatus{}
	}

	log.Printf("[AX] Permissions: accessibility=%v, screen_recording=%v",
		status.Accessibility, status.ScreenRecording)
	return status
}

// RequestScreenCapture explicitly triggers the system Screen Recording permission dialog.
func (r *RustAccessibility) RequestScreenCapture() bool {
	return bool(C.argus_request_screen_capture())
}
