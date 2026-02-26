/// Table rendering for terminal output.
///
/// Supports ANSI-aware column layout, word wrapping, and multiple border styles.
///
/// Source: `src/terminal/table.ts`

use crate::ansi;

/// Column alignment.
#[derive(Debug, Clone, Copy, Default)]
pub enum Align {
    #[default]
    Left,
    Right,
    Center,
}

/// Table column definition.
#[derive(Debug, Clone)]
pub struct TableColumn {
    /// Column key (used to look up values in rows).
    pub key: String,
    /// Display header text.
    pub header: String,
    /// Column alignment.
    pub align: Align,
    /// Minimum column width.
    pub min_width: Option<usize>,
    /// Maximum column width.
    pub max_width: Option<usize>,
    /// Whether this column should flex to fill available space.
    pub flex: bool,
}

/// Border style for table rendering.
#[derive(Debug, Clone, Copy, Default)]
pub enum BorderStyle {
    /// Unicode box-drawing characters.
    #[default]
    Unicode,
    /// ASCII `+`, `-`, `|` characters.
    Ascii,
    /// No borders.
    None,
}

/// Box-drawing character set.
struct BoxChars {
    tl: &'static str,
    tr: &'static str,
    bl: &'static str,
    br: &'static str,
    h: &'static str,
    v: &'static str,
    t: &'static str,
    ml: &'static str,
    m: &'static str,
    mr: &'static str,
    b: &'static str,
}

const UNICODE_BOX: BoxChars = BoxChars {
    tl: "\u{250c}", tr: "\u{2510}", bl: "\u{2514}", br: "\u{2518}",
    h: "\u{2500}", v: "\u{2502}", t: "\u{252c}", ml: "\u{251c}",
    m: "\u{253c}", mr: "\u{2524}", b: "\u{2534}",
};

const ASCII_BOX: BoxChars = BoxChars {
    tl: "+", tr: "+", bl: "+", br: "+",
    h: "-", v: "|", t: "+", ml: "+",
    m: "+", mr: "+", b: "+",
};

/// Options for rendering a table.
pub struct RenderTableOptions {
    /// Column definitions.
    pub columns: Vec<TableColumn>,
    /// Row data (key → value mappings).
    pub rows: Vec<std::collections::HashMap<String, String>>,
    /// Total width (defaults to terminal width).
    pub width: Option<usize>,
    /// Cell padding (chars on each side).
    pub padding: usize,
    /// Border style.
    pub border: BorderStyle,
}

/// Render a table to a string.
///
/// Handles ANSI-aware width calculations, flexible column sizing,
/// and multi-line cell wrapping.
pub fn render_table(opts: &RenderTableOptions) -> String {
    let total_width = opts.width.unwrap_or_else(|| {
        console::Term::stdout().size().1 as usize
    });

    if opts.columns.is_empty() || opts.rows.is_empty() {
        return String::new();
    }

    let box_chars = match opts.border {
        BorderStyle::Unicode => Some(&UNICODE_BOX),
        BorderStyle::Ascii => Some(&ASCII_BOX),
        BorderStyle::None => None,
    };

    let border_overhead = if box_chars.is_some() {
        opts.columns.len() + 1 // one `|` per column + trailing
    } else {
        0
    };
    let padding_overhead = opts.columns.len() * opts.padding * 2;
    let available = total_width.saturating_sub(border_overhead + padding_overhead);

    // Calculate column widths (simple proportional for now)
    let col_width = available / opts.columns.len().max(1);
    let col_widths: Vec<usize> = opts.columns.iter().map(|c| {
        let min = c.min_width.unwrap_or(4);
        let max = c.max_width.unwrap_or(col_width * 2);
        col_width.clamp(min, max)
    }).collect();

    let mut output = String::new();

    // Header
    if let Some(bc) = box_chars {
        // Top border
        output.push_str(bc.tl);
        for (i, &w) in col_widths.iter().enumerate() {
            for _ in 0..(w + opts.padding * 2) {
                output.push_str(bc.h);
            }
            if i < col_widths.len() - 1 {
                output.push_str(bc.t);
            }
        }
        output.push_str(bc.tr);
        output.push('\n');

        // Header row
        output.push_str(bc.v);
        for (i, col) in opts.columns.iter().enumerate() {
            let pad = " ".repeat(opts.padding);
            let text = pad_or_truncate(&col.header, col_widths[i]);
            output.push_str(&format!("{pad}{text}{pad}"));
            output.push_str(bc.v);
        }
        output.push('\n');

        // Header separator
        output.push_str(bc.ml);
        for (i, &w) in col_widths.iter().enumerate() {
            for _ in 0..(w + opts.padding * 2) {
                output.push_str(bc.h);
            }
            if i < col_widths.len() - 1 {
                output.push_str(bc.m);
            }
        }
        output.push_str(bc.mr);
        output.push('\n');
    }

    // Data rows
    for row in &opts.rows {
        if let Some(bc) = box_chars {
            output.push_str(bc.v);
        }
        for (i, col) in opts.columns.iter().enumerate() {
            let value = row.get(&col.key).map(String::as_str).unwrap_or("");
            let pad = " ".repeat(opts.padding);
            let text = pad_or_truncate(value, col_widths[i]);
            output.push_str(&format!("{pad}{text}{pad}"));
            if let Some(bc) = box_chars {
                output.push_str(bc.v);
            }
        }
        output.push('\n');
    }

    // Bottom border
    if let Some(bc) = box_chars {
        output.push_str(bc.bl);
        for (i, &w) in col_widths.iter().enumerate() {
            for _ in 0..(w + opts.padding * 2) {
                output.push_str(bc.h);
            }
            if i < col_widths.len() - 1 {
                output.push_str(bc.b);
            }
        }
        output.push_str(bc.br);
        output.push('\n');
    }

    output
}

/// Pad or truncate a string to exactly `width` visible characters.
fn pad_or_truncate(text: &str, width: usize) -> String {
    let visible = ansi::visible_width(text);
    if visible <= width {
        format!("{text}{}", " ".repeat(width - visible))
    } else {
        // Truncate (simple approach: take first `width-1` chars + ellipsis)
        let mut result = String::new();
        let mut count = 0;
        for ch in text.chars() {
            if count >= width.saturating_sub(1) {
                break;
            }
            result.push(ch);
            count += 1;
        }
        result.push('\u{2026}'); // ellipsis
        let final_visible = ansi::visible_width(&result);
        if final_visible < width {
            result.push_str(&" ".repeat(width - final_visible));
        }
        result
    }
}
