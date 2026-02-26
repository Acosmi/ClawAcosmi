// ir.go — Markdown 中间表示 (IR) 类型定义与工具函数。
//
// TS 对照: markdown/ir.ts (882L)
//
// 本文件包含:
//   - MarkdownIR / MarkdownStyleSpan / MarkdownLinkSpan 类型
//   - MarkdownParseOptions 解析选项
//   - span clamp / merge / slice 工具函数
//   - MarkdownToIR 入口 (简化版，不依赖 markdown-it)
//   - ChunkMarkdownIR 围栏感知 IR 分块
//
// TS 原版使用 markdown-it npm 包做解析，Go 版使用正则+状态机
// 实现基础解析（bold, italic, strikethrough, code, links）。
// 表格处理由 tables.go 负责。
package markdown

import (
	"sort"
	"strings"
)

// MarkdownStyle 样式类型。
// TS 对照: ir.ts MarkdownStyle
type MarkdownStyle string

const (
	StyleBold          MarkdownStyle = "bold"
	StyleItalic        MarkdownStyle = "italic"
	StyleStrikethrough MarkdownStyle = "strikethrough"
	StyleCode          MarkdownStyle = "code"
	StyleCodeBlock     MarkdownStyle = "code_block"
	StyleSpoiler       MarkdownStyle = "spoiler"
)

// MarkdownStyleSpan 文本中的样式范围。
// TS 对照: ir.ts MarkdownStyleSpan
type MarkdownStyleSpan struct {
	Start int           `json:"start"`
	End   int           `json:"end"`
	Style MarkdownStyle `json:"style"`
}

// MarkdownLinkSpan 文本中的链接范围。
// TS 对照: ir.ts MarkdownLinkSpan
type MarkdownLinkSpan struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Href  string `json:"href"`
}

// MarkdownIR Markdown 中间表示。
// TS 对照: ir.ts MarkdownIR
type MarkdownIR struct {
	Text   string              `json:"text"`
	Styles []MarkdownStyleSpan `json:"styles"`
	Links  []MarkdownLinkSpan  `json:"links"`
}

// MarkdownParseOptions Markdown 解析选项。
// TS 对照: ir.ts MarkdownParseOptions
type MarkdownParseOptions struct {
	// Linkify 自动检测裸 URL 并转为链接。默认 true。
	Linkify *bool
	// EnableSpoilers 启用 || spoiler || 语法。
	EnableSpoilers bool
	// HeadingStyle 标题渲染方式: "none" 或 "bold"。
	HeadingStyle string
	// BlockquotePrefix 引用块前缀。
	BlockquotePrefix string
	// Autolink 自动链接，默认 true。
	Autolink *bool
	// TableMode 表格渲染模式 "off"|"bullets"|"code"。
	TableMode string
}

// MarkdownToIRResult MarkdownToIRWithMeta() 的返回值。
type MarkdownToIRResult struct {
	IR        MarkdownIR
	HasTables bool
}

// MarkdownToIR 将 Markdown 文本转为中间表示。
// 简化版 — 仅提取纯文本、基本样式和链接。
// TS 对照: ir.ts markdownToIR()
func MarkdownToIR(markdown string, opts *MarkdownParseOptions) MarkdownIR {
	return MarkdownToIRWithMeta(markdown, opts).IR
}

