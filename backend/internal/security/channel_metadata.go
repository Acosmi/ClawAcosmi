// channel_metadata.go — 不可信频道元数据处理。
//
// TS 对照: security/channel-metadata.ts (46L)
//
// 构建安全包装的频道元数据字符串，用于 prompt 注入防护。
package security

import "strings"

const (
	defaultMaxChars      = 800
	defaultMaxEntryChars = 400
)

// normalizeEntry 规范化元数据条目（合并空白）。
func normalizeEntry(entry string) string {
	// 将连续空白替换为单个空格
	fields := strings.Fields(entry)
	return strings.Join(fields, " ")
}

// truncateText 截断文本到 maxChars 长度。
func truncateText(value string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value
	}
	if maxChars <= 3 {
		return string(runes[:maxChars])
	}
	return strings.TrimRight(string(runes[:maxChars-3]), " ") + "..."
}

// BuildUntrustedChannelMetadata 构建不可信频道元数据的安全包装。
//
// 步骤:
//  1. 清理和去重条目
//  2. 截断过长条目
//  3. 用安全边界标记包装
//
// TS 对照: channel-metadata.ts buildUntrustedChannelMetadata()
func BuildUntrustedChannelMetadata(source, label string, entries []string, maxChars *int) string {
	var cleaned []string
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		normalized := normalizeEntry(entry)
		if normalized == "" {
			continue
		}
		cleaned = append(cleaned, truncateText(normalized, defaultMaxEntryChars))
	}

	// 去重
	var deduped []string
	seen := make(map[string]struct{})
	for _, entry := range cleaned {
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		deduped = append(deduped, entry)
	}

	if len(deduped) == 0 {
		return ""
	}

	body := strings.Join(deduped, "\n")
	header := "UNTRUSTED channel metadata (" + source + ")"
	labeled := label + ":\n" + body

	mc := defaultMaxChars
	if maxChars != nil {
		mc = *maxChars
	}
	truncated := truncateText(header+"\n"+labeled, mc)

	return WrapExternalContent(truncated, WrapOptions{
		Source:         SourceChannelMetadata,
		IncludeWarning: false,
	})
}
