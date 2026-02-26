package slack

import "strings"

// Slack Token 解析 — 继承自 src/slack/token.ts (12L)

// NormalizeSlackToken 规范化 Slack token 字符串，去除首尾空白。
// 返回空字符串表示 token 无效。
func NormalizeSlackToken(raw string) string {
	trimmed := strings.TrimSpace(raw)
	return trimmed
}

// ResolveSlackBotToken 解析 Slack Bot Token。
func ResolveSlackBotToken(raw string) string {
	return NormalizeSlackToken(raw)
}

// ResolveSlackAppToken 解析 Slack App Token。
func ResolveSlackAppToken(raw string) string {
	return NormalizeSlackToken(raw)
}
