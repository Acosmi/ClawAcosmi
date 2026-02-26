/**
 * Formatting utilities.
 *
 * Inlined from TS backend:
 *   - formatDurationHuman()          ← src/infra/format-time/format-duration.ts
 *   - formatRelativeTimestamp()      ← src/infra/format-time/format-relative.ts
 *   - stripReasoningTagsFromText()   ← src/shared/text/reasoning-tags.ts
 */

import { getLocale, t } from "./i18n.ts";

// ---------------------------------------------------------------------------
// formatDurationHuman (from format-duration.ts)
// ---------------------------------------------------------------------------

/**
 * Rounded single-unit duration for display: "500ms", "5s", "3m", "2h", "5d".
 * Returns fallback string for null/undefined/non-finite input.
 */
export function formatDurationHuman(ms?: number | null, fallback = "n/a"): string {
  if (ms == null || !Number.isFinite(ms) || ms < 0) {
    return fallback;
  }
  if (ms < 1000) {
    return `${Math.round(ms)}ms`;
  }
  const sec = Math.round(ms / 1000);
  if (sec < 60) {
    return `${sec}s`;
  }
  const min = Math.round(sec / 60);
  if (min < 60) {
    return `${min}m`;
  }
  const hr = Math.round(min / 60);
  if (hr < 24) {
    return `${hr}h`;
  }
  const day = Math.round(hr / 24);
  return `${day}d`;
}

// ---------------------------------------------------------------------------
// formatDurationCompact (from format-duration.ts)
// ---------------------------------------------------------------------------

export type FormatDurationCompactOptions = {
  /** If true, separate units with a space (e.g. "5m 30s"). Default: no space. */
  spaced?: boolean;
};

/**
 * Multi-unit compact duration: "5m30s", "2h15m", "3d4h".
 * Returns undefined for null/undefined/non-positive input.
 */
export function formatDurationCompact(
  ms?: number | null,
  options?: FormatDurationCompactOptions,
): string | undefined {
  if (ms == null || !Number.isFinite(ms) || ms <= 0) {
    return undefined;
  }
  if (ms < 1000) {
    return `${Math.round(ms)}ms`;
  }
  const sep = options?.spaced ? " " : "";
  const totalSeconds = Math.round(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours >= 24) {
    const days = Math.floor(hours / 24);
    const remainingHours = hours % 24;
    return remainingHours > 0 ? `${days}d${sep}${remainingHours}h` : `${days}d`;
  }
  if (hours > 0) {
    return minutes > 0 ? `${hours}h${sep}${minutes}m` : `${hours}h`;
  }
  if (minutes > 0) {
    return seconds > 0 ? `${minutes}m${sep}${seconds}s` : `${minutes}m`;
  }
  return `${seconds}s`;
}

// ---------------------------------------------------------------------------
// formatRelativeTimestamp (from format-relative.ts)
// ---------------------------------------------------------------------------

export type FormatRelativeTimestampOptions = {
  /** If true, fall back to short date (e.g. "Oct 5") for timestamps >7 days. Default: false */
  dateFallback?: boolean;
  /** IANA timezone for date fallback display */
  timezone?: string;
  /** Return value for invalid/null input. Default: "n/a" */
  fallback?: string;
};

/**
 * Format an epoch timestamp relative to now.
 *
 * Handles both past ("5m ago") and future ("in 5m") timestamps.
 * Optionally falls back to a short date for timestamps older than 7 days.
 */
