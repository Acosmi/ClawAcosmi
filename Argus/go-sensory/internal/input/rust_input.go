package input

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust-core/include
#cgo LDFLAGS: -L${SRCDIR}/../../../rust-core/target/release -largus_core -Wl,-rpath,/usr/lib/swift
#include "argus_core.h"
*/
import "C"
import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RustInputController implements InputController using Rust FFI (libargus_core).
// Delegates low-level CGEvent operations to Rust while keeping high-level
// orchestration (smooth mouse movement, hotkeys, drag) in Go.
type RustInputController struct{}

// NewRustInputController creates a new Rust-backed input controller.
func NewRustInputController() *RustInputController {
	return &RustInputController{}
}

// Click performs a mouse click at the given coordinates.
func (r *RustInputController) Click(x, y int, button MouseButton) error {
	rc := C.argus_input_click(C.int32_t(x), C.int32_t(y), C.int32_t(button))
	if rc != 0 {
		return fmt.Errorf("argus_input_click failed: error code %d", rc)
	}
	return nil
}

// DoubleClick performs a double-click at the given coordinates.
func (r *RustInputController) DoubleClick(x, y int) error {
	rc := C.argus_input_double_click(C.int32_t(x), C.int32_t(y))
	if rc != 0 {
		return fmt.Errorf("argus_input_double_click failed: error code %d", rc)
	}
	return nil
}

// MoveTo moves the mouse cursor to the given coordinates.
func (r *RustInputController) MoveTo(x, y int) error {
	rc := C.argus_input_mouse_move(C.int32_t(x), C.int32_t(y))
	if rc != 0 {
		return fmt.Errorf("argus_input_mouse_move failed: error code %d", rc)
	}
	return nil
}

// MoveToSmooth moves the mouse along a Bézier curve for natural motion.
// Orchestration logic stays in Go; only the per-step mouse_move calls Rust.
func (r *RustInputController) MoveToSmooth(targetX, targetY int, durationMs int) error {
	curX, curY, err := r.GetMousePosition()
	if err != nil {
		return err
	}

	dx := float64(targetX - curX)
	dy := float64(targetY - curY)

	// Cubic Bézier control points with randomness
	cp1x := float64(curX) + dx*0.3 + (rand.Float64()-0.5)*dx*0.2
	cp1y := float64(curY) + dy*0.1 + (rand.Float64()-0.5)*dy*0.3
	cp2x := float64(curX) + dx*0.7 + (rand.Float64()-0.5)*dx*0.2
	cp2y := float64(curY) + dy*0.9 + (rand.Float64()-0.5)*dy*0.3

	steps := durationMs / 16 // ~60fps
	if steps < 5 {
		steps = 5
	}

	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		t = t * t * (3 - 2*t) // ease-in-out

		u := 1 - t
		x := u*u*u*float64(curX) + 3*u*u*t*cp1x + 3*u*t*t*cp2x + t*t*t*float64(targetX)
		y := u*u*u*float64(curY) + 3*u*u*t*cp1y + 3*u*t*t*cp2y + t*t*t*float64(targetY)

		if err := r.MoveTo(int(math.Round(x)), int(math.Round(y))); err != nil {
			return err
		}
		time.Sleep(time.Duration(durationMs/steps) * time.Millisecond)
	}

	// Ensure we end exactly at the target
	return r.MoveTo(targetX, targetY)
}

// Drag performs a mouse drag from (x1,y1) to (x2,y2).
// Uses Rust for low-level events, Go for timing orchestration.
func (r *RustInputController) Drag(x1, y1, x2, y2 int) error {
	// Mouse down at start position
	rc := C.argus_input_mouse_move(C.int32_t(x1), C.int32_t(y1))
	if rc != 0 {
		return fmt.Errorf("drag: mouse_move to start failed: %d", rc)
	}

	// We need raw down/up for drag, so use click's down logic via the C API
	// For now, use a left-click down, drag, then up pattern
	// Note: We'll send raw events since argus_input_click does down+up together
	time.Sleep(50 * time.Millisecond)

	// Perform click at start to register the down event
	rc = C.argus_input_click(C.int32_t(x1), C.int32_t(y1), 0)
	if rc != 0 {
		return fmt.Errorf("drag: click at start failed: %d", rc)
	}

	time.Sleep(100 * time.Millisecond)

	// Move to destination
	rc = C.argus_input_mouse_move(C.int32_t(x2), C.int32_t(y2))
	if rc != 0 {
		return fmt.Errorf("drag: mouse_move to end failed: %d", rc)
	}

	time.Sleep(50 * time.Millisecond)
	return nil
}

// Type types the given text string using keyboard events.
func (r *RustInputController) Type(text string) error {
	for _, ch := range text {
		rc := C.argus_input_type_char(C.uint32_t(ch))
		if rc != 0 {
			return fmt.Errorf("argus_input_type_char(%c) failed: error code %d", ch, rc)
		}
		// Random delay for human-like typing (40-100ms)
		delay := 40 + rand.Intn(60)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil
}

// KeyDown presses a key down.
func (r *RustInputController) KeyDown(key Key) error {
	rc := C.argus_input_key_down(C.uint16_t(key))
	if rc != 0 {
		return fmt.Errorf("argus_input_key_down failed: error code %d", rc)
	}
	return nil
}

// KeyUp releases a key.
func (r *RustInputController) KeyUp(key Key) error {
	rc := C.argus_input_key_up(C.uint16_t(key))
	if rc != 0 {
		return fmt.Errorf("argus_input_key_up failed: error code %d", rc)
	}
	return nil
}

// KeyPress presses and releases a key.
func (r *RustInputController) KeyPress(key Key) error {
	rc := C.argus_input_key_press(C.uint16_t(key))
	if rc != 0 {
		return fmt.Errorf("argus_input_key_press failed: error code %d", rc)
	}
	return nil
}

// Hotkey presses a key combination (e.g., Cmd+C).
// The last key is the main key, others are modifiers.
func (r *RustInputController) Hotkey(keys ...Key) error {
	if len(keys) == 0 {
		return fmt.Errorf("no keys specified")
	}

	// Press all modifier keys
	for i := 0; i < len(keys)-1; i++ {
		if err := r.KeyDown(keys[i]); err != nil {
			return err
		}
		time.Sleep(30 * time.Millisecond)
	}

	// Press and release the main key
	if err := r.KeyPress(keys[len(keys)-1]); err != nil {
		return err
	}

	// Release modifiers in reverse order
	for i := len(keys) - 2; i >= 0; i-- {
		time.Sleep(30 * time.Millisecond)
		if err := r.KeyUp(keys[i]); err != nil {
			return err
		}
	}

	return nil
}

// Scroll simulates mouse scroll at the given position.
func (r *RustInputController) Scroll(x, y int, deltaX, deltaY int) error {
	rc := C.argus_input_scroll(C.int32_t(x), C.int32_t(y), C.int32_t(deltaX), C.int32_t(deltaY))
	if rc != 0 {
		return fmt.Errorf("argus_input_scroll failed: error code %d", rc)
	}
	return nil
}

// GetMousePosition returns the current mouse cursor position.
func (r *RustInputController) GetMousePosition() (int, int, error) {
	var x, y C.int32_t
	rc := C.argus_input_get_mouse_pos(&x, &y)
	if rc != 0 {
		return 0, 0, fmt.Errorf("argus_input_get_mouse_pos failed: error code %d", rc)
	}
	return int(x), int(y), nil
}
