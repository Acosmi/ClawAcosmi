//! macOS Accessibility (AXUIElement) module.
//!
//! Provides C ABI exports for enumerating UI elements via the native
//! macOS Accessibility API. This replaces VLM-based element detection,
//! reducing latency from ~2s to <5ms with pixel-perfect coordinates.

use crate::{ARGUS_ERR_INTERNAL, ARGUS_ERR_NULL_PTR, ARGUS_OK};
use core_foundation::boolean::CFBoolean;
use core_foundation::array::CFArray;
use core_foundation::base::{CFRange, CFType, TCFType};
use core_foundation::string::CFString;
use serde::Serialize;
use std::ffi::c_void;
use std::ptr;

// ===== CoreFoundation / ApplicationServices FFI =====

#[allow(non_camel_case_types)]
type AXUIElementRef = *const c_void;
#[allow(non_camel_case_types)]
type AXError = i32;

const K_AX_ERROR_SUCCESS: AXError = 0;

// AX attribute keys
const K_AX_ROLE_ATTR: &str = "AXRole";
const K_AX_TITLE_ATTR: &str = "AXTitle";
const K_AX_DESCRIPTION_ATTR: &str = "AXDescription";
const K_AX_POSITION_ATTR: &str = "AXPosition";
const K_AX_SIZE_ATTR: &str = "AXSize";
const K_AX_CHILDREN_ATTR: &str = "AXChildren";
const K_AX_FOCUSED_APP_ATTR: &str = "AXFocusedApplication";
const K_AX_WINDOWS_ATTR: &str = "AXWindows";
const K_AX_BUNDLE_ID_ATTR: &str = "AXBundleIdentifier";

// Known Electron app bundle ID prefixes
const ELECTRON_BUNDLE_PREFIXES: &[&str] = &[
    "com.microsoft.VSCode",
    "com.hnc.Discord",
    "com.slack.",
    "com.spotify.",
    "com.github.GitHubClient",
    "com.tinyspeck.",
    "com.figma.",
    "org.chromium.",
    "com.electron.",
];

// Known browser bundle IDs
const BROWSER_BUNDLE_IDS: &[&str] = &[
    "com.google.Chrome",
    "com.apple.Safari",
    "org.mozilla.firefox",
    "com.microsoft.edgemac",
    "com.brave.Browser",
    "com.operasoftware.Opera",
    "company.thebrowser.Browser",
];

// AX roles that are interactable
const INTERACTABLE_ROLES: &[&str] = &[
    "AXButton",
    "AXTextField",
    "AXTextArea",
    "AXCheckBox",
    "AXRadioButton",
    "AXPopUpButton",
    "AXComboBox",
    "AXSlider",
    "AXLink",
    "AXMenuItem",
    "AXMenuBarItem",
    "AXTab",
    "AXIncrementor",
    "AXColorWell",
    "AXDisclosureTriangle",
];

#[link(name = "ApplicationServices", kind = "framework")]
unsafe extern "C" {
    fn AXUIElementCreateSystemWide() -> AXUIElementRef;
    fn AXUIElementCreateApplication(pid: i32) -> AXUIElementRef;
    fn AXUIElementCopyAttributeValue(
        element: AXUIElementRef,
        attribute: *const c_void, // CFStringRef
        value: *mut *const c_void,
    ) -> AXError;
    fn AXUIElementCopyElementAtPosition(
        application: AXUIElementRef,
        x: f32,
        y: f32,
        element: *mut AXUIElementRef,
    ) -> AXError;
    fn AXIsProcessTrusted() -> bool;
    fn AXIsProcessTrustedWithOptions(options: *const c_void) -> bool;
    fn AXUIElementSetAttributeValue(
        element: AXUIElementRef,
        attribute: *const c_void,
        value: *const c_void,
    ) -> AXError;
}

#[link(name = "CoreFoundation", kind = "framework")]
unsafe extern "C" {
    fn CFRelease(cf: *mut c_void);
    fn CFRetain(cf: *const c_void) -> *const c_void;
    fn CFDictionaryCreate(
        allocator: *const c_void,
        keys: *const *const c_void,
        values: *const *const c_void,
        num_values: isize,
        key_callbacks: *const c_void,
        value_callbacks: *const c_void,
    ) -> *const c_void;
    static kCFTypeDictionaryKeyCallBacks: c_void;
    static kCFTypeDictionaryValueCallBacks: c_void;
    static kCFBooleanTrue: *const c_void;
}

