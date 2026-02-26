/// Safe stream writer with broken pipe detection.
///
/// Source: `src/terminal/stream-writer.ts`

use std::io::{self, Write};
use std::sync::atomic::{AtomicBool, Ordering};

/// A safe writer that handles broken pipe (EPIPE) errors gracefully.
pub struct SafeStreamWriter {
    closed: AtomicBool,
}

impl SafeStreamWriter {
    /// Create a new safe stream writer.
    pub fn new() -> Self {
        Self {
            closed: AtomicBool::new(false),
        }
    }

    /// Check if the stream has been closed due to a broken pipe.
    pub fn is_closed(&self) -> bool {
        self.closed.load(Ordering::Relaxed)
    }

    /// Write text to stdout, handling broken pipe gracefully.
    pub fn write(&self, text: &str) -> bool {
        if self.is_closed() {
            return false;
        }
        match io::stdout().write_all(text.as_bytes()) {
            Ok(()) => true,
            Err(e) if is_broken_pipe(&e) => {
                self.closed.store(true, Ordering::Relaxed);
                false
            }
            Err(_) => false,
        }
    }

    /// Write a line to stdout (appends newline).
    pub fn write_line(&self, text: &str) -> bool {
        if self.is_closed() {
            return false;
        }
        let result = self.write(text);
        if result {
            self.write("\n")
        } else {
            false
        }
    }

    /// Reset the closed state.
    pub fn reset(&self) {
        self.closed.store(false, Ordering::Relaxed);
    }
}

impl Default for SafeStreamWriter {
    fn default() -> Self {
        Self::new()
    }
}

/// Check if an I/O error is a broken pipe.
fn is_broken_pipe(err: &io::Error) -> bool {
    err.kind() == io::ErrorKind::BrokenPipe
}
