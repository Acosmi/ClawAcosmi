package reply

import (
	"regexp"
	"strings"
)

// TS 对照: auto-reply/reply/reply-inline.ts (42L)
// 内联简单命令提取（/help, /commands, /whoami, /id）。

// inlineSimpleCommandAliases 命令别名映射。
var inlineSimpleCommandAliases = map[string]string{
	"/help":     "/help",
	"/commands": "/commands",
	"/whoami":   "/whoami",
	"/id":       "/whoami",
}

// inlineSimpleCommandRe 匹配内联简单命令。
var inlineSimpleCommandRe = regexp.MustCompile(`(?i)(?:^|\s)/(help|commands|whoami|id)(?:$|\s|:)`)

// inlineStatusRe 匹配内联 /status 指令。
var inlineStatusRe = regexp.MustCompile(`(?i)(?:^|\s)/status(?:$|\s|:)(?:\s*:\s*)?`)

// InlineSimpleCommandResult 内联简单命令提取结果。
type InlineSimpleCommandResult struct {
	Command string
	Cleaned string
}

// ExtractInlineSimpleCommand 提取内联简单命令。
// TS 对照: reply-inline.ts extractInlineSimpleCommand
func ExtractInlineSimpleCommand(body string) *InlineSimpleCommandResult {
	if body == "" {
		return nil
	}
	match := inlineSimpleCommandRe.FindStringSubmatch(body)
	if match == nil {
		return nil
	}
	alias := "/" + strings.ToLower(match[1])
	command, ok := inlineSimpleCommandAliases[alias]
	if !ok {
		return nil
	}

	loc := inlineSimpleCommandRe.FindStringIndex(body)
	cleaned := body[:loc[0]] + " " + body[loc[1]:]
	cleaned = collapseWhitespace(cleaned)

	return &InlineSimpleCommandResult{
		Command: command,
		Cleaned: cleaned,
	}
}

// StripInlineStatusResult /status 剥离结果。
type StripInlineStatusResult struct {
	Cleaned  string
	DidStrip bool
}

// StripInlineStatus 剥离内联 /status 指令。
// TS 对照: reply-inline.ts stripInlineStatus
func StripInlineStatus(body string) StripInlineStatusResult {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return StripInlineStatusResult{Cleaned: "", DidStrip: false}
	}
	cleaned := inlineStatusRe.ReplaceAllString(trimmed, " ")
	cleaned = collapseWhitespace(cleaned)
	return StripInlineStatusResult{
		Cleaned:  cleaned,
		DidStrip: cleaned != trimmed,
	}
}