#[link(name = "CoreGraphics", kind = "framework")]
unsafe extern "C" {
    /// macOS 10.15+: check-only, does not trigger dialog.
    fn CGPreflightScreenCaptureAccess() -> bool;
    /// macOS 10.15+: requests permission, may trigger dialog.
    #[allow(dead_code)]
    fn CGRequestScreenCaptureAccess() -> bool;
}

// AXIsProcessTrustedWithOptions key
const K_AX_TRUSTED_CHECK_OPTION_PROMPT: &str = "AXTrustedCheckOptionPrompt";

// ===== Data Structures =====

/// A single UI element as detected by the Accessibility API.
#[derive(Debug, Serialize)]
struct AXElement {
    role: String,
    label: String,
    x1: i32,
    y1: i32,
    x2: i32,
    y2: i32,
    interactable: bool,
}

// ===== Internal Helpers =====

/// Get a string attribute from an AX element. Returns empty string on failure.
fn ax_get_string_attr(element: AXUIElementRef, attr: &str) -> String {
    let cf_attr = CFString::new(attr);
    let mut value: *const c_void = ptr::null();

    // SAFETY: element is a valid AXUIElementRef, cf_attr is a valid CFString.
    let err = unsafe {
        AXUIElementCopyAttributeValue(
            element,
            cf_attr.as_concrete_TypeRef() as *const c_void,
            &mut value,
        )
    };

    if err != K_AX_ERROR_SUCCESS || value.is_null() {
        return String::new();
    }

    // SAFETY: value is a CFStringRef returned by AXUIElementCopyAttributeValue.
    unsafe {
        let cf_str = CFString::wrap_under_create_rule(value as *const _);
        cf_str.to_string()
    }
}

/// Get the position (x, y) of an AX element via AXValue.
fn ax_get_position(element: AXUIElementRef) -> Option<(f64, f64)> {
    let cf_attr = CFString::new(K_AX_POSITION_ATTR);
    let mut value: *const c_void = ptr::null();

    // SAFETY: element is valid AXUIElementRef.
    let err = unsafe {
        AXUIElementCopyAttributeValue(
            element,
            cf_attr.as_concrete_TypeRef() as *const c_void,
            &mut value,
        )
    };

    if err != K_AX_ERROR_SUCCESS || value.is_null() {
        return None;
    }

    // AXValue contains a CGPoint {x: f64, y: f64}
    let mut point = [0.0f64; 2];
    // SAFETY: value is an AXValueRef containing a CGPoint.
    let ok = unsafe { AXValueGetValue(value, 1, point.as_mut_ptr() as *mut c_void) };
    // SAFETY: value was created by framework, we must release it.
    unsafe { CFRelease(value as *mut c_void) };

    if ok {
        Some((point[0], point[1]))
    } else {
        None
    }
}

/// Get the size (width, height) of an AX element via AXValue.
fn ax_get_size(element: AXUIElementRef) -> Option<(f64, f64)> {
    let cf_attr = CFString::new(K_AX_SIZE_ATTR);
    let mut value: *const c_void = ptr::null();

    // SAFETY: element is valid AXUIElementRef.
    let err = unsafe {
        AXUIElementCopyAttributeValue(
            element,
            cf_attr.as_concrete_TypeRef() as *const c_void,
            &mut value,
        )
    };

    if err != K_AX_ERROR_SUCCESS || value.is_null() {
        return None;
    }

    // AXValue contains a CGSize {width: f64, height: f64}
    let mut size = [0.0f64; 2];
    // SAFETY: value is an AXValueRef containing a CGSize.
    let ok = unsafe { AXValueGetValue(value, 2, size.as_mut_ptr() as *mut c_void) };
    // SAFETY: value was created by framework, we must release it.
    unsafe { CFRelease(value as *mut c_void) };

    if ok {
        Some((size[0], size[1]))
    } else {
        None
    }
}

#[link(name = "ApplicationServices", kind = "framework")]
unsafe extern "C" {
    fn AXValueGetValue(
        value: *const c_void,
        value_type: u32, // 1=CGPoint, 2=CGSize
        value_ptr: *mut c_void,
    ) -> bool;
}

