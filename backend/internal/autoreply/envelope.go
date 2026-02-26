package autoreply

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// TS 对照: auto-reply/envelope.ts (220L)
// F2: 扩展版 — 增加 timezone modes (utc/local/iana) + elapsed 时间。

// ---------- 类型 ----------

// EnvelopeTimezoneMode 时区模式。
type EnvelopeTimezoneMode string

const (
	EnvelopeTZDefault EnvelopeTimezoneMode = ""      // 使用系统本地
	EnvelopeTZUTC     EnvelopeTimezoneMode = "utc"   // 强制 UTC
	EnvelopeTZLocal   EnvelopeTimezoneMode = "local" // 使用系统本地
	EnvelopeTZIANA    EnvelopeTimezoneMode = "iana"  // 使用 IANA tz，具体 tz 名在 UserTimezone 字段
)

// EnvelopeFormatOptions 信封格式选项。
type EnvelopeFormatOptions struct {
	TimezoneMode     EnvelopeTimezoneMode // 时区模式
	UserTimezone     string               // 当 TimezoneMode == IANA 时使用
	IncludeTimestamp bool                 // 是否包含时间戳
	IncludeElapsed   bool                 // 是否包含经过时间
	IncludeSender    bool                 // 是否包含发送者标签
}

// DefaultEnvelopeFormatOptions 默认选项。
func DefaultEnvelopeFormatOptions() EnvelopeFormatOptions {
	return EnvelopeFormatOptions{
		TimezoneMode:     EnvelopeTZLocal,
		IncludeTimestamp: true,
		IncludeElapsed:   false,
		IncludeSender:    true,
	}
}

// EnvelopeHeader 信封头。
type EnvelopeHeader struct {
	Timestamp      string
	SenderLabel    string
	ConversationID string
	ChannelType    string
	Elapsed        string // e.g. "3m ago"
}

// ---------- 核心函数 ----------

// BuildEnvelopeHeader 构建信封头字符串。
// TS 对照: envelope.ts buildEnvelopeHeader
func BuildEnvelopeHeader(ctx *MsgContext, timezone string) EnvelopeHeader {
	return BuildEnvelopeHeaderWithOptions(ctx, timezone, DefaultEnvelopeFormatOptions(), time.Time{})
}

// BuildEnvelopeHeaderWithOptions 使用选项构建信封头。
// TS 对照: envelope.ts formatAgentEnvelope (完整版)
func BuildEnvelopeHeaderWithOptions(ctx *MsgContext, timezone string, opts EnvelopeFormatOptions, messageTime time.Time) EnvelopeHeader {
	header := EnvelopeHeader{
		ChannelType: ctx.ChannelType,
	}

	// 时间戳
	if opts.IncludeTimestamp {
		loc := resolveEnvelopeTimezone(opts, timezone)
		now := time.Now().In(loc)
		header.Timestamp = now.Format("2006-01-02 15:04")
	}

	// Elapsed 时间差
	if opts.IncludeElapsed && !messageTime.IsZero() {
		header.Elapsed = FormatTimeAgo(time.Since(messageTime))
	}

	// 发送者标签
	if opts.IncludeSender {
		header.SenderLabel = resolveEnvelopeSenderLabel(ctx)
	}

	return header
}

// FormatEnvelopeHeader 格式化信封头为字符串。
// TS 对照: envelope.ts formatEnvelopeHeader
func FormatEnvelopeHeader(header EnvelopeHeader) string {
	var parts []string
	if header.Timestamp != "" {
		ts := header.Timestamp
		if header.Elapsed != "" {
			ts += " (" + header.Elapsed + ")"
		}
		parts = append(parts, ts)
	}
	if header.SenderLabel != "" {
		parts = append(parts, header.SenderLabel)
	}
	if header.ChannelType != "" {
		parts = append(parts, "["+header.ChannelType+"]")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " | ")
}

// FormatInboundEnvelope 构建入站消息信封。
// TS 对照: envelope.ts formatInboundEnvelope
func FormatInboundEnvelope(ctx *MsgContext, timezone string, opts EnvelopeFormatOptions, messageTime time.Time) string {
	header := BuildEnvelopeHeaderWithOptions(ctx, timezone, opts, messageTime)
	return FormatEnvelopeHeader(header)
}

// FormatInboundFromLabel 构建入站消息发送者标签。
// TS 对照: envelope.ts formatInboundFromLabel (L182-203)
func FormatInboundFromLabel(isGroup bool, groupLabel, groupId, directLabel, directId, groupFallback string) string {
	if isGroup {
		label := strings.TrimSpace(groupLabel)
		if label == "" {
			label = groupFallback
		}
		if label == "" {
			label = "Group"
		}
		id := strings.TrimSpace(groupId)
		if id != "" {
			return fmt.Sprintf("%s id:%s", label, id)
		}
		return label
	}

	dl := strings.TrimSpace(directLabel)
	di := strings.TrimSpace(directId)
	if di == "" || di == dl {
		return dl
	}
	return fmt.Sprintf("%s id:%s", dl, di)
}

// ---------- 辅助函数 ----------

// resolveEnvelopeTimezone 解析信封时区。
func resolveEnvelopeTimezone(opts EnvelopeFormatOptions, fallbackTimezone string) *time.Location {
	switch opts.TimezoneMode {
	case EnvelopeTZUTC:
		return time.UTC
	case EnvelopeTZIANA:
		if opts.UserTimezone != "" {
			if loc, err := time.LoadLocation(opts.UserTimezone); err == nil {
				return loc
			}
		}
		return time.Local
	case EnvelopeTZLocal, EnvelopeTZDefault:
		if fallbackTimezone != "" {
			if loc, err := time.LoadLocation(fallbackTimezone); err == nil {
				return loc
			}
		}
		return time.Local
	default:
		return time.Local
	}
}

// resolveEnvelopeSenderLabel 解析发送者显示标签。
func resolveEnvelopeSenderLabel(ctx *MsgContext) string {
	label := ctx.SenderDisplayName
	if label == "" {
		label = ctx.SenderName
	}
	if label == "" {
		label = ctx.SenderID
	}
	return label
}

// FormatTimeAgo 将 Duration 格式化为 "Xm ago" 风格的相对时间。
// TS 对照: infra/format-time/format-relative.ts formatTimeAgo
func FormatTimeAgo(d time.Duration) string {
	secs := int(math.Abs(d.Seconds()))
	if secs < 60 {
		return "just now"
	}
	mins := secs / 60
	if mins < 60 {
		return fmt.Sprintf("%dm ago", mins)
	}
	hours := mins / 60
	if hours < 24 {
		return fmt.Sprintf("%dh ago", hours)
	}
	days := hours / 24
	return fmt.Sprintf("%dd ago", days)
}