// MarkdownToIRWithMeta 将 Markdown 转为 IR 并返回元信息。
// TS 对照: ir.ts markdownToIRWithMeta()
func MarkdownToIRWithMeta(markdown string, opts *MarkdownParseOptions) MarkdownToIRResult {
	if opts == nil {
		opts = &MarkdownParseOptions{}
	}

	state := &renderState{
		headingStyle:     opts.HeadingStyle,
		blockquotePrefix: opts.BlockquotePrefix,
		enableSpoilers:   opts.EnableSpoilers,
	}

	// 使用简化解析器
	parseMarkdownSimple(markdown, state)

	// 裁剪尾部空白
	trimmedText := strings.TrimRight(state.text.String(), " \t\n\r")
	trimmedLen := len(trimmedText)

	// 确保 code_block 范围被保留
	codeBlockEnd := 0
	for _, span := range state.styles {
		if span.Style == StyleCodeBlock && span.End > codeBlockEnd {
			codeBlockEnd = span.End
		}
	}
	finalLen := trimmedLen
	if codeBlockEnd > finalLen {
		finalLen = codeBlockEnd
	}

	fullText := state.text.String()
	finalText := fullText
	if finalLen < len(fullText) {
		finalText = fullText[:finalLen]
	}

	return MarkdownToIRResult{
		IR: MarkdownIR{
			Text:   finalText,
			Styles: MergeStyleSpans(ClampStyleSpans(state.styles, finalLen)),
			Links:  ClampLinkSpans(state.links, finalLen),
		},
		HasTables: state.hasTables,
	}
}

// ---------- 内部简化解析器 ----------

type renderState struct {
	text             strings.Builder
	styles           []MarkdownStyleSpan
	links            []MarkdownLinkSpan
	headingStyle     string
	blockquotePrefix string
	enableSpoilers   bool
	hasTables        bool
}

func (s *renderState) appendText(value string) {
	s.text.WriteString(value)
}

func (s *renderState) pos() int {
	return s.text.Len()
}

// parseMarkdownSimple 轻量 Markdown 解析器。
// 逐行解析，处理围栏代码块、标题、列表、引用块，行内解析 bold/italic/code/link。
func parseMarkdownSimple(md string, state *renderState) {
	if md == "" {
		return
	}

	lines := strings.Split(md, "\n")
	fences := ParseFenceSpans(md)
	fenceIdx := 0

	// 跟踪当前字节偏移用于 fence 匹配
	offset := 0
	inFence := false
	fenceStart := 0

	for _, line := range lines {
		lineEnd := offset + len(line)

		// 检查是否进入/退出围栏代码块
		for fenceIdx < len(fences) && fences[fenceIdx].Start < lineEnd {
			f := fences[fenceIdx]
			if offset >= f.Start && offset <= f.End {
				if !inFence {
					inFence = true
					fenceStart = state.pos()
				}
				if lineEnd >= f.End {
					// 围栏结束
					state.appendText(line + "\n")
					state.styles = append(state.styles, MarkdownStyleSpan{
						Start: fenceStart,
						End:   state.pos(),
						Style: StyleCodeBlock,
					})
					inFence = false
					fenceIdx++
					goto nextLine
				}
			}
			break
		}

		if inFence {
			state.appendText(line + "\n")
			goto nextLine
		}

		// 空行 → 段落分隔
		if strings.TrimSpace(line) == "" {
			text := state.text.String()
			if len(text) > 0 && !strings.HasSuffix(text, "\n\n") {
				state.appendText("\n\n")
			}
			goto nextLine
		}

		// 标题
		if strings.HasPrefix(line, "#") {
			level, content := parseHeadingLine(line)
			if level > 0 {
				if state.headingStyle == "bold" {
					start := state.pos()
					parseInlineMarkdown(content, state)
					state.styles = append(state.styles, MarkdownStyleSpan{
						Start: start,
						End:   state.pos(),
						Style: StyleBold,
					})
				} else {
					parseInlineMarkdown(content, state)
				}
				state.appendText("\n\n")
				goto nextLine
			}
		}

		// 引用块
		if strings.HasPrefix(line, ">") {
			content := strings.TrimPrefix(line, ">")
			content = strings.TrimPrefix(content, " ")
			if state.blockquotePrefix != "" {
				state.appendText(state.blockquotePrefix)
			}
			parseInlineMarkdown(content, state)
			state.appendText("\n")
			goto nextLine
		}

		// 无序列表
		if len(line) >= 2 && (line[0] == '-' || line[0] == '*' || line[0] == '+') && line[1] == ' ' {
			state.appendText("• ")
			parseInlineMarkdown(strings.TrimSpace(line[2:]), state)
			state.appendText("\n")
			goto nextLine
		}

		// 有序列表
		if idx := parseOrderedListPrefix(line); idx > 0 {
			content := line[idx:]
			state.appendText(line[:idx])
			parseInlineMarkdown(strings.TrimSpace(content), state)
			state.appendText("\n")
			goto nextLine
		}

		// 水平线
		if isHorizontalRule(line) {
			state.appendText("\n")
			goto nextLine
		}

		// 普通段落
		parseInlineMarkdown(line, state)
		state.appendText("\n")

	nextLine:
		offset = lineEnd + 1 // +1 for the \n
	}
}

