package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/markdown"
	"github.com/openacosmi/claw-acismi/pkg/polls"
)

// Discord 服务器管理 — 继承自 src/discord/send.guild.ts (141L)

// RecordChannelActivityFn 可选的频道活动记录回调（Phase 9 DI）。
var RecordChannelActivityFn func(channel, accountID, direction string)

// DirectoryLookupFnVar 可选的目录查找回调（Phase 9 DI）。
var DirectoryLookupFnVar DirectoryLookupFunc

// FetchMemberInfoDiscord 获取成员信息
func FetchMemberInfoDiscord(ctx context.Context, guildID, userID, token string) (json.RawMessage, error) {
	return discordGET(ctx, fmt.Sprintf("/guilds/%s/members/%s", guildID, userID), token)
}

// FetchRoleInfoDiscord 获取角色列表
func FetchRoleInfoDiscord(ctx context.Context, guildID, token string) (json.RawMessage, error) {
	return discordGET(ctx, fmt.Sprintf("/guilds/%s/roles", guildID), token)
}

// AddRoleDiscord 添加角色
func AddRoleDiscord(ctx context.Context, p DiscordRoleChange, token string) error {
	_, err := discordPUT(ctx, fmt.Sprintf("/guilds/%s/members/%s/roles/%s", p.GuildID, p.UserID, p.RoleID), token, nil)
	return err
}

// RemoveRoleDiscord 移除角色
func RemoveRoleDiscord(ctx context.Context, p DiscordRoleChange, token string) error {
	path := fmt.Sprintf("/guilds/%s/members/%s/roles/%s", p.GuildID, p.UserID, p.RoleID)
	var extraHeaders map[string]string
	if p.Reason != "" {
		extraHeaders = map[string]string{
			"X-Audit-Log-Reason": url.PathEscape(p.Reason),
		}
	}
	_, err := discordRESTWithHeaders(ctx, http.MethodDelete, path, token, nil, extraHeaders)
	return err
}

// FetchChannelInfoDiscord 获取频道信息
func FetchChannelInfoDiscord(ctx context.Context, channelID, token string) (json.RawMessage, error) {
	return discordGET(ctx, fmt.Sprintf("/channels/%s", channelID), token)
}

// ListGuildChannelsDiscord 列出服务器频道
func ListGuildChannelsDiscord(ctx context.Context, guildID, token string) (json.RawMessage, error) {
	return discordGET(ctx, fmt.Sprintf("/guilds/%s/channels", guildID), token)
}

// FetchVoiceStatusDiscord 获取语音状态
func FetchVoiceStatusDiscord(ctx context.Context, guildID, userID, token string) (json.RawMessage, error) {
	return discordGET(ctx, fmt.Sprintf("/guilds/%s/voice-states/%s", guildID, userID), token)
}

// ListScheduledEventsDiscord 列出计划事件
func ListScheduledEventsDiscord(ctx context.Context, guildID, token string) (json.RawMessage, error) {
	return discordGET(ctx, fmt.Sprintf("/guilds/%s/scheduled-events", guildID), token)
}

// CreateScheduledEventDiscord 创建计划事件
func CreateScheduledEventDiscord(ctx context.Context, guildID string, payload interface{}, token string) (json.RawMessage, error) {
	return discordPOST(ctx, fmt.Sprintf("/guilds/%s/scheduled-events", guildID), token, payload)
}

// TimeoutMemberDiscord 超时成员
func TimeoutMemberDiscord(ctx context.Context, p DiscordTimeoutTarget, token string) (json.RawMessage, error) {
	until := p.Until
	if until == "" && p.DurationMinutes > 0 {
		t := time.Now().Add(time.Duration(p.DurationMinutes) * time.Minute)
		until = t.UTC().Format(time.RFC3339)
	}
	body := map[string]interface{}{"communication_disabled_until": nil}
	if until != "" {
		body["communication_disabled_until"] = until
	}
	path := fmt.Sprintf("/guilds/%s/members/%s", p.GuildID, p.UserID)
	var extraHeaders map[string]string
	if p.Reason != "" {
		extraHeaders = map[string]string{
			"X-Audit-Log-Reason": url.PathEscape(p.Reason),
		}
	}
	return discordRESTWithHeaders(ctx, http.MethodPatch, path, token, body, extraHeaders)
}

// KickMemberDiscord 踢出成员
func KickMemberDiscord(ctx context.Context, p DiscordModerationTarget, token string) error {
	path := fmt.Sprintf("/guilds/%s/members/%s", p.GuildID, p.UserID)
	var extraHeaders map[string]string
	if p.Reason != "" {
		extraHeaders = map[string]string{
			"X-Audit-Log-Reason": url.PathEscape(p.Reason),
		}
	}
	_, err := discordRESTWithHeaders(ctx, http.MethodDelete, path, token, nil, extraHeaders)
	return err
}

