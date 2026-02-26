// Package autoreply 自动回复引擎核心逻辑。
// TS 对照: auto-reply/ 模块
package autoreply

import (
	"regexp"
	"strings"
)

// TS 对照: auto-reply/tokens.ts

// HeartbeatToken 心跳确认令牌。
const HeartbeatToken = "HEARTBEAT_OK"

// SilentReplyToken 静默回复令牌。
const SilentReplyToken = "NO_REPLY"

// escapeRegExp 转义正则元字符。
// TS 对照: tokens.ts L4-6
func escapeRegExp(value string) string {
	return regexp.QuoteMeta(value)
}

// IsSilentReplyText 判断文本是否为静默回复。
// token 出现在文本首或尾（含前后空白和非单词字符边界）时返回 true。
// TS 对照: tokens.ts L8-22
func IsSilentReplyText(text string, token ...string) bool {
	if text == "" {
		return false
	}
	tok := SilentReplyToken
	if len(token) > 0 && token[0] != "" {
		tok = token[0]
	}
	escaped := escapeRegExp(tok)
	// 前缀匹配：起始空白 + token + (结尾 | 非单词字符)
	prefix := regexp.MustCompile(`^\s*` + escaped + `(?:$|\W)`)
	if prefix.MatchString(text) {
		return true
	}
	// 后缀匹配：单词边界 + token + 可选非单词字符 + 结尾
	suffix := regexp.MustCompile(`\b` + escaped + `\b\W*$`)
	return suffix.MatchString(text)
}

// collapseWhitespace 将多个空白字符折叠为单个空格。
func collapseWhitespace(s string) string {
	re := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(re.ReplaceAllString(s, " "))
}
