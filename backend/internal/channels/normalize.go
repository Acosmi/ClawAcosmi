package channels

import (
	"regexp"
	"strings"

	"github.com/anthropic/open-acosmi/internal/channels/whatsapp"
)

// 消息目标规范化 — 继承自 src/channels/plugins/normalize/ (6 个频道文件)
// 每个频道提供 NormalizeXxxMessagingTarget + LooksLikeXxxTargetID

// ── 白名单去重工具 (src/channels/allowlists/resolve-utils.ts) ──

// MergeAllowlist 合并允许列表并去重（大小写不敏感去重）
// 对齐 TS: src/channels/allowlists/resolve-utils.ts mergeAllowlist()
func MergeAllowlist(existing []string, additions []string) []string {
	seen := make(map[string]bool)
	var merged []string
	push := func(value string) {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			return
		}
		key := strings.ToLower(normalized)
		if seen[key] {
			return
		}
		seen[key] = true
		merged = append(merged, normalized)
	}
	for _, entry := range existing {
		push(entry)
	}
	for _, entry := range additions {
		push(entry)
	}
	return merged
}

// DeduplicateAllowlist 对已有列表进行大小写不敏感去重
func DeduplicateAllowlist(list []string) []string {
	return MergeAllowlist(list, nil)
}

// ── Telegram ──

// NormalizeTelegramMessagingTarget 规范化 Telegram 消息目标
func NormalizeTelegramMessagingTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	normalized := trimmed
	if strings.HasPrefix(strings.ToLower(normalized), "telegram:") {
		normalized = strings.TrimSpace(normalized[len("telegram:"):])
	} else if strings.HasPrefix(strings.ToLower(normalized), "tg:") {
		normalized = strings.TrimSpace(normalized[len("tg:"):])
	}
	if normalized == "" {
		return ""
	}
	// t.me 链接解析
	tmeRe := regexp.MustCompile(`(?i)^(?:https?://)?t\.me/([A-Za-z0-9_]+)$`)
	if m := tmeRe.FindStringSubmatch(normalized); len(m) > 1 {
		normalized = "@" + m[1]
	}
	if normalized == "" {
		return ""
	}
	return strings.ToLower("telegram:" + normalized)
}

var tgTargetRe = regexp.MustCompile(`^(telegram|tg):`)
var tgNumericRe = regexp.MustCompile(`^-?\d{6,}$`)

// LooksLikeTelegramTargetID 判断是否看起来像 Telegram 目标
func LooksLikeTelegramTargetID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if tgTargetRe.MatchString(strings.ToLower(trimmed)) {
		return true
	}
	if strings.HasPrefix(trimmed, "@") {
		return true
	}
	return tgNumericRe.MatchString(trimmed)
}

// ── Discord ──

var discordMentionRe = regexp.MustCompile(`^<@!?\d+>$`)
var discordMentionExtractRe = regexp.MustCompile(`^<@!?(\d+)>$`)
var discordPrefixRe = regexp.MustCompile(`(?i)^(user|channel|discord):`)
var discordNumericRe = regexp.MustCompile(`^\d{6,}$`)