// BanMemberDiscord 封禁成员
func BanMemberDiscord(ctx context.Context, p DiscordModerationTarget, deleteMessageDays *int, token string) error {
	var body interface{}
	if deleteMessageDays != nil {
		days := int(math.Min(math.Max(float64(*deleteMessageDays), 0), 7))
		body = map[string]interface{}{"delete_message_days": days}
	}
	path := fmt.Sprintf("/guilds/%s/bans/%s", p.GuildID, p.UserID)
	var extraHeaders map[string]string
	if p.Reason != "" {
		extraHeaders = map[string]string{
			"X-Audit-Log-Reason": url.PathEscape(p.Reason),
		}
	}
	_, err := discordRESTWithHeaders(ctx, http.MethodPut, path, token, body, extraHeaders)
	return err
}

// ListGuildEmojisDiscord 列出服务器 emoji
func ListGuildEmojisDiscord(ctx context.Context, guildID, token string) (json.RawMessage, error) {
	return discordGET(ctx, fmt.Sprintf("/guilds/%s/emojis", guildID), token)
}

// SendMessageDiscord 发送消息（主入口）
// 继承自 send.outbound.ts sendMessageDiscord (L34-98)
func SendMessageDiscord(ctx context.Context, cfg SendMessageConfig) (*DiscordSendResult, error) {
	recipient, err := ParseAndResolveRecipient(ctx, cfg.To, cfg.Token, cfg.AccountID)
	if err != nil {
		return nil, err
	}
	channelID, _, err := ResolveChannelIDFromRecipient(ctx, cfg.Token, recipient)
	if err != nil {
		return nil, err
	}

	opts := SendDiscordTextOpts{
		Embeds:    cfg.Embeds,
		ChunkMode: cfg.ChunkMode,
	}

	text := cfg.Text
	if cfg.TableMode != "" {
		text = markdown.ConvertMarkdownTables(text, markdown.TableMode(cfg.TableMode))
	}

	var result *DiscordSendResult
	if cfg.MediaURL != "" {
		result, err = SendDiscordMedia(ctx, cfg.Token, channelID, text, cfg.MediaURL, cfg.ReplyTo, cfg.MaxLinesPerMessage, opts)
	} else {
		result, err = SendDiscordText(ctx, cfg.Token, channelID, text, cfg.ReplyTo, cfg.MaxLinesPerMessage, opts)
	}
	if err != nil {
		return nil, BuildDiscordSendErrorFromErr(err, channelID, ctx, cfg.Token, cfg.MediaURL != "")
	}
	// Phase 9: recordChannelActivity via DI callback
	if RecordChannelActivityFn != nil {
		RecordChannelActivityFn("discord", cfg.AccountID, "outbound")
	}
	return result, nil
}

// SendMessageConfig 发送消息配置
type SendMessageConfig struct {
	To                 string
	Text               string
	Token              string
	AccountID          string
	ReplyTo            string
	MaxLinesPerMessage int
	MediaURL           string        // 附件 URL
	Embeds             []interface{} // 嵌入对象
	ChunkMode          ChunkMode     // 分块模式
	TableMode          string        // Markdown 表格转换模式 ("off"/"bullets"/"code")
}