/// Get children of an AX element as a Vec of AXUIElementRef.
/// Each child is CFRetain'd — caller MUST CFRelease every element after use.
fn ax_get_children(element: AXUIElementRef) -> Vec<AXUIElementRef> {
    let cf_attr = CFString::new(K_AX_CHILDREN_ATTR);
    let mut value: *const c_void = ptr::null();

    // SAFETY: element is valid AXUIElementRef.
    let err = unsafe {
        AXUIElementCopyAttributeValue(
            element,
            cf_attr.as_concrete_TypeRef() as *const c_void,
            &mut value,
        )
    };

    if err != K_AX_ERROR_SUCCESS || value.is_null() {
        return Vec::new();
    }

    // SAFETY: value is a CFArrayRef of AXUIElementRef.
    let array = unsafe { CFArray::<CFType>::wrap_under_create_rule(value as *const _) };
    let count = array.len();
    let mut children = Vec::with_capacity(count as usize);
    let range = CFRange::init(0, count);
    let values = array.get_values(range);

    for i in 0..count {
        let child = values[i as usize];
        // SAFETY: Retain each child so it survives past array drop.
        unsafe { CFRetain(child as *const c_void) };
        children.push(child as AXUIElementRef);
    }

    // Array is dropped here; each child has +1 retain count, caller must CFRelease.
    children
}

/// Convert an AXUIElementRef to an AXElement (with bounding box).
fn ax_element_to_struct(element: AXUIElementRef) -> Option<AXElement> {
    let role = ax_get_string_attr(element, K_AX_ROLE_ATTR);
    if role.is_empty() {
        return None;
    }

    let title = ax_get_string_attr(element, K_AX_TITLE_ATTR);
    let desc = ax_get_string_attr(element, K_AX_DESCRIPTION_ATTR);
    let label = if !title.is_empty() {
        title
    } else {
        desc
    };

    let (x, y) = ax_get_position(element)?;
    let (w, h) = ax_get_size(element)?;

    // Skip zero-size elements
    if w < 1.0 || h < 1.0 {
        return None;
    }

    let interactable = INTERACTABLE_ROLES.iter().any(|r| role == *r);

    Some(AXElement {
        role: role.strip_prefix("AX").unwrap_or(&role).to_lowercase(),
        label,
        x1: x as i32,
        y1: y as i32,
        x2: (x + w) as i32,
        y2: (y + h) as i32,
        interactable,
    })
}

/// Recursively enumerate all UI elements under a root element.
/// `max_depth` prevents infinite recursion. Results appended to `out`.
/// `web_deep` enables extended depth for web content areas.
fn enumerate_elements(
    element: AXUIElementRef,
    out: &mut Vec<AXElement>,
    depth: u32,
    web_deep: bool,
) {
    const MAX_DEPTH_STANDARD: u32 = 15;
    const MAX_DEPTH_WEB: u32 = 25;
    const MAX_ELEMENTS: usize = 500;

    let max_depth = if web_deep {
        MAX_DEPTH_WEB
    } else {
        MAX_DEPTH_STANDARD
    };

    if depth > max_depth || out.len() >= MAX_ELEMENTS {
        return;
    }

    // Detect AXWebArea role — reset depth for deep web content
    let role = ax_get_string_attr(element, K_AX_ROLE_ATTR);
    let entering_web = role == "AXWebArea";
    let effective_depth = if entering_web { 0 } else { depth };
    let child_web_deep = web_deep || entering_web;

    if let Some(el) = ax_element_to_struct(element) {
        out.push(el);
    }

    // Recurse into children
    let children = ax_get_children(element);
    for child in &children {
        enumerate_elements(
            *child,
            out,
            effective_depth + 1,
            child_web_deep,
        );
    }
    // SAFETY: Release children retained by ax_get_children.
    for child in &children {
        unsafe { CFRelease(*child as *mut c_void) };
    }
}

/// Serialize elements to JSON and write to out pointers.
fn write_json_output(
    elements: &[AXElement],
    out_json: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    let json = match serde_json::to_vec(elements) {
        Ok(j) => j,
        Err(e) => {
            eprintln!("[accessibility] JSON serialize error: {e}");
            return ARGUS_ERR_INTERNAL;
        }
    };

    let boxed = json.into_boxed_slice();
    let len = boxed.len();
    let ptr = Box::into_raw(boxed) as *mut u8;

    // SAFETY: out_json/out_len were verified non-null by caller.
    unsafe {
        *out_json = ptr as *mut u8;
        *out_len = len;
    }
    ARGUS_OK
}

// ===== C ABI Exports =====