// NormalizeDiscordMessagingTarget 规范化 Discord 消息目标
// - `<@123>` / `<@!123>` mention → 提取用户 ID，输出 user:123
// - `user:123` → user:123
// - `channel:123` → channel:123
// - `discord:123` → user:123
// - 裸数字 ID（≥6 位）→ 默认 channel:123（对齐 TS defaultKind:"channel"）
func NormalizeDiscordMessagingTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	// <@123> 或 <@!123> mention → user
	if m := discordMentionExtractRe.FindStringSubmatch(trimmed); len(m) > 1 {
		return "user:" + m[1]
	}

	lower := strings.ToLower(trimmed)

	// user:ID
	if strings.HasPrefix(lower, "user:") {
		id := strings.TrimSpace(trimmed[5:])
		if id == "" {
			return ""
		}
		return "user:" + id
	}

	// channel:ID
	if strings.HasPrefix(lower, "channel:") {
		id := strings.TrimSpace(trimmed[8:])
		if id == "" {
			return ""
		}
		return "channel:" + id
	}

	// discord:ID → user（对齐 TS discord: 前缀映射为 user）
	if strings.HasPrefix(lower, "discord:") {
		id := strings.TrimSpace(trimmed[8:])
		if id == "" {
			return ""
		}
		return "user:" + id
	}

	// 裸数字 ID（≥6 位）→ 默认 channel（对齐 TS defaultKind:"channel"）
	if discordNumericRe.MatchString(trimmed) {
		return "channel:" + trimmed
	}

	return "channel:" + trimmed
}

// LooksLikeDiscordTargetID 判断是否看起来像 Discord 目标
func LooksLikeDiscordTargetID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if discordMentionRe.MatchString(trimmed) {
		return true
	}
	if discordPrefixRe.MatchString(trimmed) {
		return true
	}
	return discordNumericRe.MatchString(trimmed)
}

// ── Slack ──

var slackMentionRe = regexp.MustCompile(`(?i)^<@([A-Z0-9]+)>$`)
var slackPrefixRe = regexp.MustCompile(`(?i)^(user|channel|slack):`)
var slackChannelIDRe = regexp.MustCompile(`(?i)^[CUWGD][A-Z0-9]{8,}$`)

// NormalizeSlackMessagingTarget 规范化 Slack 消息目标
func NormalizeSlackMessagingTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	s := trimmed
	for _, prefix := range []string{"slack:", "channel:", "user:"} {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			s = strings.TrimSpace(s[len(prefix):])
			break
		}
	}
	if s == "" {
		return ""
	}
	return "channel:" + s
}

// LooksLikeSlackTargetID 判断是否看起来像 Slack 目标
func LooksLikeSlackTargetID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if slackMentionRe.MatchString(trimmed) {
		return true
	}
	if slackPrefixRe.MatchString(trimmed) {
		return true
	}
	if strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "#") {
		return true
	}
	return slackChannelIDRe.MatchString(trimmed)
}

// ── WhatsApp ──

var waPhoneRe = regexp.MustCompile(`^\+?\d{3,}$`)

// NormalizeWhatsAppMessagingTarget 规范化 WhatsApp 消息目标
// 委托给 whatsapp.NormalizeWhatsAppTarget 进行完整 JID/LID/Group 解析
func NormalizeWhatsAppMessagingTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	return whatsapp.NormalizeWhatsAppTarget(trimmed)
}

// LooksLikeWhatsAppTargetID 判断是否看起来像 WhatsApp 目标
func LooksLikeWhatsAppTargetID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "whatsapp:") {
		return true
	}
	if strings.Contains(trimmed, "@") {
		return true
	}
	return waPhoneRe.MatchString(trimmed)
}

// ── Signal ──

var uuidRe = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var uuidCompactRe = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
var signalPhoneRe = regexp.MustCompile(`^\+?\d{3,}$`)

// NormalizeSignalMessagingTarget 规范化 Signal 消息目标
func NormalizeSignalMessagingTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	normalized := trimmed
	if strings.HasPrefix(strings.ToLower(normalized), "signal:") {
		normalized = strings.TrimSpace(normalized[len("signal:"):])
	}
	if normalized == "" {
		return ""
	}
	lower := strings.ToLower(normalized)
	switch {
	case strings.HasPrefix(lower, "group:"):
		id := strings.TrimSpace(normalized[len("group:"):])
		if id == "" {
			return ""
		}
		return "group:" + strings.ToLower(id)
	case strings.HasPrefix(lower, "username:"):
		id := strings.TrimSpace(normalized[len("username:"):])
		if id == "" {
			return ""
		}
		return "username:" + strings.ToLower(id)
	case strings.HasPrefix(lower, "u:"):
		id := strings.TrimSpace(normalized[len("u:"):])
		if id == "" {
			return ""
		}
		return "username:" + strings.ToLower(id)
	case strings.HasPrefix(lower, "uuid:"):
		id := strings.TrimSpace(normalized[len("uuid:"):])
		if id == "" {
			return ""
		}
		return strings.ToLower(id)
	}
	return strings.ToLower(normalized)
}

