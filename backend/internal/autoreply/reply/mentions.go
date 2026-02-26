package reply

import (
	"regexp"
	"strings"
)

// TS 对照: auto-reply/reply/mentions.ts (158L)

// CurrentMessageMarker 当前消息标记（用于历史上下文分隔）。
const CurrentMessageMarker = "[Current message - respond to this]"

// StripStructuralPrefixes 去除结构性前缀（时间戳、发送者标签等）。
// 使得 directive-only 检测在包含历史/上下文的群组批次中仍能正常工作。
// TS 对照: mentions.ts L110-123
func StripStructuralPrefixes(text string) string {
	afterMarker := text
	if idx := strings.Index(text, CurrentMessageMarker); idx >= 0 {
		afterMarker = strings.TrimLeft(text[idx+len(CurrentMessageMarker):], " \t\n\r")
	}

	// 去除 [xxx] 包装标签
	bracketRe := regexp.MustCompile(`\[[^\]]+\]\s*`)
	afterMarker = bracketRe.ReplaceAllString(afterMarker, "")

	// 去除行首 "SenderName:" 前缀
	senderRe := regexp.MustCompile(`(?m)^[ \t]*[A-Za-z0-9+()\-_. ]+:\s*`)
	afterMarker = senderRe.ReplaceAllString(afterMarker, "")

	// 标准化空白
	afterMarker = strings.ReplaceAll(afterMarker, `\n`, " ")
	spaceRe := regexp.MustCompile(`\s+`)
	afterMarker = spaceRe.ReplaceAllString(afterMarker, " ")

	return strings.TrimSpace(afterMarker)
}

// NormalizeMentionText 标准化提及文本（去除不可见字符并转小写）。
// TS 对照: mentions.ts L71-73
func NormalizeMentionText(text string) string {
	// 去除零宽字符
	zeroWidthRe := regexp.MustCompile(`[\x{200b}-\x{200f}\x{202a}-\x{202e}\x{2060}-\x{206f}]`)
	cleaned := zeroWidthRe.ReplaceAllString(text, "")
	return strings.ToLower(cleaned)
}

// MatchesMentionPatterns 检查文本是否匹配任何提及模式。
// TS 对照: mentions.ts L75-84
func MatchesMentionPatterns(text string, mentionRegexes []*regexp.Regexp) bool {
	if len(mentionRegexes) == 0 {
		return false
	}
	cleaned := NormalizeMentionText(text)
	if cleaned == "" {
		return false
	}
	for _, re := range mentionRegexes {
		if re.MatchString(cleaned) {
			return true
		}
	}
	return false
}

// ExplicitMentionSignal 显式提及信号。
type ExplicitMentionSignal struct {
	HasAnyMention         bool
	IsExplicitlyMentioned bool
	CanResolveExplicit    bool
}

// MatchesMentionWithExplicit 带显式信号的提及匹配。
// TS 对照: mentions.ts L92-108
func MatchesMentionWithExplicit(text string, mentionRegexes []*regexp.Regexp, explicit *ExplicitMentionSignal) bool {
	cleaned := NormalizeMentionText(text)
	isExplicit := explicit != nil && explicit.IsExplicitlyMentioned
	explicitAvailable := explicit != nil && explicit.CanResolveExplicit
	hasAny := explicit != nil && explicit.HasAnyMention

	if hasAny && explicitAvailable {
		if isExplicit {
			return true
		}
		for _, re := range mentionRegexes {
			if re.MatchString(cleaned) {
				return true
			}
		}
		return false
	}
	if cleaned == "" {
		return isExplicit
	}
	if isExplicit {
		return true
	}
	for _, re := range mentionRegexes {
		if re.MatchString(cleaned) {
			return true
		}
	}
	return false
}

// BuildMentionRegexes 从配置构建提及正则列表。
// TS 对照: mentions.ts L58-69
func BuildMentionRegexes(patterns []string) []*regexp.Regexp {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			continue
		}
		result = append(result, re)
	}
	return result
}

// StripMentions 从文本中移除提及。
// TS 对照: mentions.ts L125-157
func StripMentions(text string, patterns []string) string {
	result := text
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			continue
		}
		result = re.ReplaceAllString(result, " ")
	}
	// 通用数字提及模式 @123456789
	numericRe := regexp.MustCompile(`@[0-9+]{5,}`)
	result = numericRe.ReplaceAllString(result, " ")
	// 合并空白
	spaceRe := regexp.MustCompile(`\s+`)
	return strings.TrimSpace(spaceRe.ReplaceAllString(result, " "))
}
