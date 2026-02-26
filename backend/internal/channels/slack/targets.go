package slack

import (
	"fmt"
	"regexp"
	"strings"
)

// Slack 消息目标解析 — 继承自 src/slack/targets.ts (67L)
// 原版使用 ../channels/targets.js 的 buildMessagingTarget / ensureTargetId / requireTargetKind。
// Go 端在 Slack 包内独立实现（与 iMessage/Telegram 模式一致）。

// SlackTargetKind 目标类型
type SlackTargetKind string

const (
	SlackTargetKindUser    SlackTargetKind = "user"
	SlackTargetKindChannel SlackTargetKind = "channel"
)

// SlackTarget 解析后的 Slack 投递目标
type SlackTarget struct {
	Kind     SlackTargetKind
	ID       string
	RawInput string
}

var slackUserMentionRe = regexp.MustCompile(`(?i)^<@([A-Z0-9]+)>$`)
var slackAlphanumericRe = regexp.MustCompile(`(?i)^[A-Z0-9]+$`)

// ParseSlackTarget 解析 Slack 投递目标字符串。
//
// 支持格式：
//   - `<@UXXXX>` → user
//   - `user:UXXXX` → user
//   - `channel:CXXXX` → channel
//   - `slack:UXXXX` → user
//   - `@UXXXX` → user
//   - `#CXXXX` → channel
//   - 其他 → 用 defaultKind 或 channel
func ParseSlackTarget(raw string, defaultKind SlackTargetKind) *SlackTarget {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	// <@UXXXX>
	if m := slackUserMentionRe.FindStringSubmatch(trimmed); m != nil {
		return &SlackTarget{Kind: SlackTargetKindUser, ID: m[1], RawInput: trimmed}
	}

	// user:UXXXX
	if strings.HasPrefix(trimmed, "user:") {
		id := strings.TrimSpace(trimmed[5:])
		if id == "" {
			return nil
		}
		return &SlackTarget{Kind: SlackTargetKindUser, ID: id, RawInput: trimmed}
	}

	// channel:CXXXX
	if strings.HasPrefix(trimmed, "channel:") {
		id := strings.TrimSpace(trimmed[8:])
		if id == "" {
			return nil
		}
		return &SlackTarget{Kind: SlackTargetKindChannel, ID: id, RawInput: trimmed}
	}

	// slack:UXXXX
	if strings.HasPrefix(trimmed, "slack:") {
		id := strings.TrimSpace(trimmed[6:])
		if id == "" {
			return nil
		}
		return &SlackTarget{Kind: SlackTargetKindUser, ID: id, RawInput: trimmed}
	}

	// @UXXXX
	if strings.HasPrefix(trimmed, "@") {
		candidate := strings.TrimSpace(trimmed[1:])
		if !slackAlphanumericRe.MatchString(candidate) {
			// Slack DMs 需要 user id
			return nil
		}
		return &SlackTarget{Kind: SlackTargetKindUser, ID: candidate, RawInput: trimmed}
	}

	// #CXXXX
	if strings.HasPrefix(trimmed, "#") {
		candidate := strings.TrimSpace(trimmed[1:])
		if !slackAlphanumericRe.MatchString(candidate) {
			return nil
		}
		return &SlackTarget{Kind: SlackTargetKindChannel, ID: candidate, RawInput: trimmed}
	}

	// 使用 defaultKind
	if defaultKind != "" {
		return &SlackTarget{Kind: defaultKind, ID: trimmed, RawInput: trimmed}
	}
	return &SlackTarget{Kind: SlackTargetKindChannel, ID: trimmed, RawInput: trimmed}
}

// ResolveSlackChannelID 解析 Slack 频道 ID。
func ResolveSlackChannelID(raw string) (string, error) {
	target := ParseSlackTarget(raw, SlackTargetKindChannel)
	if target == nil || target.Kind != SlackTargetKindChannel {
		return "", fmt.Errorf("slack: expected channel target, got %q", raw)
	}
	return target.ID, nil
}
