// code_spans.go — 行内代码 span 解析。
//
// TS 对照: markdown/code-spans.ts (106L)
//
// 识别 Markdown 中的行内代码（`code`）和围栏代码块，
// 提供 CodeSpanIndex 用于判断任意字节偏移是否在代码上下文中。
// 供 Markdown 格式化和分块逻辑使用。
package markdown

// InlineCodeState 跨文本块的行内代码解析状态。
// 用于流式/分块解析时保持跨块连续性。
// TS 对照: code-spans.ts InlineCodeState
type InlineCodeState struct {
	// Open 当前是否有未关闭的行内代码段。
	Open bool
	// Ticks 开始标记的反引号数量。
	Ticks int
}

// NewInlineCodeState 创建初始行内代码状态。
// TS 对照: code-spans.ts createInlineCodeState()
func NewInlineCodeState() InlineCodeState {
	return InlineCodeState{Open: false, Ticks: 0}
}

// inlineCodeSpansResult 行内代码 span 解析结果。
type inlineCodeSpansResult struct {
	spans [][2]int
	state InlineCodeState
}

// CodeSpanIndex 代码 span 索引，提供快速查询。
// TS 对照: code-spans.ts CodeSpanIndex
type CodeSpanIndex struct {
	// InlineState 解析完成后的行内代码状态（可用于续接下一块）。
	InlineState InlineCodeState

	fenceSpans  []FenceSpan
	inlineSpans [][2]int
}

// IsInside 判断给定字节偏移是否在代码上下文（围栏或行内代码）中。
// TS 对照: code-spans.ts CodeSpanIndex.isInside()
func (idx *CodeSpanIndex) IsInside(index int) bool {
	return isInsideFenceSpanRange(index, idx.fenceSpans) ||
		isInsideInlineSpanRange(index, idx.inlineSpans)
}

// BuildCodeSpanIndex 构建代码 span 索引。
// 解析文本中的围栏代码块和行内代码，提供 IsInside 查询方法。
// TS 对照: code-spans.ts buildCodeSpanIndex()
func BuildCodeSpanIndex(text string, inlineState *InlineCodeState) *CodeSpanIndex {
	fenceSpans := ParseFenceSpans(text)

	startState := NewInlineCodeState()
	if inlineState != nil {
		startState = InlineCodeState{Open: inlineState.Open, Ticks: inlineState.Ticks}
	}

	result := parseInlineCodeSpans(text, fenceSpans, startState)

	return &CodeSpanIndex{
		InlineState: result.state,
		fenceSpans:  fenceSpans,
		inlineSpans: result.spans,
	}
}

// parseInlineCodeSpans 解析行内代码 span。
// 跳过围栏代码块区域，匹配反引号对。
// TS 对照: code-spans.ts parseInlineCodeSpans()
func parseInlineCodeSpans(text string, fenceSpans []FenceSpan, initialState InlineCodeState) inlineCodeSpansResult {
	var spans [][2]int
	open := initialState.Open
	ticks := initialState.Ticks
	openStart := -1
	if open {
		openStart = 0
	}

	bytes := []byte(text)
	i := 0
	n := len(bytes)

	for i < n {
		// 检查是否在围栏区域内（含边界）
		if fence := findFenceSpanAtInclusive(fenceSpans, i); fence != nil {
			i = fence.End
			continue
		}

		if bytes[i] != '`' {
			i++
			continue
		}

		// 计算连续反引号长度
		runStart := i
		runLength := 0
		for i < n && bytes[i] == '`' {
			runLength++
			i++
		}

		if !open {
			open = true
			ticks = runLength
			openStart = runStart
			continue
		}

		if runLength == ticks {
			spans = append(spans, [2]int{openStart, i})
			open = false
			ticks = 0
			openStart = -1
		}
	}

	// 未关闭的行内代码延伸到文本末尾
	if open {
		spans = append(spans, [2]int{openStart, n})
	}

	return inlineCodeSpansResult{
		spans: spans,
		state: InlineCodeState{Open: open, Ticks: ticks},
	}
}

// findFenceSpanAtInclusive 查找包含给定偏移的围栏 span（含开始边界）。
// 注意：与 FindFenceSpanAt 不同，此函数 start 边界是包含的（>=）。
// TS 对照: code-spans.ts findFenceSpanAtInclusive()
func findFenceSpanAtInclusive(spans []FenceSpan, index int) *FenceSpan {
	for i := range spans {
		if index >= spans[i].Start && index < spans[i].End {
			return &spans[i]
		}
	}
	return nil
}

// isInsideFenceSpanRange 判断偏移是否在任一围栏 span 内（含边界）。
func isInsideFenceSpanRange(index int, spans []FenceSpan) bool {
	for _, span := range spans {
		if index >= span.Start && index < span.End {
			return true
		}
	}
	return false
}

// isInsideInlineSpanRange 判断偏移是否在任一行内代码 span 内（含边界）。
func isInsideInlineSpanRange(index int, spans [][2]int) bool {
	for _, span := range spans {
		if index >= span[0] && index < span[1] {
			return true
		}
	}
	return false
}
