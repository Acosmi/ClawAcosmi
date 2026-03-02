package autoreply

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/openacosmi/claw-acismi/pkg/markdown"
)

// TS 对照: auto-reply/chunk.ts (501L) — 完整围栏感知版。

// DefaultChunkLimit 默认分块大小（字符数）。
// TS 对照: chunk.ts DEFAULT_CHUNK_LIMIT
const DefaultChunkLimit = 4000

// ChunkMode 分块模式。
// TS 对照: chunk.ts ChunkMode
type ChunkMode string

const (
	// ChunkModeLength 长度分块（仅超出 limit 时分块）。
	ChunkModeLength ChunkMode = "length"
	// ChunkModeNewline 段落分块（在空行处分块）。
	ChunkModeNewline ChunkMode = "newline"
)

// ChunkOptions 分块选项（旧版兼容）。
type ChunkOptions struct {
	MaxChunkSize int
}

// ChunkReplyText 将长文本分块（旧版兼容入口）。
// 使用 ChunkText 实现。
func ChunkReplyText(text string, opts *ChunkOptions) []string {
	limit := DefaultChunkLimit
	if opts != nil && opts.MaxChunkSize > 0 {
		limit = opts.MaxChunkSize
	}
	return ChunkText(text, limit)
}

// ---------- 核心分块函数 ----------

// scanParenAwareBreakpoints 扫描窗口中的括号感知断点。
// 在括号内部时不设置 break 点，避免截断括号表达式。
// TS 对照: chunk.ts scanParenAwareBreakpoints
func scanParenAwareBreakpoints(window string, isAllowed func(int) bool) (lastNewline, lastWhitespace int) {
	lastNewline = -1
	lastWhitespace = -1
	depth := 0

	for i, ch := range window {
		if isAllowed != nil && !isAllowed(i) {
			continue
		}
		switch ch {
		case '(':
			depth++
			continue
		case ')':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 {
			continue
		}
		if ch == '\n' {
			lastNewline = i
		} else if unicode.IsSpace(ch) {
			lastWhitespace = i
		}
	}
	return
}

// ChunkText 将文本分块。
// 优先在换行/空白处断开，括号感知。
// TS 对照: chunk.ts chunkText
func ChunkText(text string, limit int) []string {
	if text == "" {
		return nil
	}
	if limit <= 0 {
		return []string{text}
	}
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > limit {
		window := remaining[:limit]
		lastNewline, lastWhitespace := scanParenAwareBreakpoints(window, nil)

		breakIdx := -1
		if lastNewline > 0 {
			breakIdx = lastNewline
		} else if lastWhitespace > 0 {
			breakIdx = lastWhitespace
		}
		if breakIdx <= 0 {
			breakIdx = limit
		}

		rawChunk := strings.TrimRight(remaining[:breakIdx], " \t\n\r")
		if len(rawChunk) > 0 {
			chunks = append(chunks, rawChunk)
		}

		// 如果断在分隔符上，跳过分隔符
		brokeOnSep := breakIdx < len(remaining) && unicode.IsSpace(rune(remaining[breakIdx]))
		nextStart := breakIdx
		if brokeOnSep {
			nextStart = breakIdx + 1
		}
		if nextStart > len(remaining) {
			nextStart = len(remaining)
		}
		remaining = strings.TrimLeft(remaining[nextStart:], " \t\n\r")
	}

	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}
	return chunks
}

// pickSafeBreakIndex 在窗口中选择围栏安全的断点。
// TS 对照: chunk.ts pickSafeBreakIndex
func pickSafeBreakIndex(window string, spans []markdown.FenceSpan) int {
	lastNewline, lastWhitespace := scanParenAwareBreakpoints(window, func(index int) bool {
		return markdown.IsSafeFenceBreak(spans, index)
	})
	if lastNewline > 0 {
		return lastNewline
	}
	if lastWhitespace > 0 {
		return lastWhitespace
	}
	return -1
}

// stripLeadingNewlines 剥离前导换行。
func stripLeadingNewlines(value string) string {
	i := 0
	for i < len(value) && value[i] == '\n' {
		i++
	}
	if i > 0 {
		return value[i:]
	}
	return value
}

