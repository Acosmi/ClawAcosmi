package discord

import (
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
)

// Discord 文本分块 — 继承自 src/discord/chunk.ts (278L)

// ChunkDiscordTextOpts 分块选项
type ChunkDiscordTextOpts struct {
	MaxChars int // 每条消息最大字符数，默认 2000
	MaxLines int // 每条消息最大行数（软限制），默认 17
}

// ChunkMode 分块模式
type ChunkMode string

const (
	ChunkModeLength  ChunkMode = "length"
	ChunkModeNewline ChunkMode = "newline"
)

const (
	defaultMaxChars = 2000
	defaultMaxLines = 17
)

// openFence 打开的代码围栏信息
type openFence struct {
	indent     string
	markerChar byte
	markerLen  int
	openLine   string
}

var fenceRe = regexp.MustCompile(`^( {0,3})(` + "`" + `{3,}|~{3,})(.*)$`)

func countLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func parseFenceLine(line string) *openFence {
	m := fenceRe.FindStringSubmatch(line)
	if m == nil {
		return nil
	}
	indent := m[1]
	marker := m[2]
	return &openFence{
		indent:     indent,
		markerChar: marker[0],
		markerLen:  len(marker),
		openLine:   line,
	}
}

func closeFenceLine(f *openFence) string {
	return f.indent + strings.Repeat(string(f.markerChar), f.markerLen)
}

func closeFenceIfNeeded(text string, f *openFence) string {
	if f == nil {
		return text
	}
	cl := closeFenceLine(f)
	if text == "" {
		return cl
	}
	if !strings.HasSuffix(text, "\n") {
		return text + "\n" + cl
	}
	return text + cl
}

func splitLongLine(line string, maxChars int, preserveWhitespace bool) []string {
	limit := maxChars
	if limit < 1 {
		limit = 1
	}
	if len(line) <= limit {
		return []string{line}
	}
	var out []string
	remaining := line
	for len(remaining) > limit {
		if preserveWhitespace {
			out = append(out, remaining[:limit])
			remaining = remaining[limit:]
			continue
		}
		window := remaining[:limit]
		breakIdx := -1
		for i := len(window) - 1; i >= 0; i-- {
			if unicode.IsSpace(rune(window[i])) {
				breakIdx = i
				break
			}
		}
		if breakIdx <= 0 {
			breakIdx = limit
		}
		out = append(out, remaining[:breakIdx])
		remaining = remaining[breakIdx:]
	}
	if len(remaining) > 0 {
		out = append(out, remaining)
	}
	return out
}

