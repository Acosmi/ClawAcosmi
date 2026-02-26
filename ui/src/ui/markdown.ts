import DOMPurify from "dompurify";
import { Marked } from "marked";
import { markedHighlight } from "marked-highlight";
import markedKatex from "marked-katex-extension";
import Prism from "prismjs";
import "katex/dist/katex.min.css";
import { truncateText } from "./format.ts";

import "prismjs/components/prism-bash";
import "prismjs/components/prism-c";
import "prismjs/components/prism-cpp";
import "prismjs/components/prism-css";
import "prismjs/components/prism-diff";
import "prismjs/components/prism-go";
import "prismjs/components/prism-java";
import "prismjs/components/prism-json";
import "prismjs/components/prism-markdown";
import "prismjs/components/prism-python";
import "prismjs/components/prism-rust";
import "prismjs/components/prism-sql";
import "prismjs/components/prism-toml";
import "prismjs/components/prism-typescript";
import "prismjs/components/prism-yaml";

const markedInstance = new Marked(
  { gfm: true, breaks: true },
  markedHighlight({
    highlight(code: string, lang: string) {
      const grammar = lang ? Prism.languages[lang] : undefined;
      if (grammar) {
        return Prism.highlight(code, grammar, lang);
      }
      return code;
    },
  }),
);

const codeRenderer = {
  code({ text, lang }: { text: string; lang?: string | null }) {
    const displayLang = lang ?? "";
    const langAttr = displayLang
      ? ` data-lang="${displayLang}"`
      : "";
    const langLabel = displayLang
      ? `<span class="code-block-lang">${escapeHtml(displayLang)}</span>`
      : "";
    return `<div class="code-block-wrapper"${langAttr}>`
      + `<div class="code-block-header">${langLabel}<span class="code-block-copy-trigger" title="Copy">⎘</span></div>`
      + `<pre><code class="language-${escapeHtml(displayLang || "text")}">${text}</code></pre>`
      + `</div>`;
  },
};

const tableRenderer = {
  table(token: import("marked").Tokens.Table) {
    const headerCells = token.header
      .map((cell) => {
        const align = cell.align ? ` style="text-align:${cell.align}"` : "";
        return `<th${align}>${cell.text}</th>`;
      })
      .join("");
    const header = `<tr>${headerCells}</tr>`;
    const bodyRows = token.rows
      .map((row) => {
        const cells = row
          .map((cell) => {
            const align = cell.align ? ` style="text-align:${cell.align}"` : "";
            return `<td${align}>${cell.text}</td>`;
          })
          .join("");
        return `<tr>${cells}</tr>`;
      })
      .join("");
    return `<div class="table-wrapper"><table><thead>${header}</thead><tbody>${bodyRows}</tbody></table></div>`;
  },
};

markedInstance.use({ renderer: { ...codeRenderer, ...tableRenderer } });
markedInstance.use(markedKatex({ throwOnError: false, output: "html", nonStandard: true }));

const allowedTags = [
  "a",
  "b",
  "blockquote",
  "br",
  "code",
  "del",
  "details",
  "div",
  "em",
  "g",
  "h1",
  "h2",
  "h3",
  "h4",
  "hr",
  "i",
  "li",
  "line",
  "ol",
  "p",
  "path",
  "pre",
  "rect",
  "span",
  "strong",
  "summary",
  "svg",
  "table",
  "tbody",
  "td",
  "th",
  "thead",
  "tr",
  "ul",
];

const allowedAttrs = [
  "aria-hidden",
  "class",
  "d",
  "data-lang",
  "fill",
  "height",
  "href",
  "rel",
  "start",
  "stroke",
  "style",
  "target",
  "title",
  "viewBox",
  "width",
  "xmlns",
];

let hooksInstalled = false;
const MARKDOWN_CHAR_LIMIT = 140_000;
const MARKDOWN_PARSE_LIMIT = 40_000;
const MARKDOWN_CACHE_LIMIT = 200;
const MARKDOWN_CACHE_MAX_CHARS = 50_000;
const markdownCache = new Map<string, string>();

function getCachedMarkdown(key: string): string | null {
  const cached = markdownCache.get(key);
  if (cached === undefined) {
    return null;
  }
  markdownCache.delete(key);
  markdownCache.set(key, cached);
  return cached;
}

function setCachedMarkdown(key: string, value: string) {
  markdownCache.set(key, value);
  if (markdownCache.size <= MARKDOWN_CACHE_LIMIT) {
    return;
  }
  const oldest = markdownCache.keys().next().value;
  if (oldest) {
    markdownCache.delete(oldest);
  }
}

function installHooks() {
  if (hooksInstalled) {
    return;
  }
  hooksInstalled = true;

  DOMPurify.addHook("afterSanitizeAttributes", (node) => {
    if (!(node instanceof HTMLAnchorElement)) {
      return;
    }
    const href = node.getAttribute("href");
    if (!href) {
      return;
    }
    node.setAttribute("rel", "noreferrer noopener");
    node.setAttribute("target", "_blank");
  });
}

export function toSanitizedMarkdownHtml(markdown: string): string {
  const input = markdown.trim();
  if (!input) {
    return "";
  }
  installHooks();
  if (input.length <= MARKDOWN_CACHE_MAX_CHARS) {
    const cached = getCachedMarkdown(input);
    if (cached !== null) {
      return cached;
    }
  }
  const truncated = truncateText(input, MARKDOWN_CHAR_LIMIT);
  const suffix = truncated.truncated
    ? `\n\n… truncated (${truncated.total} chars, showing first ${truncated.text.length}).`
    : "";
  if (truncated.text.length > MARKDOWN_PARSE_LIMIT) {
    const escaped = escapeHtml(`${truncated.text}${suffix}`);
    const html = `<pre class="code-block">${escaped}</pre>`;
    const sanitized = DOMPurify.sanitize(html, {
      ALLOWED_TAGS: allowedTags,
      ALLOWED_ATTR: allowedAttrs,
    });
    if (input.length <= MARKDOWN_CACHE_MAX_CHARS) {
      setCachedMarkdown(input, sanitized);
    }
    return sanitized;
  }
  const rendered = markedInstance.parse(`${truncated.text}${suffix}`) as string;
  const sanitized = DOMPurify.sanitize(rendered, {
    ALLOWED_TAGS: allowedTags,
    ALLOWED_ATTR: allowedAttrs,
  });
  if (input.length <= MARKDOWN_CACHE_MAX_CHARS) {
    setCachedMarkdown(input, sanitized);
  }
  return sanitized;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}
