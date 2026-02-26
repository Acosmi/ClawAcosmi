//! Input injection module using CoreGraphics CGEvent API.
//!
//! Provides C ABI functions for simulating mouse and keyboard input on macOS.
//! Requires Accessibility permission (System Settings → Privacy → Accessibility).

use core_graphics::event::{
    CGEvent, CGEventTapLocation, CGEventType, CGKeyCode, CGMouseButton,
    EventField,
};
use core_graphics::event_source::{CGEventSource, CGEventSourceStateID};
use core_graphics::geometry::CGPoint;

use crate::{ARGUS_ERR_INTERNAL, ARGUS_ERR_INVALID_PARAM, ARGUS_ERR_NULL_PTR, ARGUS_OK};

// Raw CoreGraphics FFI for scroll wheel (not exposed by core-graphics crate)
unsafe extern "C" {
    fn CGEventCreateScrollWheelEvent(
        source: *const std::ffi::c_void,
        units: u32,
        wheel_count: u32,
        wheel1: i32,
        wheel2: i32,
    ) -> *mut std::ffi::c_void;
    fn CGEventPost(tap: u32, event: *mut std::ffi::c_void);
    fn CFRelease(cf: *mut std::ffi::c_void);
}

// kCGScrollEventUnitPixel = 1, kCGHIDEventTap = 0
const CG_SCROLL_EVENT_UNIT_PIXEL: u32 = 1;
const CG_HID_EVENT_TAP: u32 = 0;

// ===== Mouse Operations =====

