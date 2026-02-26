package media

import (
	"regexp"
	"strings"
)

// TS 对照: media/audio-tags.ts (20L) + utils/directive-tags.ts (83L)
// parseInlineDirectives 未在 Go 端实现，此处内联音频标签解析逻辑。

// audioTagRe 匹配 [[audio_as_voice]] 标签（大小写不敏感）。
// TS 对照: directive-tags.ts L17
var audioTagRe = regexp.MustCompile(`(?i)\[\[\s*audio_as_voice\s*\]\]`)

// AudioTagResult 音频标签解析结果。
type AudioTagResult struct {
	Text         string
	AudioAsVoice bool
	HadTag       bool
}

// ParseAudioTag 从文本中提取 [[audio_as_voice]] 标签。
// 支持发送音频作为语音气泡而非文件附件。
// TS 对照: audio-tags.ts L8-19
func ParseAudioTag(text string) AudioTagResult {
	if text == "" {
		return AudioTagResult{}
	}

	hadTag := audioTagRe.MatchString(text)
	if !hadTag {
		return AudioTagResult{
			Text: text,
		}
	}

	cleaned := audioTagRe.ReplaceAllString(text, " ")
	// 规范化空白
	cleaned = normalizeDirectiveWhitespace(cleaned)

	return AudioTagResult{
		Text:         cleaned,
		AudioAsVoice: true,
		HadTag:       true,
	}
}

// normalizeDirectiveWhitespace 规范化指令标签移除后的空白。
// TS 对照: directive-tags.ts L20-25
func normalizeDirectiveWhitespace(text string) string {
	// 合并连续空格/制表符
	spaceRe := regexp.MustCompile(`[ \t]+`)
	text = spaceRe.ReplaceAllString(text, " ")
	// 规范化换行周围的空白
	nlRe := regexp.MustCompile(`[ \t]*\n[ \t]*`)
	text = nlRe.ReplaceAllString(text, "\n")
	return strings.TrimSpace(text)
}
