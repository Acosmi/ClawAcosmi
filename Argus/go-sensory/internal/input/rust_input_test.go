package input

import (
	"testing"
)

// TestRustInputController_ImplementsInterface is a compile-time check
// that RustInputController satisfies the InputController interface.
func TestRustInputController_ImplementsInterface(t *testing.T) {
	var _ InputController = (*RustInputController)(nil)
	t.Log("RustInputController implements InputController")
}

// TestRustInputController_GetMousePosition verifies the FFI call succeeds
// and returns reasonable coordinates (non-negative).
func TestRustInputController_GetMousePosition(t *testing.T) {
	ctrl := NewRustInputController()
	x, y, err := ctrl.GetMousePosition()
	if err != nil {
		t.Fatalf("GetMousePosition() error: %v", err)
	}
	// Coordinates should be non-negative on any display
	if x < 0 || y < 0 {
		t.Errorf("GetMousePosition() = (%d, %d), expected non-negative", x, y)
	}
	t.Logf("Mouse position: (%d, %d)", x, y)
}

// TestRustInputController_MoveTo verifies mouse move FFI call.
// Moves to a safe position (100, 100) and reads back.
func TestRustInputController_MoveTo(t *testing.T) {
	ctrl := NewRustInputController()
	err := ctrl.MoveTo(100, 100)
	if err != nil {
		t.Fatalf("MoveTo(100, 100) error: %v", err)
	}
	// Give the event time to propagate
	x, y, err := ctrl.GetMousePosition()
	if err != nil {
		t.Fatalf("GetMousePosition() after MoveTo error: %v", err)
	}
	t.Logf("After MoveTo(100,100): position = (%d, %d)", x, y)
}
