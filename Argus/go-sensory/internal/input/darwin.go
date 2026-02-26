package input

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework ApplicationServices

#include <CoreGraphics/CoreGraphics.h>
#include <ApplicationServices/ApplicationServices.h>
#include <unistd.h>

// Move mouse to absolute position
void mouse_move(int x, int y) {
    CGEventRef event = CGEventCreateMouseEvent(
        NULL, kCGEventMouseMoved,
        CGPointMake(x, y), kCGMouseButtonLeft
    );
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

// Mouse button down
void mouse_down(int x, int y, int button) {
    CGEventType eventType;
    CGMouseButton mouseButton;

    switch (button) {
        case 1:  // right
            eventType = kCGEventRightMouseDown;
            mouseButton = kCGMouseButtonRight;
            break;
        case 2:  // middle
            eventType = kCGEventOtherMouseDown;
            mouseButton = kCGMouseButtonCenter;
            break;
        default: // left
            eventType = kCGEventLeftMouseDown;
            mouseButton = kCGMouseButtonLeft;
            break;
    }

    CGEventRef event = CGEventCreateMouseEvent(
        NULL, eventType, CGPointMake(x, y), mouseButton
    );
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

// Mouse button up
void mouse_up(int x, int y, int button) {
    CGEventType eventType;
    CGMouseButton mouseButton;

    switch (button) {
        case 1:
            eventType = kCGEventRightMouseUp;
            mouseButton = kCGMouseButtonRight;
            break;
        case 2:
            eventType = kCGEventOtherMouseUp;
            mouseButton = kCGMouseButtonCenter;
            break;
        default:
            eventType = kCGEventLeftMouseUp;
            mouseButton = kCGMouseButtonLeft;
            break;
    }

    CGEventRef event = CGEventCreateMouseEvent(
        NULL, eventType, CGPointMake(x, y), mouseButton
    );
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

// Click at position (down + delay + up)
void mouse_click(int x, int y, int button) {
    mouse_down(x, y, button);
    usleep(50000);  // 50ms delay between down and up
    mouse_up(x, y, button);
}

// Double click
void mouse_double_click(int x, int y) {
    CGEventRef event1 = CGEventCreateMouseEvent(
        NULL, kCGEventLeftMouseDown, CGPointMake(x, y), kCGMouseButtonLeft
    );
    CGEventSetIntegerValueField(event1, kCGMouseEventClickState, 1);
    CGEventPost(kCGHIDEventTap, event1);
    CFRelease(event1);

    CGEventRef event2 = CGEventCreateMouseEvent(
        NULL, kCGEventLeftMouseUp, CGPointMake(x, y), kCGMouseButtonLeft
    );
    CGEventSetIntegerValueField(event2, kCGMouseEventClickState, 1);
    CGEventPost(kCGHIDEventTap, event2);
    CFRelease(event2);

    usleep(50000); // 50ms

    CGEventRef event3 = CGEventCreateMouseEvent(
        NULL, kCGEventLeftMouseDown, CGPointMake(x, y), kCGMouseButtonLeft
    );
    CGEventSetIntegerValueField(event3, kCGMouseEventClickState, 2);
    CGEventPost(kCGHIDEventTap, event3);
    CFRelease(event3);

    CGEventRef event4 = CGEventCreateMouseEvent(
        NULL, kCGEventLeftMouseUp, CGPointMake(x, y), kCGMouseButtonLeft
    );
    CGEventSetIntegerValueField(event4, kCGMouseEventClickState, 2);
    CGEventPost(kCGHIDEventTap, event4);
    CFRelease(event4);
}

// Mouse drag
void mouse_drag(int x1, int y1, int x2, int y2) {
    mouse_down(x1, y1, 0);
    usleep(100000); // 100ms

    CGEventRef dragEvent = CGEventCreateMouseEvent(
        NULL, kCGEventLeftMouseDragged,
        CGPointMake(x2, y2), kCGMouseButtonLeft
    );
    CGEventPost(kCGHIDEventTap, dragEvent);
    CFRelease(dragEvent);

    usleep(50000); // 50ms
    mouse_up(x2, y2, 0);
}

// Scroll wheel
void mouse_scroll(int x, int y, int deltaX, int deltaY) {
    mouse_move(x, y);
    usleep(10000);

    CGEventRef event = CGEventCreateScrollWheelEvent(
        NULL, kCGScrollEventUnitPixel, 2, deltaY, deltaX
    );
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

// Key down
void key_down(int keyCode) {
    CGEventRef event = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, true);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

// Key up
void key_up(int keyCode) {
    CGEventRef event = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, false);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);
}

// Key press (down + up)
void key_press(int keyCode) {
    key_down(keyCode);
    usleep(30000); // 30ms
    key_up(keyCode);
}

// Type a Unicode character
void type_char(int charCode) {
    CGEventRef event = CGEventCreateKeyboardEvent(NULL, 0, true);
    UniChar chars[1] = { (UniChar)charCode };
    CGEventKeyboardSetUnicodeString(event, 1, chars);
    CGEventPost(kCGHIDEventTap, event);
    CFRelease(event);

    CGEventRef eventUp = CGEventCreateKeyboardEvent(NULL, 0, false);
    CGEventPost(kCGHIDEventTap, eventUp);
    CFRelease(eventUp);
}

// Get current mouse position
void get_mouse_pos(int* outX, int* outY) {
    CGEventRef event = CGEventCreate(NULL);
    CGPoint point = CGEventGetLocation(event);
    *outX = (int)point.x;
    *outY = (int)point.y;
    CFRelease(event);
}
*/
import "C"

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// DarwinInputController implements InputController for macOS.
type DarwinInputController struct{}

// NewDarwinInputController creates a new macOS input controller.
func NewDarwinInputController() *DarwinInputController {
	return &DarwinInputController{}
}

// Click performs a mouse click at the given coordinates.
func (d *DarwinInputController) Click(x, y int, button MouseButton) error {
	C.mouse_click(C.int(x), C.int(y), C.int(button))
	return nil
}

// DoubleClick performs a double-click.
func (d *DarwinInputController) DoubleClick(x, y int) error {
	C.mouse_double_click(C.int(x), C.int(y))
	return nil
}

// MoveTo moves the cursor instantly.
func (d *DarwinInputController) MoveTo(x, y int) error {
	C.mouse_move(C.int(x), C.int(y))
	return nil
}

// MoveToSmooth moves the mouse along a Bézier curve for natural motion.
func (d *DarwinInputController) MoveToSmooth(targetX, targetY int, durationMs int) error {
	curX, curY, err := d.GetMousePosition()
	if err != nil {
		return err
	}

	// Generate Bézier control points with some randomness
	dx := float64(targetX - curX)
	dy := float64(targetY - curY)

	// Two control points for a cubic Bézier curve
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
		// Apply easing (ease-in-out)
		t = t * t * (3 - 2*t)

		// Cubic Bézier interpolation
		u := 1 - t
		x := u*u*u*float64(curX) + 3*u*u*t*cp1x + 3*u*t*t*cp2x + t*t*t*float64(targetX)
		y := u*u*u*float64(curY) + 3*u*u*t*cp1y + 3*u*t*t*cp2y + t*t*t*float64(targetY)

		C.mouse_move(C.int(math.Round(x)), C.int(math.Round(y)))
		time.Sleep(time.Duration(durationMs/steps) * time.Millisecond)
	}

	// Ensure we end exactly at the target
	C.mouse_move(C.int(targetX), C.int(targetY))
	return nil
}

// Drag performs a mouse drag operation.
func (d *DarwinInputController) Drag(x1, y1, x2, y2 int) error {
	C.mouse_drag(C.int(x1), C.int(y1), C.int(x2), C.int(y2))
	return nil
}

// Type types a string of text character by character.
func (d *DarwinInputController) Type(text string) error {
	for _, ch := range text {
		C.type_char(C.int(ch))
		// Random delay between keystrokes (40-100ms) for human-like typing
		delay := 40 + rand.Intn(60)
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil
}

// KeyDown presses a key.
func (d *DarwinInputController) KeyDown(key Key) error {
	C.key_down(C.int(key))
	return nil
}

// KeyUp releases a key.
func (d *DarwinInputController) KeyUp(key Key) error {
	C.key_up(C.int(key))
	return nil
}

// KeyPress presses and releases a key.
func (d *DarwinInputController) KeyPress(key Key) error {
	C.key_press(C.int(key))
	return nil
}

// Hotkey presses a key combination (e.g., Cmd+C).
// The last key is considered the main key, others are modifiers.
func (d *DarwinInputController) Hotkey(keys ...Key) error {
	if len(keys) == 0 {
		return fmt.Errorf("no keys specified")
	}

	// Press all modifier keys
	for i := 0; i < len(keys)-1; i++ {
		C.key_down(C.int(keys[i]))
		time.Sleep(30 * time.Millisecond)
	}

	// Press and release the main key
	mainKey := keys[len(keys)-1]
	C.key_press(C.int(mainKey))

	// Release modifiers in reverse order
	for i := len(keys) - 2; i >= 0; i-- {
		time.Sleep(30 * time.Millisecond)
		C.key_up(C.int(keys[i]))
	}

	return nil
}

// Scroll simulates mouse scroll.
func (d *DarwinInputController) Scroll(x, y int, deltaX, deltaY int) error {
	C.mouse_scroll(C.int(x), C.int(y), C.int(deltaX), C.int(deltaY))
	return nil
}

// GetMousePosition returns the current cursor position.
func (d *DarwinInputController) GetMousePosition() (int, int, error) {
	var x, y C.int
	C.get_mouse_pos(&x, &y)
	return int(x), int(y), nil
}

// NewInputController creates a platform-specific input controller.
func NewInputController() InputController {
	return NewDarwinInputController()
}