// ChunkMarkdownText 围栏感知 Markdown 文本分块。
// 当文本中包含围栏代码块时，确保：
//   - 尽量在围栏外部断开
//   - 不得不在围栏内断开时，自动添加关闭标记并在下一块重开
//
// TS 对照: chunk.ts chunkMarkdownText
func ChunkMarkdownText(text string, limit int) []string {
	if text == "" {
		return nil
	}
	if limit <= 0 {
		return []string{text}
	}
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > limit {
		spans := markdown.ParseFenceSpans(remaining)
		window := remaining[:limit]

		softBreak := pickSafeBreakIndex(window, spans)
		breakIdx := softBreak
		if breakIdx <= 0 {
			breakIdx = limit
		}

		// 检查断点是否在围栏内
		var fenceToSplit *markdown.FenceSpan
		initialFence := markdown.FindFenceSpanAt(spans, breakIdx)

		if initialFence != nil {
			closeLine := initialFence.Indent + initialFence.Marker
			maxIdxIfNeedNewline := limit - (len(closeLine) + 1)

			if maxIdxIfNeedNewline <= 0 {
				// 无法容纳关闭标记，硬断
				breakIdx = limit
			} else {
				minProgressIdx := initialFence.Start + len(initialFence.OpenLine) + 2
				if minProgressIdx > len(remaining) {
					minProgressIdx = len(remaining)
				}
				maxIdxIfAlreadyNewline := limit - len(closeLine)

				pickedNewline := false
				lastNL := strings.LastIndex(remaining[:max(0, maxIdxIfAlreadyNewline)], "\n")
				for lastNL != -1 {
					candidateBreak := lastNL + 1
					if candidateBreak < minProgressIdx {
						break
					}
					candidateFence := markdown.FindFenceSpanAt(spans, candidateBreak)
					if candidateFence != nil && candidateFence.Start == initialFence.Start {
						if candidateBreak > 0 {
							breakIdx = candidateBreak
						} else {
							breakIdx = 1
						}
						pickedNewline = true
						break
					}
					if lastNL == 0 {
						break
					}
					lastNL = strings.LastIndex(remaining[:lastNL], "\n")
				}

				if !pickedNewline {
					if minProgressIdx > maxIdxIfAlreadyNewline {
						breakIdx = limit
					} else {
						breakIdx = max(minProgressIdx, maxIdxIfNeedNewline)
					}
				}
			}

			fenceAtBreak := markdown.FindFenceSpanAt(spans, breakIdx)
			if fenceAtBreak != nil && fenceAtBreak.Start == initialFence.Start {
				fenceToSplit = fenceAtBreak
			}
		}

		rawChunk := remaining[:breakIdx]
		if rawChunk == "" {
			break
		}

		brokeOnSep := breakIdx < len(remaining) && unicode.IsSpace(rune(remaining[breakIdx]))
		nextStart := breakIdx
		if brokeOnSep {
			nextStart = breakIdx + 1
		}
		if nextStart > len(remaining) {
			nextStart = len(remaining)
		}
		next := remaining[nextStart:]

		if fenceToSplit != nil {
			closeLine := fenceToSplit.Indent + fenceToSplit.Marker
			if strings.HasSuffix(rawChunk, "\n") {
				rawChunk = rawChunk + closeLine
			} else {
				rawChunk = rawChunk + "\n" + closeLine
			}
			next = fenceToSplit.OpenLine + "\n" + next
		} else {
			next = stripLeadingNewlines(next)
		}

		chunks = append(chunks, rawChunk)
		remaining = next
	}

	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}
	return chunks
}

// ---------- 段落分块 ----------

// paragraphBreakRe 段落分隔符：包含空白行的位置。
var paragraphBreakRe = regexp.MustCompile(`\n[\t ]*\n+`)

// ChunkByParagraph 在段落边界（空行）处分块。
// 保留列表和单换行内容在同一段落中。围栏内空行不会被当作分隔符。
// TS 对照: chunk.ts chunkByParagraph
func ChunkByParagraph(text string, limit int, splitLongParagraphs bool) []string {
	if text == "" {
		return nil
	}
	if limit <= 0 {
		return []string{text}
	}

	// 规范化 \r\n → \n
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// 快速路径：无段落分隔符
	if !paragraphBreakRe.MatchString(normalized) {
		if len(normalized) <= limit {
			return []string{normalized}
		}
		if !splitLongParagraphs {
			return []string{normalized}
		}
		return ChunkText(normalized, limit)
	}

	spans := markdown.ParseFenceSpans(normalized)

	// 找段落分隔位置（跳过围栏内的）
	var parts []string
	matches := paragraphBreakRe.FindAllStringIndex(normalized, -1)
	lastIndex := 0
	for _, match := range matches {
		idx := match[0]
		// 不在围栏内才分割
		if !markdown.IsSafeFenceBreak(spans, idx) {
			continue
		}
		parts = append(parts, normalized[lastIndex:idx])
		lastIndex = match[1]
	}
	parts = append(parts, normalized[lastIndex:])

	var chunks []string
	for _, part := range parts {
		paragraph := strings.TrimRight(part, " \t\n\r")
		if strings.TrimSpace(paragraph) == "" {
			continue
		}
		if len(paragraph) <= limit {
			chunks = append(chunks, paragraph)
		} else if !splitLongParagraphs {
			chunks = append(chunks, paragraph)
		} else {
			chunks = append(chunks, ChunkText(paragraph, limit)...)
		}
	}
	return chunks
}

