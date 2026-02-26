// bash/pty_dsr.go — PTY DSR（Device Status Report）处理。
// TS 参考：src/agents/pty-dsr.ts (16L)
//
// 清理终端输出中的 DSR 请求，并构建光标位置响应。
package bash

import (
	"fmt"
	"regexp"
)

// dsrPattern 匹配终端的 DSR 请求序列：ESC[6n 或 ESC[?6n。
var dsrPattern = regexp.MustCompile("\x1b\\[\\??6n")

// DsrStripResult 清理 DSR 请求的结果。
type DsrStripResult struct {
	Cleaned  string
	Requests int
}

// StripDsrRequests 从输入中清除所有 DSR 请求序列。
// TS 参考: pty-dsr.ts stripDsrRequests
func StripDsrRequests(input string) DsrStripResult {
	requests := 0
	cleaned := dsrPattern.ReplaceAllStringFunc(input, func(_ string) string {
		requests++
		return ""
	})
	return DsrStripResult{Cleaned: cleaned, Requests: requests}
}

// BuildCursorPositionResponse 构建光标位置响应序列 ESC[row;colR。
// TS 参考: pty-dsr.ts buildCursorPositionResponse
func BuildCursorPositionResponse(row, col int) string {
	if row <= 0 {
		row = 1
	}
	if col <= 0 {
		col = 1
	}
	return fmt.Sprintf("\x1b[%d;%dR", row, col)
}