export function formatRelativeTimestamp(
  timestampMs: number | null | undefined,
  options?: FormatRelativeTimestampOptions,
): string {
  const fallback = options?.fallback ?? "n/a";
  if (timestampMs == null || !Number.isFinite(timestampMs)) {
    return fallback;
  }

  const diff = Date.now() - timestampMs;
  const absDiff = Math.abs(diff);
  const isPast = diff >= 0;

  const sec = Math.round(absDiff / 1000);
  if (sec < 60) {
    return isPast ? t("format.justNow") : t("format.inLessThan1m");
  }

  const min = Math.round(sec / 60);
  if (min < 60) {
    return isPast ? t("format.mAgo", { n: min }) : t("format.inM", { n: min });
  }

  const hr = Math.round(min / 60);
  if (hr < 48) {
    return isPast ? t("format.hAgo", { n: hr }) : t("format.inH", { n: hr });
  }

  const day = Math.round(hr / 24);
  if (!options?.dateFallback || day <= 7) {
    return isPast ? t("format.dAgo", { n: day }) : t("format.inD", { n: day });
  }

  // Fall back to short date display for old timestamps
  try {
    return new Intl.DateTimeFormat(resolveIntlLocale(), {
      month: "short",
      day: "numeric",
      ...(options.timezone ? { timeZone: options.timezone } : {}),
    }).format(new Date(timestampMs));
  } catch {
    return t("format.dAgo", { n: day });
  }
}

// ---------------------------------------------------------------------------
// stripReasoningTagsFromText (from reasoning-tags.ts)
// ---------------------------------------------------------------------------

type ReasoningTagMode = "strict" | "preserve";
type ReasoningTagTrim = "none" | "start" | "both";

const QUICK_TAG_RE = /<\s*\/?\s*(?:think(?:ing)?|thought|antthinking|final)\b/i;
const FINAL_TAG_RE = /<\s*\/?\s*final\b[^<>]*>/gi;
const THINKING_TAG_RE = /<\s*(\/?)\s*(?:think(?:ing)?|thought|antthinking)\b[^<>]*>/gi;

interface CodeRegion {
  start: number;
  end: number;
}

