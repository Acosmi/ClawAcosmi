// fences.go — 围栏代码块 span 解析。
//
// TS 对照: markdown/fences.ts (82L)
//
// 识别 Markdown 中的围栏代码块（``` 或 ~~~），返回各代码块的字节偏移范围。
// 供 code_spans.go 和分块逻辑使用，确保分块/格式化不会切断围栏块。
package markdown

import "regexp"

// fenceOpenPattern 匹配围栏开始/结束行: 0-3 个前导空格 + 3 个以上 ` 或 ~。
// TS 对照: fences.ts L28
var fenceOpenPattern = regexp.MustCompile(`^( {0,3})(\x60{3,}|~{3,})(.*)$`)

// FenceSpan 表示一个围栏代码块在源文本中的偏移范围。
// TS 对照: fences.ts FenceSpan
type FenceSpan struct {
	// Start 围栏开始行的字节偏移（含）。
	Start int
	// End 围栏结束行末尾的字节偏移（不含）。
	End int
	// OpenLine 围栏开始行的完整文本。
	OpenLine string
	// Marker 围栏标记符（如 "```" 或 "~~~"）。
	Marker string
	// Indent 围栏标记前的缩进空格。
	Indent string
}

// ParseFenceSpans 解析文本中的所有围栏代码块 span。
//
// 按 CommonMark 规则：
//   - 开始行：0-3 个空格 + 3 个以上相同标记字符（` 或 ~）
//   - 结束行：相同标记字符，长度 >= 开始标记
//   - 未关闭的围栏延伸到文本末尾
//
// TS 对照: fences.ts parseFenceSpans()
func ParseFenceSpans(buffer string) []FenceSpan {
	var spans []FenceSpan

	type openState struct {
		start     int
		markerCh  byte
		markerLen int
		openLine  string
		marker    string
		indent    string
	}
	var open *openState

	offset := 0
	bufLen := len(buffer)

	for offset <= bufLen {
		// 查找下一个换行符
		nextNewline := -1
		for j := offset; j < bufLen; j++ {
			if buffer[j] == '\n' {
				nextNewline = j
				break
			}
		}
		lineEnd := bufLen
		if nextNewline >= 0 {
			lineEnd = nextNewline
		}
		line := buffer[offset:lineEnd]

		// 尝试匹配围栏标记
		m := fenceOpenPattern.FindStringSubmatch(line)
		if m != nil {
			indent := m[1]
			marker := m[2]
			markerCh := marker[0]
			markerLen := len(marker)

			if open == nil {
				// 开始新围栏
				open = &openState{
					start:     offset,
					markerCh:  markerCh,
					markerLen: markerLen,
					openLine:  line,
					marker:    marker,
					indent:    indent,
				}
			} else if open.markerCh == markerCh && markerLen >= open.markerLen {
				// 关闭围栏
				spans = append(spans, FenceSpan{
					Start:    open.start,
					End:      lineEnd,
					OpenLine: open.openLine,
					Marker:   open.marker,
					Indent:   open.indent,
				})
				open = nil
			}
		}

		if nextNewline < 0 {
			break
		}
		offset = nextNewline + 1
	}

	// 未关闭的围栏延伸到文本末尾
	if open != nil {
		spans = append(spans, FenceSpan{
			Start:    open.start,
			End:      bufLen,
			OpenLine: open.openLine,
			Marker:   open.marker,
			Indent:   open.indent,
		})
	}

	return spans
}

// FindFenceSpanAt 查找包含给定字节偏移的围栏 span（不含边界）。
// TS 对照: fences.ts findFenceSpanAt() — 注意 TS 用 > start && < end
func FindFenceSpanAt(spans []FenceSpan, index int) *FenceSpan {
	for i := range spans {
		if index > spans[i].Start && index < spans[i].End {
			return &spans[i]
		}
	}
	return nil
}

// IsSafeFenceBreak 检查在给定偏移处分块是否安全（不会切断围栏块）。
// TS 对照: fences.ts isSafeFenceBreak()
func IsSafeFenceBreak(spans []FenceSpan, index int) bool {
	return FindFenceSpanAt(spans, index) == nil
}