/// Check macOS permissions for Accessibility and Screen Recording.
/// Uses prompt/request APIs to trigger system authorization dialogs
/// on first launch.
///
/// Returns JSON: {"accessibility": bool, "screen_recording": bool}
///
/// # Safety
/// `out_json` and `out_len` must be non-null. Caller must free via
/// `argus_free_buffer`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_check_permissions(
    out_json: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    if out_json.is_null() || out_len.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }

    // AX: Use AXIsProcessTrustedWithOptions with prompt option
    // This will trigger the system dialog if not yet authorized
    let ax = unsafe {
        let key_str = CFString::new(K_AX_TRUSTED_CHECK_OPTION_PROMPT);
        let key = key_str.as_concrete_TypeRef() as *const c_void;
        let value = kCFBooleanTrue;
        let options = CFDictionaryCreate(
            ptr::null(),
            &key,
            &value,
            1,
            &kCFTypeDictionaryKeyCallBacks as *const c_void,
            &kCFTypeDictionaryValueCallBacks as *const c_void,
        );
        let result = AXIsProcessTrustedWithOptions(options);
        CFRelease(options as *mut c_void);
        result
    };

    // Screen Recording: Use CGPreflightScreenCaptureAccess (check-only)
    // We intentionally do NOT call CGRequestScreenCaptureAccess here
    // because it interferes with SCK's own sck_discover() initialization.
    // SCK has its own sck_request_permission() that handles the request.
    let sr = unsafe { CGPreflightScreenCaptureAccess() };

    let json_str = format!(
        r#"{{"accessibility":{},"screen_recording":{}}}"#,
        ax, sr
    );
    let bytes = json_str.into_bytes();
    let boxed = bytes.into_boxed_slice();
    let len = boxed.len();
    let ptr = Box::into_raw(boxed) as *mut u8;

    unsafe {
        *out_json = ptr as *mut u8;
        *out_len = len;
    }
    ARGUS_OK
}

/// Requests Screen Recording permission explicitly.
/// Returns true if permission acts as "granted" (preflight check passed),
/// otherwise triggers the system dialog and returns false.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_request_screen_capture() -> bool {
    unsafe { CGRequestScreenCaptureAccess() }
}

/// List all UI elements for the given process ID.
///
/// # Safety
/// `out_json` and `out_len` must be non-null. Caller must free the
/// returned buffer via `argus_free_buffer(*out_json, *out_len)`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_ax_list_elements(
    pid: i32,
    out_json: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    if out_json.is_null() || out_len.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }

    if !unsafe { AXIsProcessTrusted() } {
        // No accessibility permission — return empty array
        return write_json_output(&[], out_json, out_len);
    }

    // SAFETY: AXUIElementCreateApplication returns retained ref.
    let app = unsafe { AXUIElementCreateApplication(pid) };
    if app.is_null() {
        return write_json_output(&[], out_json, out_len);
    }

    let mut elements = Vec::new();
    enumerate_elements(app, &mut elements, 0, false);

    // SAFETY: app was created by us, must release.
    unsafe { CFRelease(app as *mut c_void) };

    write_json_output(&elements, out_json, out_len)
}

/// Get the UI element at a specific screen position (x, y).
///
/// # Safety
/// `out_json` and `out_len` must be non-null. Caller must free via
/// `argus_free_buffer`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_ax_element_at_position(
    x: f32,
    y: f32,
    out_json: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    if out_json.is_null() || out_len.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }

    if !unsafe { AXIsProcessTrusted() } {
        return write_json_output(&[], out_json, out_len);
    }

    // SAFETY: AXUIElementCreateSystemWide returns retained ref.
    let sys = unsafe { AXUIElementCreateSystemWide() };
    if sys.is_null() {
        return write_json_output(&[], out_json, out_len);
    }

    let mut element_ref: AXUIElementRef = ptr::null();
    // SAFETY: sys is valid, element_ref receives result.
    let err = unsafe { AXUIElementCopyElementAtPosition(sys, x, y, &mut element_ref) };
    // SAFETY: sys created by us.
    unsafe { CFRelease(sys as *mut c_void) };

    if err != K_AX_ERROR_SUCCESS || element_ref.is_null() {
        return write_json_output(&[], out_json, out_len);
    }

    let mut elements = Vec::new();
    if let Some(el) = ax_element_to_struct(element_ref) {
        elements.push(el);
    }
    // SAFETY: element_ref was created by framework.
    unsafe { CFRelease(element_ref as *mut c_void) };

    write_json_output(&elements, out_json, out_len)
}

