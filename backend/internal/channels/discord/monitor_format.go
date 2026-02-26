package discord

import (
	"strconv"
	"strings"
	"time"
)

// Discord monitor 格式化工具 — 继承自 src/discord/monitor/format.ts (41L)

// ResolveDiscordSystemLocation 解析系统位置描述
func ResolveDiscordSystemLocation(isDirectMessage, isGroupDm bool, guildName, channelName string) string {
	if isDirectMessage {
		return "DM"
	}
	if isGroupDm {
		return "Group DM #" + channelName
	}
	if guildName != "" {
		return guildName + " #" + channelName
	}
	return "#" + channelName
}

// FormatDiscordReactionEmoji 格式化反应 emoji
func FormatDiscordReactionEmoji(emojiID, emojiName string) string {
	if emojiID != "" && emojiName != "" {
		return emojiName + ":" + emojiID
	}
	if emojiName != "" {
		return emojiName
	}
	return "emoji"
}

// FormatDiscordUserTag 格式化用户标签
func FormatDiscordUserTag(username, discriminator, userID string) string {
	disc := strings.TrimSpace(discriminator)
	if disc != "" && disc != "0" {
		return username + "#" + disc
	}
	if username != "" {
		return username
	}
	return userID
}

// resolveTimestampFormats lists the time formats to try when parsing timestamps.
// W-051 fix: Go only parsed RFC3339/RFC3339Nano; TS Date.parse accepts many more
// formats including ISO 8601 variants. This list covers the common formats that
// Date.parse handles.
var resolveTimestampFormats = []string{
	time.RFC3339,
	time.RFC3339Nano,
	"2006-01-02T15:04:05",          // ISO 8601 without timezone
	"2006-01-02T15:04:05Z0700",     // ISO 8601 compact tz offset
	"2006-01-02T15:04:05-07:00",    // ISO 8601 with tz offset
	"2006-01-02T15:04:05.000Z0700", // ISO 8601 with millis + compact tz
	"2006-01-02 15:04:05",          // common datetime format
	"2006-01-02",                   // date-only (ISO 8601)
	time.RFC1123,                   // HTTP date format
	time.RFC1123Z,                  // HTTP date format with numeric tz
	time.RFC850,                    // RFC 850
}

// ResolveTimestampMs 解析 ISO 时间戳为毫秒。
// W-050 fix: returns *int64 (nil when no timestamp or invalid) instead of int64 (0),
// so callers can distinguish "no timestamp" from Unix epoch (1970-01-01T00:00:00Z).
// This aligns with TS which returns undefined for missing/invalid timestamps.
// W-051 fix: tries multiple time formats beyond RFC3339/RFC3339Nano to match
// the broader parsing capability of JS Date.parse.
func ResolveTimestampMs(timestamp string) *int64 {
	if timestamp == "" {
		return nil
	}

	// Try standard time formats
	for _, layout := range resolveTimestampFormats {
		if t, err := time.Parse(layout, timestamp); err == nil {
			ms := t.UnixMilli()
			return &ms
		}
	}

	// Try numeric string as Unix timestamp (seconds or milliseconds)
	// TS Date.parse doesn't handle raw numeric strings, but this covers
	// Discord snowflake-derived timestamps that may appear as numbers.
	trimmed := strings.TrimSpace(timestamp)
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		if n > 1e15 {
			// Already in microseconds or nanoseconds — treat as milliseconds
			return &n
		}
		if n > 1e12 {
			// Already in milliseconds
			return &n
		}
		// Treat as seconds
		ms := n * 1000
		return &ms
	}

	return nil
}