// ---------- 模式统一入口 ----------

// ChunkTextWithMode 按模式分块。
// TS 对照: chunk.ts chunkTextWithMode
func ChunkTextWithMode(text string, limit int, mode ChunkMode) []string {
	if mode == ChunkModeNewline {
		return ChunkByParagraph(text, limit, true)
	}
	return ChunkText(text, limit)
}

// ChunkMarkdownTextWithMode Markdown 感知按模式分块。
// TS 对照: chunk.ts chunkMarkdownTextWithMode
func ChunkMarkdownTextWithMode(text string, limit int, mode ChunkMode) []string {
	if mode == ChunkModeNewline {
		// 段落分块（不拆分长段落），超长段落交给 Markdown 分块
		paragraphChunks := ChunkByParagraph(text, limit, false)
		var out []string
		for _, chunk := range paragraphChunks {
			nested := ChunkMarkdownText(chunk, limit)
			if len(nested) == 0 && chunk != "" {
				out = append(out, chunk)
			} else {
				out = append(out, nested...)
			}
		}
		return out
	}
	return ChunkMarkdownText(text, limit)
}

// ---------- 配置解析 ----------

// ProviderChunkConfig 频道级分块配置。
// TS 对照: chunk.ts ProviderChunkConfig
type ProviderChunkConfig struct {
	TextChunkLimit int                           `json:"textChunkLimit,omitempty"`
	ChunkMode      ChunkMode                     `json:"chunkMode,omitempty"`
	Accounts       map[string]AccountChunkConfig `json:"accounts,omitempty"`
}

// AccountChunkConfig 账号级分块配置。
type AccountChunkConfig struct {
	TextChunkLimit int       `json:"textChunkLimit,omitempty"`
	ChunkMode      ChunkMode `json:"chunkMode,omitempty"`
}

// ResolveTextChunkLimit 从配置中解析分块大小。
// TS 对照: chunk.ts resolveTextChunkLimit
func ResolveTextChunkLimit(providerConfig *ProviderChunkConfig, accountID string, fallbackLimit int) int {
	fallback := DefaultChunkLimit
	if fallbackLimit > 0 {
		fallback = fallbackLimit
	}
	if providerConfig == nil {
		return fallback
	}
	// 账号级覆盖
	if accountID != "" && providerConfig.Accounts != nil {
		normalizedID := strings.ToLower(accountID)
		if acct, ok := providerConfig.Accounts[accountID]; ok && acct.TextChunkLimit > 0 {
			return acct.TextChunkLimit
		}
		// 大小写不敏感查找
		for key, acct := range providerConfig.Accounts {
			if strings.ToLower(key) == normalizedID && acct.TextChunkLimit > 0 {
				return acct.TextChunkLimit
			}
		}
	}
	// 频道级覆盖
	if providerConfig.TextChunkLimit > 0 {
		return providerConfig.TextChunkLimit
	}
	return fallback
}

// ResolveChunkMode 从配置中解析分块模式。
// TS 对照: chunk.ts resolveChunkMode
func ResolveChunkMode(providerConfig *ProviderChunkConfig, accountID string) ChunkMode {
	if providerConfig == nil {
		return ChunkModeLength
	}
	// 账号级覆盖
	if accountID != "" && providerConfig.Accounts != nil {
		normalizedID := strings.ToLower(accountID)
		if acct, ok := providerConfig.Accounts[accountID]; ok && acct.ChunkMode != "" {
			return acct.ChunkMode
		}
		for key, acct := range providerConfig.Accounts {
			if strings.ToLower(key) == normalizedID && acct.ChunkMode != "" {
				return acct.ChunkMode
			}
		}
	}
	if providerConfig.ChunkMode != "" {
		return providerConfig.ChunkMode
	}
	return ChunkModeLength
}