// parseHeadingLine 解析标题行 (# ... ######)。
func parseHeadingLine(line string) (int, string) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, ""
	}
	if level < len(line) && line[level] != ' ' {
		return 0, ""
	}
	content := ""
	if level < len(line) {
		content = strings.TrimSpace(line[level+1:])
	}
	return level, content
}

// parseOrderedListPrefix 解析有序列表前缀 "1. "，返回前缀长度。
func parseOrderedListPrefix(line string) int {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(line) {
		return 0
	}
	if line[i] == '.' && i+1 < len(line) && line[i+1] == ' ' {
		return i + 2
	}
	return 0
}

// isHorizontalRule 检查是否为水平线 (---, ***, ___)。
func isHorizontalRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	ch := trimmed[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	for _, c := range trimmed {
		if c != rune(ch) && c != ' ' {
			return false
		}
	}
	return true
}

// parseInlineMarkdown 解析行内 Markdown（bold, italic, strikethrough, code, links, spoilers）。
func parseInlineMarkdown(text string, state *renderState) {
	i := 0
	n := len(text)

	for i < n {
		ch := text[i]

		// 行内代码
		if ch == '`' {
			tickStart := i
			ticks := 0
			for i < n && text[i] == '`' {
				ticks++
				i++
			}
			// 查找匹配的关闭反引号
			closeIdx := strings.Index(text[i:], strings.Repeat("`", ticks))
			if closeIdx >= 0 {
				content := text[i : i+closeIdx]
				start := state.pos()
				state.appendText(content)
				state.styles = append(state.styles, MarkdownStyleSpan{
					Start: start,
					End:   state.pos(),
					Style: StyleCode,
				})
				i += closeIdx + ticks
			} else {
				state.appendText(text[tickStart:i])
			}
			continue
		}

		// Markdown 链接 [text](url)
		if ch == '[' {
			linkEnd := parseLinkSyntax(text, i, state)
			if linkEnd > i {
				i = linkEnd
				continue
			}
		}

		// 粗体 **text** 或 __text__
		if i+1 < n && ((ch == '*' && text[i+1] == '*') || (ch == '_' && text[i+1] == '_')) {
			marker := string([]byte{ch, ch})
			closeIdx := strings.Index(text[i+2:], marker)
			if closeIdx >= 0 {
				start := state.pos()
				parseInlineMarkdown(text[i+2:i+2+closeIdx], state)
				state.styles = append(state.styles, MarkdownStyleSpan{
					Start: start,
					End:   state.pos(),
					Style: StyleBold,
				})
				i += 2 + closeIdx + 2
				continue
			}
		}

		// 删除线 ~~text~~
		if i+1 < n && ch == '~' && text[i+1] == '~' {
			closeIdx := strings.Index(text[i+2:], "~~")
			if closeIdx >= 0 {
				start := state.pos()
				parseInlineMarkdown(text[i+2:i+2+closeIdx], state)
				state.styles = append(state.styles, MarkdownStyleSpan{
					Start: start,
					End:   state.pos(),
					Style: StyleStrikethrough,
				})
				i += 2 + closeIdx + 2
				continue
			}
		}

		// Spoiler ||text||
		if state.enableSpoilers && i+1 < n && ch == '|' && text[i+1] == '|' {
			closeIdx := strings.Index(text[i+2:], "||")
			if closeIdx >= 0 {
				start := state.pos()
				parseInlineMarkdown(text[i+2:i+2+closeIdx], state)
				state.styles = append(state.styles, MarkdownStyleSpan{
					Start: start,
					End:   state.pos(),
					Style: StyleSpoiler,
				})
				i += 2 + closeIdx + 2
				continue
			}
		}

		// 斜体 *text* 或 _text_
		if ch == '*' || ch == '_' {
			closeIdx := strings.IndexByte(text[i+1:], ch)
			if closeIdx >= 0 {
				start := state.pos()
				parseInlineMarkdown(text[i+1:i+1+closeIdx], state)
				state.styles = append(state.styles, MarkdownStyleSpan{
					Start: start,
					End:   state.pos(),
					Style: StyleItalic,
				})
				i += 1 + closeIdx + 1
				continue
			}
		}

		// 普通字符
		state.appendText(string(ch))
		i++
	}
}

