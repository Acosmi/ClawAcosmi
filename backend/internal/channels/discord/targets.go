package discord

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/channels"
)

// Discord 消息目标解析 — 继承自 src/discord/targets.ts (163L)
// Go 端在 discord 包内独立实现（与 Slack/Telegram 模式一致）。

// DiscordTargetKind 目标类型
type DiscordTargetKind string

const (
	DiscordTargetKindUser    DiscordTargetKind = "user"
	DiscordTargetKindChannel DiscordTargetKind = "channel"
)

// DiscordTarget 解析后的 Discord 投递目标
type DiscordTarget struct {
	Kind       DiscordTargetKind
	ID         string
	RawInput   string
	Normalized string // W-061: 对齐 TS buildMessagingTarget 的 normalized 字段
}

// newDiscordTarget 构造 DiscordTarget，自动计算 Normalized 字段。
// 对齐 TS: `normalized: \`${kind}:${id}\`.toLowerCase()`
func newDiscordTarget(kind DiscordTargetKind, id, rawInput string) *DiscordTarget {
	return &DiscordTarget{
		Kind:       kind,
		ID:         id,
		RawInput:   rawInput,
		Normalized: strings.ToLower(string(kind) + ":" + id),
	}
}

// DiscordTargetParseOptions 解析选项
type DiscordTargetParseOptions struct {
	DefaultKind      DiscordTargetKind
	AmbiguousMessage string
}

var discordUserMentionRe = regexp.MustCompile(`^<@!?(\d+)>$`)

// W-070: 对齐 TS `[\d]+$` — 只要求以数字结尾，去掉行首锚点 ^
var discordNumericRe = regexp.MustCompile(`\d+$`)
var discordKnownFormatRe = regexp.MustCompile(`^(user:|channel:|discord:|@|<@!?)`)

// ParseDiscordTarget 解析 Discord 投递目标字符串。
//
// 支持格式：
//   - `<@12345>` 或 `<@!12345>` → user
//   - `user:12345` → user
//   - `channel:12345` → channel
//   - `discord:12345` → user
//   - `@12345` → user（需纯数字 ID）
//   - 纯数字 → 根据 defaultKind 决定，无默认则报错
//   - 其他 → channel
func ParseDiscordTarget(raw string, opts DiscordTargetParseOptions) (*DiscordTarget, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	// <@12345> or <@!12345>
	if m := discordUserMentionRe.FindStringSubmatch(trimmed); m != nil {
		return newDiscordTarget(DiscordTargetKindUser, m[1], trimmed), nil
	}

	// user:12345
	if strings.HasPrefix(trimmed, "user:") {
		id := strings.TrimSpace(trimmed[5:])
		if id == "" {
			return nil, nil
		}
		return newDiscordTarget(DiscordTargetKindUser, id, trimmed), nil
	}

	// channel:12345
	if strings.HasPrefix(trimmed, "channel:") {
		id := strings.TrimSpace(trimmed[8:])
		if id == "" {
			return nil, nil
		}
		return newDiscordTarget(DiscordTargetKindChannel, id, trimmed), nil
	}

	// discord:12345
	if strings.HasPrefix(trimmed, "discord:") {
		id := strings.TrimSpace(trimmed[8:])
		if id == "" {
			return nil, nil
		}
		return newDiscordTarget(DiscordTargetKindUser, id, trimmed), nil
	}

	// @12345
	if strings.HasPrefix(trimmed, "@") {
		candidate := strings.TrimSpace(trimmed[1:])
		if !discordNumericRe.MatchString(candidate) {
			return nil, fmt.Errorf("Discord DMs require a user id (use user:<id> or a <@id> mention)")
		}
		return newDiscordTarget(DiscordTargetKindUser, candidate, trimmed), nil
	}

	// 纯数字 ID
	if discordNumericRe.MatchString(trimmed) {
		if opts.DefaultKind != "" {
			return newDiscordTarget(opts.DefaultKind, trimmed, trimmed), nil
		}
		ambMsg := opts.AmbiguousMessage
		if ambMsg == "" {
			ambMsg = fmt.Sprintf(`Ambiguous Discord recipient "%s". Use "user:%s" for DMs or "channel:%s" for channel messages.`, trimmed, trimmed, trimmed)
		}
		return nil, fmt.Errorf("%s", ambMsg)
	}

	// 默认 → channel
	return newDiscordTarget(DiscordTargetKindChannel, trimmed, trimmed), nil
}

