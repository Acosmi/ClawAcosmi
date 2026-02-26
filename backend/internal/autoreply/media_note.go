package autoreply

import (
	"fmt"
	"strings"
)

// TS 对照: auto-reply/media-note.ts (94L)

// FormatMediaAttachedLine 格式化媒体附件行。
// TS 对照: media-note.ts L5-25
func FormatMediaAttachedLine(mediaType string, count int) string {
	if count <= 0 {
		return ""
	}
	label := mediaType
	if label == "" {
		label = "file"
	}
	if count == 1 {
		return fmt.Sprintf("[%s attached]", label)
	}
	return fmt.Sprintf("[%d %ss attached]", count, label)
}

// BuildInboundMediaNote 构建入站媒体注解。
// TS 对照: media-note.ts L27-94
func BuildInboundMediaNote(ctx *MsgContext) string {
	if ctx == nil {
		return ""
	}
	if !ctx.HasAttachments && ctx.MediaCount <= 0 {
		return ""
	}

	var lines []string

	mediaCount := ctx.MediaCount
	if mediaCount <= 0 {
		mediaCount = 1
	}

	line := FormatMediaAttachedLine(ctx.MediaType, mediaCount)
	if line != "" {
		lines = append(lines, line)
	}

	if ctx.SuppressedAttachments > 0 {
		lines = append(lines, fmt.Sprintf("[%d attachment(s) suppressed]", ctx.SuppressedAttachments))
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}
