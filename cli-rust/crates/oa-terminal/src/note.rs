/// Note/info box rendering for terminal output.
///
/// Renders styled note boxes with optional titles and word wrapping.
///
/// Source: `src/terminal/note.ts`

use crate::ansi;
use crate::theme::Theme;

/// Wrap a message for display inside a note box.
///
/// Preserves indentation and bullet points (-, *, \u{2022}).
pub fn wrap_note_message(message: &str, max_width: Option<usize>, columns: Option<usize>) -> String {
    let cols = columns.unwrap_or_else(|| {
        console::Term::stdout().size().1 as usize
    });
    let width = max_width.unwrap_or_else(|| {
        40_usize.max(88.min(cols.saturating_sub(10)))
    });

    let mut output = String::new();
    for line in message.lines() {
        if line.is_empty() {
            output.push('\n');
            continue;
        }

        // Detect indent and bullet prefix
        let trimmed = line.trim_start();
        let indent_len = line.len() - trimmed.len();
        let indent: String = line.chars().take(indent_len).collect();

        let effective_width = width.saturating_sub(indent_len);
        if effective_width < 10 {
            output.push_str(line);
            output.push('\n');
            continue;
        }

        // Simple word wrap
        let words: Vec<&str> = trimmed.split_whitespace().collect();
        let mut current_line = String::new();

        for word in words {
            let word_len = ansi::visible_width(word);
            let current_len = ansi::visible_width(&current_line);

            if current_len + word_len + 1 > effective_width && !current_line.is_empty() {
                output.push_str(&indent);
                output.push_str(current_line.trim_end());
                output.push('\n');
                current_line = String::new();
            }

            if !current_line.is_empty() {
                current_line.push(' ');
            }
            current_line.push_str(word);
        }

        if !current_line.is_empty() {
            output.push_str(&indent);
            output.push_str(current_line.trim_end());
            output.push('\n');
        }
    }

    output
}

/// Print a styled note box to stderr.
///
/// Renders a box with an optional title and wrapped message content.
pub fn note(message: &str, title: Option<&str>) {
    let wrapped = wrap_note_message(message, None, None);
    let bar = "\u{2502}"; // │

    if let Some(t) = title {
        eprintln!("{}", Theme::heading(t));
    }

    for line in wrapped.lines() {
        eprintln!("  {bar}  {line}");
    }
}
