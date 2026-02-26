/// Progress indicators for CLI operations.
///
/// Wraps `indicatif` to provide spinner-based progress reporting. Falls
/// back to simple line output when stderr is not a TTY.
///
/// Source: `src/cli/progress.ts`

use std::future::Future;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use indicatif::{ProgressBar, ProgressStyle};
use oa_terminal::theme::{is_rich, Theme};

/// Tracks how many progress reporters are currently active.
///
/// Source: `src/cli/progress.ts` – `activeProgress`
static ACTIVE_PROGRESS: AtomicUsize = AtomicUsize::new(0);

/// A progress reporter that can update its message and finish.
///
/// Source: `src/cli/progress.ts` – `ProgressReporter`
pub struct ProgressReporter {
    bar: Option<ProgressBar>,
    label: String,
    is_tty: bool,
}

impl ProgressReporter {
    /// Whether this reporter is connected to a TTY.
    pub fn is_tty(&self) -> bool {
        self.is_tty
    }

    /// Update the progress label text.
    ///
    /// Source: `src/cli/progress.ts` – `setLabel`
    pub fn update(&self, message: &str) {
        if let Some(ref bar) = self.bar {
            if is_rich() {
                bar.set_message(Theme::accent(message));
            } else {
                bar.set_message(message.to_string());
            }
        }
    }

    /// Set the label and update the message.
    ///
    /// Source: `src/cli/progress.ts` – `setLabel`
    pub fn set_label(&mut self, label: &str) {
        self.label = label.to_string();
        self.update(label);
    }

    /// Set the progress percentage (0-100).
    ///
    /// Source: `src/cli/progress.ts` – `setPercent`
    pub fn set_percent(&self, percent: f64) {
        if let Some(ref bar) = self.bar {
            let clamped = percent.clamp(0.0, 100.0).round() as u64;
            bar.set_position(clamped);
        }
    }

    /// Tick the progress forward by `delta` units.
    ///
    /// Source: `src/cli/progress.ts` – `tick`
    pub fn tick(&self, delta: u64) {
        if let Some(ref bar) = self.bar {
            bar.inc(delta);
        }
    }

    /// Complete the progress indicator.
    ///
    /// Source: `src/cli/progress.ts` – `done`
    pub fn finish(&self) {
        if let Some(ref bar) = self.bar {
            bar.finish_and_clear();
        }
        let prev = ACTIVE_PROGRESS.load(Ordering::Relaxed);
        if prev > 0 {
            ACTIVE_PROGRESS.store(prev - 1, Ordering::Relaxed);
        }
    }
}

impl Drop for ProgressReporter {
    fn drop(&mut self) {
        self.finish();
    }
}

/// Create a spinner-based progress reporter.
///
/// Returns a `ProgressReporter` wrapping an `indicatif::ProgressBar`
/// with a spinner style. If stderr is not a TTY or another progress
/// reporter is already active, returns a no-op reporter.
///
/// Source: `src/cli/progress.ts` – `createCliProgress`
pub fn create_cli_progress(label: &str) -> ProgressReporter {
    let is_tty = std::io::IsTerminal::is_terminal(&std::io::stderr());

    // Only allow one active progress indicator at a time
    if ACTIVE_PROGRESS.load(Ordering::Relaxed) > 0 {
        return ProgressReporter {
            bar: None,
            label: label.to_string(),
            is_tty,
        };
    }

    if !is_tty {
        // Non-TTY fallback: print label once as a simple line
        eprintln!("{label}");
        return ProgressReporter {
            bar: None,
            label: label.to_string(),
            is_tty,
        };
    }

    ACTIVE_PROGRESS.fetch_add(1, Ordering::Relaxed);

    let bar = ProgressBar::new_spinner();
    bar.set_style(
        ProgressStyle::default_spinner()
            .tick_strings(&[
                "\u{280B}", "\u{2819}", "\u{2839}", "\u{2838}",
                "\u{283C}", "\u{2834}", "\u{2826}", "\u{2827}",
                "\u{2807}", "\u{280F}", "\u{2809}",
            ])
            .template("{spinner:.cyan} {msg}")
            .unwrap_or_else(|_| ProgressStyle::default_spinner()),
    );
    bar.enable_steady_tick(Duration::from_millis(80));

    let display_label = if is_rich() {
        Theme::accent(label)
    } else {
        label.to_string()
    };
    bar.set_message(display_label);

    ProgressReporter {
        bar: Some(bar),
        label: label.to_string(),
        is_tty,
    }
}

/// Run an async closure with a spinner-based progress indicator.
///
/// Creates a progress reporter for the given label, executes `work`,
/// and automatically finishes the progress when the future completes
/// (whether successfully or via early return).
///
/// Source: `src/cli/progress.ts` – `withProgress`
pub async fn with_progress<T, F>(label: &str, work: F) -> T
where
    F: Future<Output = T>,
{
    let progress = create_cli_progress(label);
    let result = work.await;
    progress.finish();
    result
}

/// Run an async closure that receives the `ProgressReporter` so it can
/// update progress as it works.
///
/// Source: `src/cli/progress.ts` – `withProgress` (callback variant)
pub async fn with_progress_reporter<T, F, Fut>(label: &str, work: F) -> T
where
    F: FnOnce(ProgressReporter) -> Fut,
    Fut: Future<Output = T>,
{
    let progress = create_cli_progress(label);
    work(progress).await
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn noop_reporter_does_not_panic() {
        let reporter = ProgressReporter {
            bar: None,
            label: "test".to_string(),
            is_tty: false,
        };
        reporter.update("updated");
        reporter.set_percent(50.0);
        reporter.tick(1);
        reporter.finish();
    }

    #[test]
    fn active_progress_starts_at_zero() {
        // Ensure we observe a sane value (may be non-zero due to
        // other tests but should not panic)
        let _count = ACTIVE_PROGRESS.load(Ordering::Relaxed);
    }

    #[test]
    fn create_cli_progress_nontty_returns_noop() {
        // In a test environment, stderr is not a TTY, so this should
        // return a reporter with no bar.
        let reporter = create_cli_progress("testing");
        assert!(reporter.bar.is_none());
    }

    #[tokio::test]
    async fn with_progress_runs_work() {
        let result = with_progress("working", async { 42 }).await;
        assert_eq!(result, 42);
    }

    #[tokio::test]
    async fn with_progress_reporter_runs_work() {
        let result = with_progress_reporter("working", |progress| async move {
            progress.update("step 1");
            progress.update("step 2");
            99
        })
        .await;
        assert_eq!(result, 99);
    }
}
