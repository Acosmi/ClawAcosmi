// detect.go — 从消息中提取 URL 链接。
//
// TS 对照: link-understanding/detect.ts (64L)
//
// 先剥离 Markdown 链接语法 [text](url)，再提取裸 URL，
// 过滤非 HTTP(S) 协议和本地回环地址，去重后限制数量。
package linkparse

import (
	"net/url"
	"regexp"
	"strings"
)

// markdownLinkRe 匹配 Markdown 链接 [text](url)。
var markdownLinkRe = regexp.MustCompile(`\[[^\]]*]\((https?://\S+?)\)`)

// bareLinkRe 匹配裸 URL。
var bareLinkRe = regexp.MustCompile(`https?://\S+`)

// stripMarkdownLinks 移除 Markdown 链接语法，只保留裸 URL 用于后续匹配。
// TS 对照: detect.ts stripMarkdownLinks()
func stripMarkdownLinks(message string) string {
	return markdownLinkRe.ReplaceAllString(message, " ")
}

// ResolveMaxLinks 解析最大链接数配置。
// TS 对照: detect.ts resolveMaxLinks()
func ResolveMaxLinks(value *int) int {
	if value != nil && *value > 0 {
		return *value
	}
	return DefaultMaxLinks
}

// IsAllowedURL 检查 URL 是否允许（HTTP/HTTPS 且非本地回环）。
// TS 对照: detect.ts isAllowedUrl()
func IsAllowedURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	if parsed.Hostname() == "127.0.0.1" {
		return false
	}
	return true
}

// ExtractLinksFromMessage 从消息文本中提取链接。
//
// 步骤:
//  1. 剥离 Markdown 链接语法
//  2. 匹配所有裸 URL
//  3. 过滤 + 去重
//  4. 限制最大数量
//
// TS 对照: detect.ts extractLinksFromMessage()
func ExtractLinksFromMessage(message string, maxLinks *int) []string {
	source := strings.TrimSpace(message)
	if source == "" {
		return nil
	}

	limit := ResolveMaxLinks(maxLinks)
	sanitized := stripMarkdownLinks(source)

	seen := make(map[string]struct{})
	var results []string

	matches := bareLinkRe.FindAllString(sanitized, -1)
	for _, raw := range matches {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !IsAllowedURL(raw) {
			continue
		}
		if _, dup := seen[raw]; dup {
			continue
		}
		seen[raw] = struct{}{}
		results = append(results, raw)
		if len(results) >= limit {
			break
		}
	}

	return results
}
