package autoreply

import (
	"regexp"
	"strings"
)

// TS 对照: auto-reply/heartbeat.ts

// HeartbeatPrompt 默认心跳提示。
// TS 对照: heartbeat.ts L5-6
const HeartbeatPrompt = "Read HEARTBEAT.md if it exists (workspace context). Follow it strictly. Do not infer or repeat old tasks from prior chats. If nothing needs attention, reply HEARTBEAT_OK."

// DefaultHeartbeatEvery 默认心跳间隔。
const DefaultHeartbeatEvery = "30m"

// DefaultHeartbeatAckMaxChars 默认心跳确认最大字符数。
const DefaultHeartbeatAckMaxChars = 300

// 预编译心跳内容检测正则
var (
	headerRe    = regexp.MustCompile(`^#+(\s|$)`)
	emptyListRe = regexp.MustCompile(`^[-*+]\s*(\[[\sXx]?\]\s*)?$`)
)

// IsHeartbeatContentEffectivelyEmpty 判断 HEARTBEAT.md 内容是否「实质空」。
// 仅包含空行、markdown 标题行和空列表项时返回 true。
// TS 对照: heartbeat.ts L22-52
func IsHeartbeatContentEffectivelyEmpty(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if headerRe.MatchString(trimmed) {
			continue
		}
		if emptyListRe.MatchString(trimmed) {
			continue
		}
		return false
	}
	return true
}

// ResolveHeartbeatPrompt 解析心跳提示（优先使用用户配置）。
// TS 对照: heartbeat.ts L54-57
func ResolveHeartbeatPrompt(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return HeartbeatPrompt
	}
	return trimmed
}

// StripHeartbeatMode 心跳剥离模式。
type StripHeartbeatMode string

const (
	StripModeHeartbeat StripHeartbeatMode = "heartbeat"
	StripModeMessage   StripHeartbeatMode = "message"
)

// StripHeartbeatOpts 心跳剥离选项。
type StripHeartbeatOpts struct {
	Mode        StripHeartbeatMode
	MaxAckChars int // 0 表示使用默认值
}

// StripHeartbeatResult 心跳剥离结果。
type StripHeartbeatResult struct {
	ShouldSkip bool
	Text       string
	DidStrip   bool
}

// stripTokenResult 内部令牌剥离结果。
type stripTokenResult struct {
	text     string
	didStrip bool
}

// 预编译 HTML/Markdown 清理正则
var (
	htmlTagRe       = regexp.MustCompile(`<[^>]*>`)
	nbspRe          = regexp.MustCompile(`(?i)&nbsp;`)
	markdownLeadRe  = regexp.MustCompile(`^[*` + "`" + `~_]+`)
	markdownTrailRe = regexp.MustCompile(`[*` + "`" + `~_]+$`)
)

// stripMarkup 清理 HTML/Markdown 标记。
// TS 对照: heartbeat.ts L121-129
func stripMarkup(text string) string {
	text = htmlTagRe.ReplaceAllString(text, " ")
	text = nbspRe.ReplaceAllString(text, " ")
	text = markdownLeadRe.ReplaceAllString(text, "")
	text = markdownTrailRe.ReplaceAllString(text, "")
	return text
}

// stripTokenAtEdges 从文本首尾剥离心跳令牌。
// TS 对照: heartbeat.ts L61-94
func stripTokenAtEdges(raw string) stripTokenResult {
	text := strings.TrimSpace(raw)
	if text == "" {
		return stripTokenResult{text: "", didStrip: false}
	}

	token := HeartbeatToken
	if !strings.Contains(text, token) {
		return stripTokenResult{text: text, didStrip: false}
	}

	didStrip := false
	changed := true
	for changed {
		changed = false
		next := strings.TrimSpace(text)
		if strings.HasPrefix(next, token) {
			after := strings.TrimLeft(next[len(token):], " \t\n\r")
			text = after
			didStrip = true
			changed = true
			continue
		}
		if strings.HasSuffix(next, token) {
			before := next[:len(next)-len(token)]
			text = strings.TrimRight(before, " \t\n\r")
			didStrip = true
			changed = true
		}
	}

	collapsed := collapseWhitespace(text)
	return stripTokenResult{text: collapsed, didStrip: didStrip}
}

// StripHeartbeatToken 剥离心跳令牌，判断是否应跳过消息发送。
// TS 对照: heartbeat.ts L96-157
func StripHeartbeatToken(raw string, opts *StripHeartbeatOpts) StripHeartbeatResult {
	if raw == "" {
		return StripHeartbeatResult{ShouldSkip: true, Text: "", DidStrip: false}
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return StripHeartbeatResult{ShouldSkip: true, Text: "", DidStrip: false}
	}

	mode := StripModeMessage
	maxAckChars := DefaultHeartbeatAckMaxChars
	if opts != nil {
		if opts.Mode != "" {
			mode = opts.Mode
		}
		if opts.MaxAckChars > 0 {
			maxAckChars = opts.MaxAckChars
		}
	}

	trimmedNormalized := stripMarkup(trimmed)
	hasToken := strings.Contains(trimmed, HeartbeatToken) || strings.Contains(trimmedNormalized, HeartbeatToken)
	if !hasToken {
		return StripHeartbeatResult{ShouldSkip: false, Text: trimmed, DidStrip: false}
	}

	strippedOriginal := stripTokenAtEdges(trimmed)
	strippedNormalized := stripTokenAtEdges(trimmedNormalized)

	picked := strippedNormalized
	if strippedOriginal.didStrip && strippedOriginal.text != "" {
		picked = strippedOriginal
	}
	if !picked.didStrip {
		return StripHeartbeatResult{ShouldSkip: false, Text: trimmed, DidStrip: false}
	}

	if picked.text == "" {
		return StripHeartbeatResult{ShouldSkip: true, Text: "", DidStrip: true}
	}

	rest := strings.TrimSpace(picked.text)
	if mode == StripModeHeartbeat {
		if len([]rune(rest)) <= maxAckChars {
			return StripHeartbeatResult{ShouldSkip: true, Text: "", DidStrip: true}
		}
	}

	return StripHeartbeatResult{ShouldSkip: false, Text: rest, DidStrip: true}
}