/// Move mouse cursor to absolute position (x, y).
///
/// # Safety
/// No pointer parameters — always safe to call.
#[unsafe(no_mangle)]
pub extern "C" fn argus_input_mouse_move(x: i32, y: i32) -> i32 {
    let source = match CGEventSource::new(CGEventSourceStateID::HIDSystemState)
    {
        Ok(s) => s,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    let event = match CGEvent::new_mouse_event(
        source,
        CGEventType::MouseMoved,
        CGPoint::new(x as f64, y as f64),
        CGMouseButton::Left,
    ) {
        Ok(e) => e,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    event.post(CGEventTapLocation::HID);
    ARGUS_OK
}

/// Perform a mouse click at (x, y) with the given button.
/// button: 0 = left, 1 = right, 2 = middle.
///
/// # Safety
/// No pointer parameters — always safe to call.
#[unsafe(no_mangle)]
pub extern "C" fn argus_input_click(x: i32, y: i32, button: i32) -> i32 {
    let (down_type, up_type, cg_button) = match button {
        0 => (
            CGEventType::LeftMouseDown,
            CGEventType::LeftMouseUp,
            CGMouseButton::Left,
        ),
        1 => (
            CGEventType::RightMouseDown,
            CGEventType::RightMouseUp,
            CGMouseButton::Right,
        ),
        2 => (
            CGEventType::OtherMouseDown,
            CGEventType::OtherMouseUp,
            CGMouseButton::Center,
        ),
        _ => return ARGUS_ERR_INVALID_PARAM,
    };

    let point = CGPoint::new(x as f64, y as f64);
    let source = match CGEventSource::new(CGEventSourceStateID::HIDSystemState)
    {
        Ok(s) => s,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };

    // Mouse down
    let down = match CGEvent::new_mouse_event(
        source.clone(),
        down_type,
        point,
        cg_button,
    ) {
        Ok(e) => e,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    down.post(CGEventTapLocation::HID);

    // 50ms delay
    std::thread::sleep(std::time::Duration::from_millis(50));

    // Mouse up
    let source2 =
        match CGEventSource::new(CGEventSourceStateID::HIDSystemState) {
            Ok(s) => s,
            Err(_) => return ARGUS_ERR_INTERNAL,
        };
    let up = match CGEvent::new_mouse_event(source2, up_type, point, cg_button)
    {
        Ok(e) => e,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    up.post(CGEventTapLocation::HID);

    ARGUS_OK
}

/// Perform a double-click at (x, y) with left button.
///
/// # Safety
/// No pointer parameters.
#[unsafe(no_mangle)]
pub extern "C" fn argus_input_double_click(x: i32, y: i32) -> i32 {
    let point = CGPoint::new(x as f64, y as f64);

    // Click 1: down + up with clickState=1
    for click_state in [1i64, 2i64] {
        let source =
            match CGEventSource::new(CGEventSourceStateID::HIDSystemState) {
                Ok(s) => s,
                Err(_) => return ARGUS_ERR_INTERNAL,
            };
        let down = match CGEvent::new_mouse_event(
            source.clone(),
            CGEventType::LeftMouseDown,
            point,
            CGMouseButton::Left,
        ) {
            Ok(e) => e,
            Err(_) => return ARGUS_ERR_INTERNAL,
        };
        down.set_integer_value_field(
            EventField::MOUSE_EVENT_CLICK_STATE,
            click_state,
        );
        down.post(CGEventTapLocation::HID);

        let source2 =
            match CGEventSource::new(CGEventSourceStateID::HIDSystemState) {
                Ok(s) => s,
                Err(_) => return ARGUS_ERR_INTERNAL,
            };
        let up = match CGEvent::new_mouse_event(
            source2,
            CGEventType::LeftMouseUp,
            point,
            CGMouseButton::Left,
        ) {
            Ok(e) => e,
            Err(_) => return ARGUS_ERR_INTERNAL,
        };
        up.set_integer_value_field(
            EventField::MOUSE_EVENT_CLICK_STATE,
            click_state,
        );
        up.post(CGEventTapLocation::HID);

        if click_state == 1 {
            std::thread::sleep(std::time::Duration::from_millis(50));
        }
    }

    ARGUS_OK
}

/// Perform a mouse scroll at (x, y).
///
/// Uses raw CoreGraphics FFI because the `core-graphics` crate (0.24)
/// does not expose `CGEventCreateScrollWheelEvent`.
///
/// # Safety
/// No pointer parameters.
#[unsafe(no_mangle)]
pub extern "C" fn argus_input_scroll(
    x: i32,
    y: i32,
    delta_x: i32,
    delta_y: i32,
) -> i32 {
    // Move to position first
    let rc = argus_input_mouse_move(x, y);
    if rc != ARGUS_OK {
        return rc;
    }
    std::thread::sleep(std::time::Duration::from_millis(10));

    // SAFETY: CGEventCreateScrollWheelEvent is a well-defined CoreGraphics API.
    // NULL source is valid (uses the default event source).
    // The returned event must be posted and then released.
    unsafe {
        let event = CGEventCreateScrollWheelEvent(
            std::ptr::null(),
            CG_SCROLL_EVENT_UNIT_PIXEL,
            2, // wheelCount: 2 axes (vertical + horizontal)
            delta_y,
            delta_x,
        );
        if event.is_null() {
            return ARGUS_ERR_INTERNAL;
        }
        CGEventPost(CG_HID_EVENT_TAP, event);
        CFRelease(event);
    }

    ARGUS_OK
}

// ===== Keyboard Operations =====

/// Press a key down (without releasing).
///
/// # Safety
/// No pointer parameters.
#[unsafe(no_mangle)]
pub extern "C" fn argus_input_key_down(key_code: u16) -> i32 {
    let source = match CGEventSource::new(CGEventSourceStateID::HIDSystemState)
    {
        Ok(s) => s,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    let event = match CGEvent::new_keyboard_event(
        source,
        key_code as CGKeyCode,
        true,
    ) {
        Ok(e) => e,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    event.post(CGEventTapLocation::HID);
    ARGUS_OK
}

/// Release a key.
///
/// # Safety
/// No pointer parameters.
#[unsafe(no_mangle)]
pub extern "C" fn argus_input_key_up(key_code: u16) -> i32 {
    let source = match CGEventSource::new(CGEventSourceStateID::HIDSystemState)
    {
        Ok(s) => s,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    let event = match CGEvent::new_keyboard_event(
        source,
        key_code as CGKeyCode,
        false,
    ) {
        Ok(e) => e,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    event.post(CGEventTapLocation::HID);
    ARGUS_OK
}

/// Press and release a key.
///
/// # Safety
/// No pointer parameters.
#[unsafe(no_mangle)]
pub extern "C" fn argus_input_key_press(key_code: u16) -> i32 {
    let rc = argus_input_key_down(key_code);
    if rc != ARGUS_OK {
        return rc;
    }
    std::thread::sleep(std::time::Duration::from_millis(30));
    argus_input_key_up(key_code)
}

/// Type a single Unicode character.
///
/// # Safety
/// No pointer parameters.
#[unsafe(no_mangle)]
pub extern "C" fn argus_input_type_char(char_code: u32) -> i32 {
    let source = match CGEventSource::new(CGEventSourceStateID::HIDSystemState)
    {
        Ok(s) => s,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    let event =
        match CGEvent::new_keyboard_event(source.clone(), 0 as CGKeyCode, true)
        {
            Ok(e) => e,
            Err(_) => return ARGUS_ERR_INTERNAL,
        };

    if let Some(ch) = char::from_u32(char_code) {
        let mut buf = [0u16; 2];
        let encoded = ch.encode_utf16(&mut buf);
        event.set_string_from_utf16_unchecked(encoded);
    } else {
        return ARGUS_ERR_INVALID_PARAM;
    }
    event.post(CGEventTapLocation::HID);

    // Key up
    let event_up =
        match CGEvent::new_keyboard_event(source, 0 as CGKeyCode, false) {
            Ok(e) => e,
            Err(_) => return ARGUS_ERR_INTERNAL,
        };
    event_up.post(CGEventTapLocation::HID);

    ARGUS_OK
}

// ===== Mouse Position Query =====

/// Get current mouse cursor position.
///
/// # Safety
/// `out_x` and `out_y` must be non-null and properly aligned.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_input_get_mouse_pos(
    out_x: *mut i32,
    out_y: *mut i32,
) -> i32 {
    if out_x.is_null() || out_y.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }

    let source = match CGEventSource::new(CGEventSourceStateID::HIDSystemState)
    {
        Ok(s) => s,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    let event = match CGEvent::new(source) {
        Ok(e) => e,
        Err(_) => return ARGUS_ERR_INTERNAL,
    };
    let loc = event.location();

    // SAFETY: pointers verified non-null above
    unsafe {
        *out_x = loc.x as i32;
        *out_y = loc.y as i32;
    }
    ARGUS_OK
}