/// Get all UI elements of the currently focused application.
///
/// # Safety
/// `out_json` and `out_len` must be non-null. Caller must free via
/// `argus_free_buffer`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn argus_ax_focused_app(
    out_json: *mut *mut u8,
    out_len: *mut usize,
) -> i32 {
    if out_json.is_null() || out_len.is_null() {
        return ARGUS_ERR_NULL_PTR;
    }

    if !unsafe { AXIsProcessTrusted() } {
        return write_json_output(&[], out_json, out_len);
    }

    // SAFETY: AXUIElementCreateSystemWide returns retained ref.
    let sys = unsafe { AXUIElementCreateSystemWide() };
    if sys.is_null() {
        return write_json_output(&[], out_json, out_len);
    }

    // Get focused application
    let cf_attr = CFString::new(K_AX_FOCUSED_APP_ATTR);
    let mut app_ref: *const c_void = ptr::null();
    // SAFETY: sys is valid, cf_attr is valid CFString.
    let err = unsafe {
        AXUIElementCopyAttributeValue(
            sys,
            cf_attr.as_concrete_TypeRef() as *const c_void,
            &mut app_ref,
        )
    };
    // SAFETY: sys created by us.
    unsafe { CFRelease(sys as *mut c_void) };

    if err != K_AX_ERROR_SUCCESS || app_ref.is_null() {
        return write_json_output(&[], out_json, out_len);
    }

    // C3: Smart strategy — detect app type via bundle identifier
    let bundle_id = ax_get_string_attr(
        app_ref as AXUIElementRef,
        K_AX_BUNDLE_ID_ATTR,
    );

    let is_electron = ELECTRON_BUNDLE_PREFIXES
        .iter()
        .any(|prefix| bundle_id.starts_with(prefix));
    let is_browser = BROWSER_BUNDLE_IDS
        .iter()
        .any(|bid| bundle_id.starts_with(bid));

    // C1: For Electron apps, force AXManualAccessibility for full AX tree
    if is_electron {
        try_enable_manual_accessibility(app_ref as AXUIElementRef);
    }

    let web_deep = is_electron || is_browser;

    // Enumerate all windows → children
    let mut elements = Vec::new();

    // Get windows
    let win_attr = CFString::new(K_AX_WINDOWS_ATTR);
    let mut win_val: *const c_void = ptr::null();
    // SAFETY: app_ref is valid AXUIElementRef.
    let werr = unsafe {
        AXUIElementCopyAttributeValue(
            app_ref as AXUIElementRef,
            win_attr.as_concrete_TypeRef() as *const c_void,
            &mut win_val,
        )
    };

    if werr == K_AX_ERROR_SUCCESS && !win_val.is_null() {
        // SAFETY: win_val is CFArrayRef.
        let arr = unsafe { CFArray::<CFType>::wrap_under_create_rule(win_val as *const _) };
        let win_count = arr.len();
        let win_range = CFRange::init(0, win_count);
        let win_values = arr.get_values(win_range);
        // Retain each window ref so it survives past arr drop.
        for i in 0..win_count {
            unsafe { CFRetain(win_values[i as usize] as *const c_void) };
        }
        // arr is dropped here after this block; window refs are still valid.
        for i in 0..win_count {
            let win = win_values[i as usize];
            enumerate_elements(
                win as AXUIElementRef,
                &mut elements,
                0,
                web_deep,
            );
            // SAFETY: Release the retain we added above.
            unsafe { CFRelease(win as *mut c_void) };
        }
    }

    // SAFETY: app_ref was created by framework.
    unsafe { CFRelease(app_ref as *mut c_void) };

    write_json_output(&elements, out_json, out_len)
}

// ===== Hybrid AX Helpers =====

/// Try to enable AXManualAccessibility on an Electron/Chromium app.
/// This forces the Chromium accessibility tree to be fully exposed.
fn try_enable_manual_accessibility(app_ref: AXUIElementRef) {
    let attr = CFString::new("AXManualAccessibility");
    let value = CFBoolean::true_value();
    // SAFETY: app_ref is valid, attr is valid CFString, value is CFBoolean.
    let err = unsafe {
        AXUIElementSetAttributeValue(
            app_ref,
            attr.as_concrete_TypeRef() as *const c_void,
            value.as_concrete_TypeRef() as *const c_void,
        )
    };
    if err != K_AX_ERROR_SUCCESS {
        eprintln!(
            "[accessibility] AXManualAccessibility set failed: err={}",
            err
        );
    }
}
