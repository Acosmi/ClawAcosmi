// format.go — 链接理解输出格式化。
//
// TS 对照: link-understanding/format.ts (13L)
package linkparse

import "strings"

// FormatLinkUnderstandingBody 将链接理解输出追加到消息体。
//
// 如果 outputs 为空，返回原始 body。
// 如果 body 为空，只返回拼接的 outputs。
// 否则 body + 两个换行 + outputs。
//
// TS 对照: format.ts formatLinkUnderstandingBody()
func FormatLinkUnderstandingBody(body string, outputs []string) string {
	var filtered []string
	for _, o := range outputs {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	if len(filtered) == 0 {
		return body
	}

	base := strings.TrimSpace(body)
	joined := strings.Join(filtered, "\n")
	if base == "" {
		return joined
	}
	return base + "\n\n" + joined
}