// LooksLikeSignalTargetID 判断是否看起来像 Signal 目标
func LooksLikeSignalTargetID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if regexp.MustCompile(`(?i)^(signal:)?(group:|username:|u:)`).MatchString(trimmed) {
		return true
	}
	if regexp.MustCompile(`(?i)^(signal:)?uuid:`).MatchString(trimmed) {
		stripped := regexp.MustCompile(`(?i)^signal:`).ReplaceAllString(trimmed, "")
		stripped = regexp.MustCompile(`(?i)^uuid:`).ReplaceAllString(stripped, "")
		stripped = strings.TrimSpace(stripped)
		if stripped == "" {
			return false
		}
		return uuidRe.MatchString(stripped) || uuidCompactRe.MatchString(stripped)
	}
	if uuidRe.MatchString(trimmed) || uuidCompactRe.MatchString(trimmed) {
		return true
	}
	return signalPhoneRe.MatchString(trimmed)
}

// ── iMessage ──

var imsgServicePrefixes = []string{"imessage:", "sms:", "auto:"}
var imsgChatTargetRe = regexp.MustCompile(`(?i)^(chat_id:|chatid:|chat:|chat_guid:|chatguid:|guid:|chat_identifier:|chatidentifier:|chatident:)`)
var imsgPhoneRe = regexp.MustCompile(`^\+?\d{3,}$`)
var imsgE164Re = regexp.MustCompile(`^\+[1-9]\d{6,14}$`)

// normalizeIMessageHandle 规范化单个 iMessage handle：
// - email 地址执行 lowercase
// - 电话号码验证 E.164 格式（+[1-9]\d{6,14}）
func normalizeIMessageHandle(handle string) string {
	if handle == "" {
		return ""
	}
	// email 地址：含 @ 则 lowercase
	if strings.Contains(handle, "@") {
		return strings.ToLower(handle)
	}
	// 电话号码：要求 E.164 格式
	if imsgE164Re.MatchString(handle) {
		return handle
	}
	// 非 E.164 但有数字前缀的电话号码也保留（兼容原有行为）
	if imsgPhoneRe.MatchString(handle) {
		return handle
	}
	return handle
}

// NormalizeIMessageMessagingTarget 规范化 iMessage 消息目标
// - email 地址执行 lowercase（对齐 TS normalizeIMessageHandle）
// - 电话号码保留 E.164 格式
func NormalizeIMessageMessagingTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range imsgServicePrefixes {
		if strings.HasPrefix(lower, prefix) {
			remainder := strings.TrimSpace(trimmed[len(prefix):])
			if remainder == "" {
				return ""
			}
			// 如果是 chat 目标前缀直接返回
			if imsgChatTargetRe.MatchString(remainder) {
				return remainder
			}
			normalized := normalizeIMessageHandle(remainder)
			if normalized == "" {
				return ""
			}
			return prefix + normalized
		}
	}
	return normalizeIMessageHandle(trimmed)
}

// LooksLikeIMessageTargetID 判断是否看起来像 iMessage 目标
func LooksLikeIMessageTargetID(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	if regexp.MustCompile(`(?i)^(imessage:|sms:|auto:)`).MatchString(trimmed) {
		return true
	}
	if imsgChatTargetRe.MatchString(trimmed) {
		return true
	}
	if strings.Contains(trimmed, "@") {
		return true
	}
	return imsgPhoneRe.MatchString(trimmed)
}