// ResolveDiscordChannelID 解析 Discord 频道 ID，失败时返回错误。
func ResolveDiscordChannelID(raw string) (string, error) {
	target, err := ParseDiscordTarget(raw, DiscordTargetParseOptions{DefaultKind: DiscordTargetKindChannel})
	if err != nil {
		return "", err
	}
	if target == nil || target.Kind != DiscordTargetKindChannel {
		return "", fmt.Errorf("discord: expected channel target, got %q", raw)
	}
	return target.ID, nil
}

// isLikelyUsername 判断字符串是否可能是 Discord 用户名
func isLikelyUsername(input string) bool {
	// 已知格式前缀或纯数字 → 不是用户名
	if discordKnownFormatRe.MatchString(input) || discordNumericRe.MatchString(input) {
		return false
	}
	return true
}

// isExplicitUserLookup 判断是否为显式用户查找
func isExplicitUserLookup(input string, opts DiscordTargetParseOptions) bool {
	if discordUserMentionRe.MatchString(input) {
		return true
	}
	if strings.HasPrefix(input, "user:") || strings.HasPrefix(input, "discord:") {
		return true
	}
	if strings.HasPrefix(input, "@") {
		return true
	}
	if discordNumericRe.MatchString(input) {
		return opts.DefaultKind == DiscordTargetKindUser
	}
	return false
}

// DirectoryLookupFunc 目录查找函数类型
type DirectoryLookupFunc func(ctx context.Context, query string, limit int) ([]DirectoryEntry, error)

// DirectoryEntry 目录条目
type DirectoryEntry struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// ResolveDiscordTarget 解析 Discord 目标，支持通过目录查找用户名。
func ResolveDiscordTarget(
	ctx context.Context,
	raw string,
	opts DiscordTargetParseOptions,
	lookupFn DirectoryLookupFunc,
) (*DiscordTarget, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	likelyUsername := isLikelyUsername(trimmed)
	shouldLookup := isExplicitUserLookup(trimmed, opts) || likelyUsername

	// 先尝试直接解析
	directParse, _ := ParseDiscordTarget(trimmed, opts)
	if directParse != nil && directParse.Kind != DiscordTargetKindChannel && !likelyUsername {
		return directParse, nil
	}

	if !shouldLookup {
		if directParse != nil {
			return directParse, nil
		}
		return ParseDiscordTarget(trimmed, opts)
	}

	// 尝试通过目录查找用户名
	if lookupFn != nil {
		entries, err := lookupFn(ctx, trimmed, 1)
		if err == nil && len(entries) > 0 {
			match := entries[0]
			if match.Kind == "user" {
				userID := strings.TrimPrefix(match.ID, "user:")
				return newDiscordTarget(DiscordTargetKindUser, userID, trimmed), nil
			}
		}
		// 查找失败 — 回退到原始解析
	}

	return ParseDiscordTarget(trimmed, opts)
}

// ResolveChannelIDFromParams 模拟 TS 的 resolveChannelId() 闭包。
// 先尝试 params["channelId"]，fallback 到 params["to"]。
func ResolveChannelIDFromParams(params map[string]interface{}) (string, error) {
	chID := channels.ReadStringParam(params, "channelId")
	if chID == "" {
		chID = channels.ReadStringParam(params, "to")
	}
	if chID == "" {
		return "", fmt.Errorf("channelId or to is required")
	}
	return ResolveDiscordChannelID(chID)
}