// parseLinkSyntax 解析 [text](url) 链接语法。
// 返回解析后的下一个位置，如果不是链接返回 start。
func parseLinkSyntax(text string, start int, state *renderState) int {
	// 查找 ]
	closeIdx := strings.IndexByte(text[start+1:], ']')
	if closeIdx < 0 {
		return start
	}
	labelEnd := start + 1 + closeIdx

	// 期望 (
	if labelEnd+1 >= len(text) || text[labelEnd+1] != '(' {
		return start
	}

	// 查找 )
	urlStart := labelEnd + 2
	closeParenIdx := strings.IndexByte(text[urlStart:], ')')
	if closeParenIdx < 0 {
		return start
	}

	label := text[start+1 : labelEnd]
	href := strings.TrimSpace(text[urlStart : urlStart+closeParenIdx])

	linkStart := state.pos()
	parseInlineMarkdown(label, state)
	linkEnd := state.pos()

	if linkEnd > linkStart && href != "" {
		state.links = append(state.links, MarkdownLinkSpan{
			Start: linkStart,
			End:   linkEnd,
			Href:  href,
		})
	}

	return urlStart + closeParenIdx + 1
}

// ---------- Span 工具函数 ----------

// ClampStyleSpans 将样式 span 裁剪到 [0, maxLength) 范围。
// TS 对照: ir.ts clampStyleSpans()
func ClampStyleSpans(spans []MarkdownStyleSpan, maxLength int) []MarkdownStyleSpan {
	var clamped []MarkdownStyleSpan
	for _, span := range spans {
		start := clampInt(span.Start, 0, maxLength)
		end := clampInt(span.End, start, maxLength)
		if end > start {
			clamped = append(clamped, MarkdownStyleSpan{Start: start, End: end, Style: span.Style})
		}
	}
	return clamped
}

// ClampLinkSpans 将链接 span 裁剪到 [0, maxLength) 范围。
// TS 对照: ir.ts clampLinkSpans()
func ClampLinkSpans(spans []MarkdownLinkSpan, maxLength int) []MarkdownLinkSpan {
	var clamped []MarkdownLinkSpan
	for _, span := range spans {
		start := clampInt(span.Start, 0, maxLength)
		end := clampInt(span.End, start, maxLength)
		if end > start {
			clamped = append(clamped, MarkdownLinkSpan{Start: start, End: end, Href: span.Href})
		}
	}
	return clamped
}

