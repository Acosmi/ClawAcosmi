package media

import (
	"regexp"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/markdown"
)

// TS 对照: media/parse.ts (221L)

// mediaTokenRe 匹配 MEDIA: token。
// TS 对照: parse.ts L7
var mediaTokenRe = regexp.MustCompile(`(?i)\bMEDIA:\s*` + "`?" + `([^\n]+)` + "`?")

// NormalizeMediaSource 规范化媒体源路径。
// TS 对照: parse.ts L9-11
func NormalizeMediaSource(src string) string {
	if strings.HasPrefix(src, "file://") {
		return strings.TrimPrefix(src, "file://")
	}
	return src
}

// IsValidMedia 判断媒体候选路径是否有效。
// TS 对照: parse.ts L17-33
func IsValidMedia(candidate string, allowSpaces bool) bool {
	if candidate == "" {
		return false
	}
	if len(candidate) > 4096 {
		return false
	}
	if !allowSpaces && strings.ContainsAny(candidate, " \t\n\r") {
		return false
	}
	// HTTP(S) URL
	if strings.HasPrefix(candidate, "http://") || strings.HasPrefix(candidate, "https://") {
		return true
	}
	// 本地安全路径: 仅允许 ./ 开头且不含 ..
	return strings.HasPrefix(candidate, "./") && !strings.Contains(candidate, "..")
}

// cleanCandidate 清理候选路径的首尾引号和括号。
// TS 对照: parse.ts L13-15
func cleanCandidate(raw string) string {
	raw = strings.TrimLeft(raw, "`\"'[{(")
	raw = strings.TrimRight(raw, "`\"'\\})].,")
	return raw
}

// SplitMediaResult MEDIA token 解析结果。
type SplitMediaResult struct {
	Text         string
	MediaURLs    []string
	MediaURL     string // 兼容：第一个媒体 URL
	AudioAsVoice bool
}

// isInsideFence 判断给定字节偏移是否在任一围栏代码块 span 内。
// TS 对照: parse.ts L52-54
func isInsideFence(spans []markdown.FenceSpan, offset int) bool {
	for _, span := range spans {
		if offset >= span.Start && offset < span.End {
			return true
		}
	}
	return false
}

// SplitMediaFromOutput 从输出文本中提取 MEDIA: token 和 [[audio_as_voice]] 标签。
// TS 对照: parse.ts L56-220
func SplitMediaFromOutput(raw string) SplitMediaResult {
	trimmedRaw := strings.TrimRight(raw, " \t\n\r")
	if strings.TrimSpace(trimmedRaw) == "" {
		return SplitMediaResult{Text: ""}
	}

	var media []string
	foundMediaToken := false

	// 解析围栏代码块 span，避免提取代码块内的 MEDIA: token
	// TS 对照: parse.ts L73
	fenceSpans := markdown.ParseFenceSpans(trimmedRaw)

	lines := strings.Split(trimmedRaw, "\n")
	var keptLines []string

	lineOffset := 0 // 追踪当前行在原始文本中的字节偏移
	for _, line := range lines {
		// 如果当前行在围栏代码块内，跳过 MEDIA 提取
		// TS 对照: parse.ts L82-86
		if isInsideFence(fenceSpans, lineOffset) {
			keptLines = append(keptLines, line)
			lineOffset += len(line) + 1 // +1 for newline
			continue
		}

		trimmedStart := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmedStart, "MEDIA:") {
			keptLines = append(keptLines, line)
			lineOffset += len(line) + 1
			continue
		}

		matches := mediaTokenRe.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			keptLines = append(keptLines, line)
			lineOffset += len(line) + 1
			continue
		}

		var pieces []string
		cursor := 0
		for _, match := range matches {
			start := match[0]
			end := match[1]
			payloadStart := match[2]
			payloadEnd := match[3]

			pieces = append(pieces, line[cursor:start])
			payload := strings.TrimSpace(line[payloadStart:payloadEnd])

			parts := strings.Fields(payload)
			hasValid := false
			var invalidParts []string

			for _, part := range parts {
				candidate := NormalizeMediaSource(cleanCandidate(part))
				if IsValidMedia(candidate, false) {
					media = append(media, candidate)
					hasValid = true
					foundMediaToken = true
				} else {
					invalidParts = append(invalidParts, part)
				}
			}

			// 整个 payload 回退
			if !hasValid {
				fallback := NormalizeMediaSource(cleanCandidate(payload))
				if IsValidMedia(fallback, true) {
					media = append(media, fallback)
					hasValid = true
					foundMediaToken = true
					invalidParts = nil
				}
			}

			if hasValid {
				if len(invalidParts) > 0 {
					pieces = append(pieces, strings.Join(invalidParts, " "))
				}
			} else {
				pieces = append(pieces, line[start:end])
			}

			cursor = end
		}
		pieces = append(pieces, line[cursor:])

		cleanedLine := strings.TrimSpace(strings.Join(pieces, ""))
		if cleanedLine != "" {
			keptLines = append(keptLines, cleanedLine)
		}
		lineOffset += len(line) + 1
	}

	cleanedText := strings.Join(keptLines, "\n")
	// 规范化空白
	cleanedText = collapseWhitespace(cleanedText)

	// 检测 [[audio_as_voice]] 标签
	audioResult := ParseAudioTag(cleanedText)
	hasAudioAsVoice := audioResult.AudioAsVoice
	if audioResult.HadTag {
		cleanedText = collapseWhitespace(audioResult.Text)
	}

	if len(media) == 0 {
		text := trimmedRaw
		if foundMediaToken || hasAudioAsVoice {
			text = cleanedText
		}
		return SplitMediaResult{
			Text:         text,
			AudioAsVoice: hasAudioAsVoice,
		}
	}

	return SplitMediaResult{
		Text:         cleanedText,
		MediaURLs:    media,
		MediaURL:     media[0],
		AudioAsVoice: hasAudioAsVoice,
	}
}

// collapseWhitespace 合并多余空白和空行。
func collapseWhitespace(s string) string {
	// 行尾空白
	trailingRe := regexp.MustCompile(`[ \t]+\n`)
	s = trailingRe.ReplaceAllString(s, "\n")
	// 连续空白
	spaceRe := regexp.MustCompile(`[ \t]{2,}`)
	s = spaceRe.ReplaceAllString(s, " ")
	// 连续空行
	nlRe := regexp.MustCompile(`\n{2,}`)
	s = nlRe.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}
