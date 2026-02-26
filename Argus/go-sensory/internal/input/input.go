package input

// Key represents a keyboard key code.
type Key uint16

// Common key codes (CGKeyCode values for macOS).
const (
	KeyA         Key = 0x00
	KeyS         Key = 0x01
	KeyD         Key = 0x02
	KeyF         Key = 0x03
	KeyH         Key = 0x04
	KeyG         Key = 0x05
	KeyZ         Key = 0x06
	KeyX         Key = 0x07
	KeyC         Key = 0x08
	KeyV         Key = 0x09
	KeyB         Key = 0x0B
	KeyQ         Key = 0x0C
	KeyW         Key = 0x0D
	KeyE         Key = 0x0E
	KeyR         Key = 0x0F
	KeyY         Key = 0x10
	KeyT         Key = 0x11
	KeyO         Key = 0x1F
	KeyU         Key = 0x20
	KeyI         Key = 0x22
	KeyP         Key = 0x23
	KeyL         Key = 0x25
	KeyJ         Key = 0x26
	KeyK         Key = 0x28
	KeyN         Key = 0x2D
	KeyM         Key = 0x2E
	KeyReturn    Key = 0x24
	KeyTab       Key = 0x30
	KeySpace     Key = 0x31
	KeyDelete    Key = 0x33
	KeyEscape    Key = 0x35
	KeyCommand   Key = 0x37
	KeyShift     Key = 0x38
	KeyCapsLock  Key = 0x39
	KeyOption    Key = 0x3A
	KeyControl   Key = 0x3B
	KeyArrowLeft  Key = 0x7B
	KeyArrowRight Key = 0x7C
	KeyArrowDown  Key = 0x7D
	KeyArrowUp    Key = 0x7E
)

// MouseButton represents mouse buttons.
type MouseButton int

const (
	MouseLeft   MouseButton = 0
	MouseRight  MouseButton = 1
	MouseMiddle MouseButton = 2
)

// InputController defines the interface for simulated input across platforms.
type InputController interface {
	// Click performs a mouse click at the given screen coordinates.
	Click(x, y int, button MouseButton) error

	// DoubleClick performs a double-click at the given coordinates.
	DoubleClick(x, y int) error

	// MoveTo moves the mouse cursor to the given coordinates.
	MoveTo(x, y int) error

	// MoveToSmooth moves the mouse along a natural curve (anti-detection).
	MoveToSmooth(x, y int, durationMs int) error

	// Drag performs a mouse drag from (x1,y1) to (x2,y2).
	Drag(x1, y1, x2, y2 int) error

	// Type types the given text string using keyboard events.
	Type(text string) error

	// KeyDown presses a key down (without releasing).
	KeyDown(key Key) error

	// KeyUp releases a key.
	KeyUp(key Key) error

	// KeyPress presses and releases a key.
	KeyPress(key Key) error

	// Hotkey presses a key combination (e.g., Cmd+C).
	Hotkey(keys ...Key) error

	// Scroll simulates mouse scroll at the given position.
	Scroll(x, y int, deltaX, deltaY int) error

	// GetMousePosition returns the current mouse cursor position.
	GetMousePosition() (x, y int, err error)
}