// MergeStyleSpans 合并相邻/重叠的同类样式 span。
// TS 对照: ir.ts mergeStyleSpans()
func MergeStyleSpans(spans []MarkdownStyleSpan) []MarkdownStyleSpan {
	if len(spans) == 0 {
		return nil
	}
	sorted := make([]MarkdownStyleSpan, len(spans))
	copy(sorted, spans)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Start != sorted[j].Start {
			return sorted[i].Start < sorted[j].Start
		}
		if sorted[i].End != sorted[j].End {
			return sorted[i].End < sorted[j].End
		}
		return sorted[i].Style < sorted[j].Style
	})

	var merged []MarkdownStyleSpan
	for _, span := range sorted {
		if len(merged) > 0 {
			prev := &merged[len(merged)-1]
			if prev.Style == span.Style && span.Start <= prev.End {
				if span.End > prev.End {
					prev.End = span.End
				}
				continue
			}
		}
		merged = append(merged, span)
	}
	return merged
}

// SliceStyleSpans 提取 [start, end) 范围内的样式 span 并重新偏移。
// TS 对照: ir.ts sliceStyleSpans()
func SliceStyleSpans(spans []MarkdownStyleSpan, start, end int) []MarkdownStyleSpan {
	if len(spans) == 0 {
		return nil
	}
	var sliced []MarkdownStyleSpan
	for _, span := range spans {
		s := max(span.Start, start)
		e := min(span.End, end)
		if e > s {
			sliced = append(sliced, MarkdownStyleSpan{
				Start: s - start,
				End:   e - start,
				Style: span.Style,
			})
		}
	}
	return MergeStyleSpans(sliced)
}

// SliceLinkSpans 提取 [start, end) 范围内的链接 span 并重新偏移。
// TS 对照: ir.ts sliceLinkSpans()
func SliceLinkSpans(spans []MarkdownLinkSpan, start, end int) []MarkdownLinkSpan {
	if len(spans) == 0 {
		return nil
	}
	var sliced []MarkdownLinkSpan
	for _, span := range spans {
		s := max(span.Start, start)
		e := min(span.End, end)
		if e > s {
			sliced = append(sliced, MarkdownLinkSpan{
				Start: s - start,
				End:   e - start,
				Href:  span.Href,
			})
		}
	}
	return sliced
}

// ---------- IR 分块 ----------

// TextChunkerFunc 文本分块函数签名。
// 由调用方注入（例如 autoreply.ChunkMarkdownText），避免循环依赖。
type TextChunkerFunc func(text string, limit int) []string

// ChunkMarkdownIR 将 MarkdownIR 按文本分块，保留各 chunk 的 styles 和 links。
// chunker 参数避免 pkg/markdown → internal/autoreply 的循环依赖。
// TS 对照: ir.ts chunkMarkdownIR()
func ChunkMarkdownIR(ir MarkdownIR, limit int, chunker TextChunkerFunc) []MarkdownIR {
	if len(ir.Text) <= limit || limit <= 0 {
		return []MarkdownIR{ir}
	}

	textChunks := chunker(ir.Text, limit)
	if len(textChunks) <= 1 {
		return []MarkdownIR{ir}
	}

	result := make([]MarkdownIR, 0, len(textChunks))
	offset := 0
	for _, chunk := range textChunks {
		chunkLen := len(chunk)
		// 在原文中查找该 chunk 的偏移
		idx := findChunkOffset(ir.Text, chunk, offset)
		if idx < 0 {
			idx = offset
		}
		end := idx + chunkLen

		chunkIR := MarkdownIR{
			Text:   chunk,
			Styles: SliceStyleSpans(ir.Styles, idx, end),
			Links:  SliceLinkSpans(ir.Links, idx, end),
		}
		result = append(result, chunkIR)
		offset = end
	}
	return result
}

// findChunkOffset 在文本中查找 chunk 的精确位置。
// 从 startOffset 开始搜索。
func findChunkOffset(text, chunk string, startOffset int) int {
	if startOffset >= len(text) {
		return -1
	}
	idx := strings.Index(text[startOffset:], chunk)
	if idx < 0 {
		return -1
	}
	return startOffset + idx
}

// clampInt 将 v 限制在 [lo, hi] 范围内。
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