// ChunkDiscordText 按字符数和行数分块 Discord 文本，保持代码围栏平衡。
func ChunkDiscordText(text string, opts ChunkDiscordTextOpts) []string {
	maxChars := opts.MaxChars
	if maxChars <= 0 {
		maxChars = defaultMaxChars
	}
	maxChars = int(math.Max(1, float64(maxChars)))

	maxLines := opts.MaxLines
	if maxLines <= 0 {
		maxLines = defaultMaxLines
	}
	maxLines = int(math.Max(1, float64(maxLines)))

	body := text
	if strings.TrimSpace(body) == "" {
		return []string{}
	}

	if len(body) <= maxChars && countLines(body) <= maxLines {
		return []string{body}
	}

	lines := strings.Split(body, "\n")
	var chunks []string

	current := ""
	currentLines := 0
	var curFence *openFence

	flush := func() {
		if current == "" {
			return
		}
		payload := closeFenceIfNeeded(current, curFence)
		if strings.TrimSpace(payload) != "" {
			chunks = append(chunks, payload)
		}
		current = ""
		currentLines = 0
		if curFence != nil {
			current = curFence.openLine
			currentLines = 1
		}
	}

	for _, originalLine := range lines {
		fenceInfo := parseFenceLine(originalLine)
		wasInsideFence := curFence != nil
		nextOpenFence := curFence
		if fenceInfo != nil {
			if curFence == nil {
				nextOpenFence = fenceInfo
			} else if curFence.markerChar == fenceInfo.markerChar && fenceInfo.markerLen >= curFence.markerLen {
				nextOpenFence = nil
			}
		}

		reserveChars := 0
		reserveLines := 0
		if nextOpenFence != nil {
			reserveChars = len(closeFenceLine(nextOpenFence)) + 1
			reserveLines = 1
		}
		effectiveMaxChars := maxChars - reserveChars
		effectiveMaxLines := maxLines - reserveLines
		charLimit := effectiveMaxChars
		if charLimit <= 0 {
			charLimit = maxChars
		}
		lineLimit := effectiveMaxLines
		if lineLimit <= 0 {
			lineLimit = maxLines
		}

		prefixLen := 0
		if len(current) > 0 {
			prefixLen = len(current) + 1
		}
		segmentLimit := charLimit - prefixLen
		if segmentLimit < 1 {
			segmentLimit = 1
		}
		segments := splitLongLine(originalLine, segmentLimit, wasInsideFence)

		for segIndex, segment := range segments {
			isLineContinuation := segIndex > 0
			delimiter := ""
			if !isLineContinuation && len(current) > 0 {
				delimiter = "\n"
			}
			addition := delimiter + segment
			nextLen := len(current) + len(addition)
			nextLines := currentLines
			if !isLineContinuation {
				nextLines++
			}

			wouldExceedChars := nextLen > charLimit
			wouldExceedLines := nextLines > lineLimit

			if (wouldExceedChars || wouldExceedLines) && len(current) > 0 {
				flush()
			}

			if len(current) > 0 {
				current += addition
				if !isLineContinuation {
					currentLines++
				}
			} else {
				current = segment
				currentLines = 1
			}
		}

		curFence = nextOpenFence
	}

	if len(current) > 0 {
		payload := closeFenceIfNeeded(current, curFence)
		if strings.TrimSpace(payload) != "" {
			chunks = append(chunks, payload)
		}
	}

	return rebalanceReasoningItalics(text, chunks)
}

// ChunkDiscordTextWithMode 按模式分块 Discord 文本。
func ChunkDiscordTextWithMode(text string, opts ChunkDiscordTextOpts, chunkMode ChunkMode) []string {
	if chunkMode != ChunkModeNewline {
		return ChunkDiscordText(text, opts)
	}
	// newline 模式：使用 autoreply 包的段落感知分块，然后对每个段落再做行数/字符数限制
	limit := opts.MaxChars
	if limit <= 0 {
		limit = defaultMaxChars
	}
	lineChunks := autoreply.ChunkMarkdownTextWithMode(text, limit, autoreply.ChunkModeNewline)
	var chunks []string
	for _, line := range lineChunks {
		nested := ChunkDiscordText(line, opts)
		if len(nested) == 0 && line != "" {
			chunks = append(chunks, line)
			continue
		}
		chunks = append(chunks, nested...)
	}
	return chunks
}

// rebalanceReasoningItalics 重平衡推理输出中的斜体标记。
// 当 Discord 分块分割了以 "Reasoning:\n_" 开头的消息时，
// 在每个分块末尾关闭斜体，在下一个分块开头重新打开。
func rebalanceReasoningItalics(source string, chunks []string) []string {
	if len(chunks) <= 1 {
		return chunks
	}

	opensWithReasoningItalics := strings.HasPrefix(source, "Reasoning:\n_") &&
		strings.HasSuffix(strings.TrimRight(source, " \t\n\r"), "_")
	if !opensWithReasoningItalics {
		return chunks
	}

	adjusted := make([]string, len(chunks))
	copy(adjusted, chunks)

	for i := 0; i < len(adjusted); i++ {
		isLast := i == len(adjusted)-1
		current := adjusted[i]

		// 确保当前分块关闭斜体
		needsClosing := !strings.HasSuffix(strings.TrimRight(current, " \t\n\r"), "_")
		if needsClosing {
			adjusted[i] = current + "_"
		}

		if isLast {
			break
		}

		// 在下一个分块开头重新打开斜体
		next := adjusted[i+1]
		trimmedNext := strings.TrimLeft(next, " \t\n\r")
		leadingWhitespace := next[:len(next)-len(trimmedNext)]
		if !strings.HasPrefix(trimmedNext, "_") {
			adjusted[i+1] = leadingWhitespace + "_" + trimmedNext
		}
	}

	return adjusted
}