function findCodeRegions(text: string): CodeRegion[] {
  const regions: CodeRegion[] = [];

  const fencedRe = /(^|\n)(```|~~~)[^\n]*\n[\s\S]*?(?:\n\2(?:\n|$)|$)/g;
  for (const match of text.matchAll(fencedRe)) {
    const start = (match.index ?? 0) + match[1].length;
    regions.push({ start, end: start + match[0].length - match[1].length });
  }

  const inlineRe = /`+[^`]+`+/g;
  for (const match of text.matchAll(inlineRe)) {
    const start = match.index ?? 0;
    const end = start + match[0].length;
    const insideFenced = regions.some((r) => start >= r.start && end <= r.end);
    if (!insideFenced) {
      regions.push({ start, end });
    }
  }

  regions.sort((a, b) => a.start - b.start);
  return regions;
}

function isInsideCode(pos: number, regions: CodeRegion[]): boolean {
  return regions.some((r) => pos >= r.start && pos < r.end);
}

function applyTrim(value: string, mode: ReasoningTagTrim): string {
  if (mode === "none") {
    return value;
  }
  if (mode === "start") {
    return value.trimStart();
  }
  return value.trim();
}

export function stripReasoningTagsFromText(
  text: string,
  options?: {
    mode?: ReasoningTagMode;
    trim?: ReasoningTagTrim;
  },
): string {
  if (!text) {
    return text;
  }
  if (!QUICK_TAG_RE.test(text)) {
    return text;
  }

  const mode = options?.mode ?? "strict";
  const trimMode = options?.trim ?? "both";

  let cleaned = text;
  if (FINAL_TAG_RE.test(cleaned)) {
    FINAL_TAG_RE.lastIndex = 0;
    const finalMatches: Array<{ start: number; length: number; inCode: boolean }> = [];
    const preCodeRegions = findCodeRegions(cleaned);
    for (const match of cleaned.matchAll(FINAL_TAG_RE)) {
      const start = match.index ?? 0;
      finalMatches.push({
        start,
        length: match[0].length,
        inCode: isInsideCode(start, preCodeRegions),
      });
    }

    for (let i = finalMatches.length - 1; i >= 0; i--) {
      const m = finalMatches[i];
      if (!m.inCode) {
        cleaned = cleaned.slice(0, m.start) + cleaned.slice(m.start + m.length);
      }
    }
  } else {
    FINAL_TAG_RE.lastIndex = 0;
  }

  const codeRegions = findCodeRegions(cleaned);

  THINKING_TAG_RE.lastIndex = 0;
  let result = "";
  let lastIndex = 0;
  let inThinking = false;

  for (const match of cleaned.matchAll(THINKING_TAG_RE)) {
    const idx = match.index ?? 0;
    const isClose = match[1] === "/";

    if (isInsideCode(idx, codeRegions)) {
      continue;
    }

    if (!inThinking) {
      result += cleaned.slice(lastIndex, idx);
      if (!isClose) {
        inThinking = true;
      }
    } else if (isClose) {
      inThinking = false;
    }

    lastIndex = idx + match[0].length;
  }

  if (!inThinking || mode === "preserve") {
    result += cleaned.slice(lastIndex);
  }

  return applyTrim(result, trimMode);
}

// ---------------------------------------------------------------------------
// Locale-aware Intl formatting
// ---------------------------------------------------------------------------

const LOCALE_MAP: Record<string, string> = { zh: "zh-CN", en: "en-US" };

function resolveIntlLocale(): string {
  return LOCALE_MAP[getLocale()] ?? "en-US";
}

/**
 * Full date-time display: "2026/2/17 12:30:00" (zh) / "2/17/2026, 12:30:00 PM" (en).
 * Uses the app's current i18n locale.
 */
export function formatDateTime(ms?: number | null, fallback = "n/a"): string {
  if (ms == null || !Number.isFinite(ms)) {
    return fallback;
  }
  try {
    return new Intl.DateTimeFormat(resolveIntlLocale(), {
      year: "numeric",
      month: "numeric",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit",
      second: "2-digit",
    }).format(new Date(ms));
  } catch {
    return new Date(ms).toLocaleString();
  }
}

/**
 * Short time display: "12:30" (zh) / "12:30 PM" (en).
 * Uses the app's current i18n locale.
 */
export function formatTimeShort(ms: number): string {
  try {
    return new Intl.DateTimeFormat(resolveIntlLocale(), {
      hour: "numeric",
      minute: "2-digit",
    }).format(new Date(ms));
  } catch {
    return new Date(ms).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" });
  }
}

/**
 * Locale-aware number formatting with grouping separators.
 * "1,234.5" (en) / "1,234.5" (zh).
 */
export function formatNumber(value: number, maximumFractionDigits = 2): string {
  try {
    return new Intl.NumberFormat(resolveIntlLocale(), { maximumFractionDigits }).format(value);
  } catch {
    return String(value);
  }
}

// ---------------------------------------------------------------------------
// Local utilities (originally in this file)
// ---------------------------------------------------------------------------

/** @deprecated Use `formatDateTime` instead for locale-aware output. */
export function formatMs(ms?: number | null): string {
  if (!ms && ms !== 0) {
    return "n/a";
  }
  return formatDateTime(ms);
}

export function formatList(values?: Array<string | null | undefined>): string {
  if (!values || values.length === 0) {
    return t("format.none");
  }
  return values.filter((v): v is string => Boolean(v && v.trim())).join(", ");
}

export function clampText(value: string, max = 120): string {
  if (value.length <= max) {
    return value;
  }
  return `${value.slice(0, Math.max(0, max - 1))}…`;
}

export function truncateText(
  value: string,
  max: number,
): {
  text: string;
  truncated: boolean;
  total: number;
} {
  if (value.length <= max) {
    return { text: value, truncated: false, total: value.length };
  }
  return {
    text: value.slice(0, Math.max(0, max)),
    truncated: true,
    total: value.length,
  };
}

export function toNumber(value: string, fallback: number): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : fallback;
}

export function parseList(input: string): string[] {
  return input
    .split(/[,\n]/)
    .map((v) => v.trim())
    .filter((v) => v.length > 0);
}

export function stripThinkingTags(value: string): string {
  return stripReasoningTagsFromText(value, { mode: "preserve", trim: "start" });
}
