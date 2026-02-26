/// Global CLI state flags.
///
/// Provides global verbose and yes-to-all flags that are shared across
/// CLI commands. Uses atomic operations for thread-safe access.
///
/// Source: `src/globals.ts`

use std::sync::atomic::{AtomicBool, Ordering};

use oa_terminal::theme::Theme;

/// Global verbose mode flag.
///
/// Source: `src/globals.ts` – `globalVerbose`
static VERBOSE: AtomicBool = AtomicBool::new(false);

/// Global yes-to-all (skip confirmation prompts) flag.
///
/// Source: `src/globals.ts` – `globalYes`
static YES: AtomicBool = AtomicBool::new(false);

/// Enable or disable verbose mode globally.
///
/// Source: `src/globals.ts` – `setVerbose`
pub fn set_verbose(v: bool) {
    VERBOSE.store(v, Ordering::Relaxed);
}

/// Check whether verbose mode is currently enabled.
///
/// Source: `src/globals.ts` – `isVerbose`
pub fn is_verbose() -> bool {
    VERBOSE.load(Ordering::Relaxed)
}

/// Determine whether verbose-level messages should be logged.
///
/// Returns `true` when the global verbose flag is set. The TS
/// implementation also checks file-log-level; here we rely solely on
/// the verbose flag since file logging configuration is handled by
/// the tracing subscriber layer.
///
/// Source: `src/globals.ts` – `shouldLogVerbose`
pub fn should_log_verbose() -> bool {
    is_verbose()
}

/// Log a verbose-level message.
///
/// When verbose mode is enabled, emits the message both as a `tracing::debug`
/// event and as muted text to stderr. When verbose mode is disabled, this
/// function is a no-op.
///
/// Source: `src/globals.ts` – `logVerbose`
pub fn log_verbose(message: &str) {
    if !should_log_verbose() {
        return;
    }
    tracing::debug!(%message, "verbose");
    if !is_verbose() {
        return;
    }
    eprintln!("{}", Theme::muted(message));
}

/// Log a verbose-level message to the console only (no tracing).
///
/// Source: `src/globals.ts` – `logVerboseConsole`
pub fn log_verbose_console(message: &str) {
    if !is_verbose() {
        return;
    }
    eprintln!("{}", Theme::muted(message));
}

/// Enable or disable yes-to-all mode globally.
///
/// Source: `src/globals.ts` – `setYes`
pub fn set_yes(v: bool) {
    YES.store(v, Ordering::Relaxed);
}

/// Check whether yes-to-all mode is currently enabled.
///
/// Source: `src/globals.ts` – `isYes`
pub fn is_yes() -> bool {
    YES.load(Ordering::Relaxed)
}

/// Format a message with the success theme color.
///
/// Source: `src/globals.ts` – `success`
pub fn success(text: &str) -> String {
    Theme::success(text)
}

/// Format a message with the warning theme color.
///
/// Source: `src/globals.ts` – `warn`
pub fn warn(text: &str) -> String {
    Theme::warn(text)
}

/// Format a message with the info theme color.
///
/// Source: `src/globals.ts` – `info`
pub fn info(text: &str) -> String {
    Theme::info(text)
}

/// Format a message with the error/danger theme color.
///
/// Source: `src/globals.ts` – `danger`
pub fn danger(text: &str) -> String {
    Theme::error(text)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn verbose_defaults_to_false() {
        // Reset to known state for test
        set_verbose(false);
        assert!(!is_verbose());
    }

    #[test]
    fn set_and_get_verbose() {
        set_verbose(true);
        assert!(is_verbose());
        assert!(should_log_verbose());
        set_verbose(false);
        assert!(!is_verbose());
        assert!(!should_log_verbose());
    }

    #[test]
    fn yes_defaults_to_false() {
        set_yes(false);
        assert!(!is_yes());
    }

    #[test]
    fn set_and_get_yes() {
        set_yes(true);
        assert!(is_yes());
        set_yes(false);
        assert!(!is_yes());
    }

    #[test]
    fn log_verbose_noop_when_disabled() {
        set_verbose(false);
        // Should not panic or produce side effects
        log_verbose("test message");
    }

    #[test]
    fn log_verbose_console_noop_when_disabled() {
        set_verbose(false);
        log_verbose_console("test message");
    }

    #[test]
    fn color_helpers_return_strings() {
        let s = success("ok");
        assert!(!s.is_empty());
        let w = warn("careful");
        assert!(!w.is_empty());
        let i = info("note");
        assert!(!i.is_empty());
        let d = danger("bad");
        assert!(!d.is_empty());
    }
}
