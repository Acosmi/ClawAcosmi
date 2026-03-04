package telegram

import (
	"fmt"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/pkg/markdown"
)

// Telegram 格式化 — 继承自 src/telegram/format.ts (102L)
// 使用 markdown IR 中间表示层 + renderMarkdownWithMarkers 实现 Telegram HTML 输出。

// TelegramFormattedChunk 格式化后的文本块
type TelegramFormattedChunk struct {
	HTML string
	Text string
}

// MarkdownTableMode Markdown 表格模式
type MarkdownTableMode string

const (
	TableModeDefault MarkdownTableMode = ""
	TableModeOff     MarkdownTableMode = "off"
	TableModeBullets MarkdownTableMode = "bullets"
	TableModeCode    MarkdownTableMode = "code"
)

// EscapeHTML 转义 HTML 特殊字符（Telegram HTML parse mode 要求）
func EscapeHTML(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")
	return text
}

// escapeHTMLAttr 转义 HTML 属性值
func escapeHTMLAttr(text string) string {
	return strings.ReplaceAll(EscapeHTML(text), "\"", "&quot;")
}

// telegramHTMLStyleMarkers Telegram HTML 的样式标记映射。
// TS 对照: telegram/format.ts telegramFormatMap
var telegramHTMLStyleMarkers = markdown.RenderStyleMap{
	markdown.StyleBold:          {Open: "<b>", Close: "</b>"},
	markdown.StyleItalic:        {Open: "<i>", Close: "</i>"},
	markdown.StyleStrikethrough: {Open: "<s>", Close: "</s>"},
	markdown.StyleCode:          {Open: "<code>", Close: "</code>"},
	markdown.StyleCodeBlock:     {Open: "<pre><code>", Close: "</code></pre>"},
	markdown.StyleSpoiler:       {Open: "<tg-spoiler>", Close: "</tg-spoiler>"},
}

// telegramBuildLink 构建 Telegram HTML 链接 <a href="url">label</a>。
func telegramBuildLink(link markdown.MarkdownLinkSpan, text string) *markdown.RenderLink {
	href := strings.TrimSpace(link.Href)
	if href == "" {
		return nil
	}
	return &markdown.RenderLink{
		Start: link.Start,
		End:   link.End,
		Open:  fmt.Sprintf(`<a href="%s">`, escapeHTMLAttr(href)),
		Close: "</a>",
	}
}

// MarkdownToTelegramHTML 将 Markdown 文本转换为 Telegram HTML。
// 使用 markdown IR + RenderMarkdownWithMarkers。
func MarkdownToTelegramHTML(md string, tableMode MarkdownTableMode) string {
	if md == "" {
		return ""
	}
	ir := markdown.MarkdownToIR(md, &markdown.MarkdownParseOptions{
		HeadingStyle:     "none",
		EnableSpoilers:   true,
		BlockquotePrefix: "",
		TableMode:        string(tableMode),
	})
	return markdown.RenderMarkdownWithMarkers(ir, markdown.RenderOptions{
		StyleMarkers: telegramHTMLStyleMarkers,
		EscapeText:   EscapeHTML,
		BuildLink:    telegramBuildLink,
	})
}

// RenderTelegramHTMLText 根据 textMode 将文本渲染为 Telegram HTML。
// textMode="html" 直接返回，否则视为 Markdown 转换。
func RenderTelegramHTMLText(text string, textMode string, tableMode MarkdownTableMode) string {
	if textMode == "html" {
		return text
	}
	return MarkdownToTelegramHTML(text, tableMode)
}

// MarkdownToTelegramChunks 将 Markdown 分块转换为 Telegram HTML。
// 使用 ChunkMarkdownIR 进行围栏感知分块，保留代码块完整性。
func MarkdownToTelegramChunks(md string, limit int, tableMode MarkdownTableMode) []TelegramFormattedChunk {
	if md == "" {
		return nil
	}

	parseOpts := &markdown.MarkdownParseOptions{
		HeadingStyle:     "none",
		EnableSpoilers:   true,
		BlockquotePrefix: "",
		TableMode:        string(tableMode),
	}
	renderOpts := markdown.RenderOptions{
		StyleMarkers: telegramHTMLStyleMarkers,
		EscapeText:   EscapeHTML,
		BuildLink:    telegramBuildLink,
	}

	ir := markdown.MarkdownToIR(md, parseOpts)

	// 单块优化（使用字节长度，与 ChunkMarkdownIR 内部一致）
	rendered := markdown.RenderMarkdownWithMarkers(ir, renderOpts)
	if limit <= 0 || len(ir.Text) <= limit {
		return []TelegramFormattedChunk{{HTML: rendered, Text: ir.Text}}
	}

	// IR 分块（围栏感知）
	irChunks := markdown.ChunkMarkdownIR(ir, limit, autoreply.ChunkMarkdownText)

	results := make([]TelegramFormattedChunk, 0, len(irChunks))
	for _, chunk := range irChunks {
		html := markdown.RenderMarkdownWithMarkers(chunk, renderOpts)
		if html != "" {
			results = append(results, TelegramFormattedChunk{
				HTML: html,
				Text: chunk.Text,
			})
		}
	}
	if len(results) == 0 && rendered != "" {
		results = []TelegramFormattedChunk{{HTML: rendered, Text: ir.Text}}
	}
	return results
}

// MarkdownToTelegramHTMLChunks 将 Markdown 分块转换为 HTML 字符串列表。
func MarkdownToTelegramHTMLChunks(md string, limit int) []string {
	chunks := MarkdownToTelegramChunks(md, limit, TableModeDefault)
	result := make([]string, len(chunks))
	for i, chunk := range chunks {
		result[i] = chunk.HTML
	}
	return result
}