// SendStickerDiscord 发送贴纸消息
func SendStickerDiscord(ctx context.Context, to string, stickerIDs []string, token string, content string) (*DiscordSendResult, error) {
	recipient, err := ParseRecipient(to)
	if err != nil {
		return nil, err
	}
	channelID, _, err := ResolveChannelIDFromRecipient(ctx, token, recipient)
	if err != nil {
		return nil, err
	}
	stickers, err := NormalizeStickerIds(stickerIDs)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{"sticker_ids": stickers}
	if c := trimOrEmpty(content); c != "" {
		body["content"] = c
	}
	resp, err := discordPOST(ctx, fmt.Sprintf("/channels/%s/messages", channelID), token, body)
	if err != nil {
		return nil, err
	}
	var msg struct {
		ID        string `json:"id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal(resp, &msg); err != nil {
		return nil, fmt.Errorf("parse sticker response: %w", err)
	}
	return &DiscordSendResult{MessageID: orDefault(msg.ID, "unknown"), ChannelID: orDefault(msg.ChannelID, channelID)}, nil
}

// DiscordPollInput 投票输入（类型化）
type DiscordPollInput struct {
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	DurationHours int      `json:"durationHours,omitempty"`
	MaxSelections int      `json:"maxSelections,omitempty"`
}

const (
	discordPollMaxAnswers      = 10
	discordPollMaxDurationHrs  = 32 * 24 // 32 天
	discordPollDefaultDuration = 24
)

// NormalizeDiscordPollInput 规范化 Discord 投票输入。
// 使用 pkg/polls 共享验证 + Discord 平台特有默认值和 API 格式化。
// 继承自 send.shared.ts normalizeDiscordPollInput (L169-184)
func NormalizeDiscordPollInput(input DiscordPollInput) (map[string]interface{}, error) {
	// Discord 特有：空问题默认为 "Poll"（共享层会报错）
	question := strings.TrimSpace(input.Question)
	if question == "" {
		question = "Poll"
	}

	// 调用共享验证层
	normalized, err := polls.NormalizePollInput(polls.PollInput{
		Question:      question,
		Options:       input.Options,
		MaxSelections: input.MaxSelections,
	}, discordPollMaxAnswers)
	if err != nil {
		return nil, fmt.Errorf("discord poll validation: %w", err)
	}

	// Discord 特有：时长钳位到 [1, 768h]
	duration := polls.NormalizePollDurationHours(
		float64(input.DurationHours),
		float64(discordPollDefaultDuration),
		float64(discordPollMaxDurationHrs),
	)

	// 格式化为 Discord API 结构
	answers := make([]map[string]interface{}, 0, len(normalized.Options))
	for _, opt := range normalized.Options {
		answers = append(answers, map[string]interface{}{
			"poll_media": map[string]string{"text": opt},
		})
	}

	return map[string]interface{}{
		"question":          map[string]string{"text": normalized.Question},
		"answers":           answers,
		"duration":          int(duration),
		"allow_multiselect": normalized.MaxSelections > 1,
		"layout_type":       1, // PollLayoutType.Default
	}, nil
}

// SendPollDiscord 发送投票消息
func SendPollDiscord(ctx context.Context, to string, poll DiscordPollInput, token string, content string) (*DiscordSendResult, error) {
	recipient, err := ParseRecipient(to)
	if err != nil {
		return nil, err
	}
	channelID, _, err := ResolveChannelIDFromRecipient(ctx, token, recipient)
	if err != nil {
		return nil, err
	}
	normalized, err := NormalizeDiscordPollInput(poll)
	if err != nil {
		return nil, err
	}
	body := map[string]interface{}{"poll": normalized}
	if c := trimOrEmpty(content); c != "" {
		body["content"] = c
	}
	resp, err := discordPOST(ctx, fmt.Sprintf("/channels/%s/messages", channelID), token, body)
	if err != nil {
		return nil, err
	}
	var msg struct {
		ID        string `json:"id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal(resp, &msg); err != nil {
		return nil, fmt.Errorf("parse poll response: %w", err)
	}
	return &DiscordSendResult{MessageID: orDefault(msg.ID, "unknown"), ChannelID: orDefault(msg.ChannelID, channelID)}, nil
}

// ParseAndResolveRecipient 解析并解析接收方（含目录查找）
// 继承自 send.shared.ts parseAndResolveRecipient (L105-148)
// 先尝试目录查找（支持用户名发送 DM），回退到标准解析
func ParseAndResolveRecipient(ctx context.Context, raw, token, accountID string) (DiscordRecipient, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DiscordRecipient{}, fmt.Errorf("recipient is required for Discord sends")
	}

	opts := DiscordTargetParseOptions{
		AmbiguousMessage: fmt.Sprintf(
			`Ambiguous Discord recipient "%s". Use "user:%s" for DMs or "channel:%s" for channel messages.`,
			trimmed, trimmed, trimmed),
	}

	// Phase 9: DirectoryLookupFunc via DI callback
	resolved, err := ResolveDiscordTarget(ctx, raw, opts, DirectoryLookupFnVar)
	if err == nil && resolved != nil {
		return DiscordRecipient{Kind: resolved.Kind, ID: resolved.ID}, nil
	}

	// 回退到标准解析
	parsed, err := ParseDiscordTarget(raw, opts)
	if err != nil {
		return DiscordRecipient{}, err
	}
	if parsed == nil {
		return DiscordRecipient{}, fmt.Errorf("recipient is required for Discord sends")
	}
	return DiscordRecipient{Kind: parsed.Kind, ID: parsed.ID}, nil
}

func trimOrEmpty(s string) string {
	if s == "" {
		return ""
	}
	return s
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// 确保 url 包被引用
var _ = url.Values{}
