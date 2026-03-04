package discord

import (
	"github.com/Acosmi/ClawAcosmi/pkg/retry"
)

// Discord 发送类型 — 继承自 src/discord/send.types.ts (159L)

// DiscordSendErrorKind 发送错误类型
type DiscordSendErrorKind string

const (
	DiscordSendErrorKindMissingPerms DiscordSendErrorKind = "missing-permissions"
	DiscordSendErrorKindDMBlocked    DiscordSendErrorKind = "dm-blocked"
)

// DiscordSendError 发送错误
type DiscordSendError struct {
	Kind               DiscordSendErrorKind
	ChannelID          string
	MissingPermissions []string
	Message            string
}

func (e *DiscordSendError) Error() string { return e.Message }

// 大小限制常量
const (
	DiscordMaxEmojiBytes   = 256 * 1024
	DiscordMaxStickerBytes = 512 * 1024
)

// DiscordSendResult 发送结果
type DiscordSendResult struct {
	MessageID string `json:"messageId"`
	ChannelID string `json:"channelId"`
}

// DiscordReactOpts 反应操作选项
type DiscordReactOpts struct {
	Token     string
	AccountID string
	Verbose   bool
	Retry     *retry.Config
}

// DiscordReactionUser 反应用户
type DiscordReactionUser struct {
	ID       string `json:"id"`
	Username string `json:"username,omitempty"`
	Tag      string `json:"tag,omitempty"`
}

// DiscordReactionSummary 反应摘要
type DiscordReactionSummary struct {
	Emoji DiscordEmojiRef       `json:"emoji"`
	Count int                   `json:"count"`
	Users []DiscordReactionUser `json:"users"`
}

// DiscordEmojiRef emoji 引用
type DiscordEmojiRef struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Raw  string `json:"raw"`
}

// DiscordPermissionsSummary 权限摘要
type DiscordPermissionsSummary struct {
	ChannelID   string   `json:"channelId"`
	GuildID     string   `json:"guildId,omitempty"`
	Permissions []string `json:"permissions"`
	Raw         string   `json:"raw"`
	IsDM        bool     `json:"isDm"`
	ChannelType *int     `json:"channelType,omitempty"`
}

// DiscordMessageQuery 消息查询参数
type DiscordMessageQuery struct {
	Limit  *int   `json:"limit,omitempty"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Around string `json:"around,omitempty"`
}

// DiscordMessageEdit 消息编辑
type DiscordMessageEdit struct {
	Content string `json:"content,omitempty"`
}

// DiscordThreadCreate 线程创建
type DiscordThreadCreate struct {
	MessageID          string `json:"messageId,omitempty"`
	Name               string `json:"name"`
	AutoArchiveMinutes int    `json:"autoArchiveMinutes,omitempty"`
	Content            string `json:"content,omitempty"`
}

// DiscordThreadList 线程列表查询
type DiscordThreadList struct {
	GuildID         string `json:"guildId"`
	ChannelID       string `json:"channelId,omitempty"`
	IncludeArchived bool   `json:"includeArchived,omitempty"`
	Before          string `json:"before,omitempty"`
	Limit           int    `json:"limit,omitempty"`
}

// DiscordSearchQuery 消息搜索
type DiscordSearchQuery struct {
	GuildID    string   `json:"guildId"`
	Content    string   `json:"content"`
	ChannelIDs []string `json:"channelIds,omitempty"`
	AuthorIDs  []string `json:"authorIds,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// DiscordRoleChange 角色变更
type DiscordRoleChange struct {
	GuildID string `json:"guildId"`
	UserID  string `json:"userId"`
	RoleID  string `json:"roleId"`
	Reason  string `json:"reason,omitempty"` // X-Audit-Log-Reason
}

// DiscordModerationTarget 管理操作目标
type DiscordModerationTarget struct {
	GuildID string `json:"guildId"`
	UserID  string `json:"userId"`
	Reason  string `json:"reason,omitempty"`
}

// DiscordTimeoutTarget 超时目标
type DiscordTimeoutTarget struct {
	DiscordModerationTarget
	Until           string `json:"until,omitempty"`
	DurationMinutes int    `json:"durationMinutes,omitempty"`
	Reason          string `json:"reason,omitempty"` // X-Audit-Log-Reason
}

// DiscordEmojiUpload emoji 上传
type DiscordEmojiUpload struct {
	GuildID  string   `json:"guildId"`
	Name     string   `json:"name"`
	MediaURL string   `json:"mediaUrl"`
	RoleIDs  []string `json:"roleIds,omitempty"`
}

// DiscordStickerUpload sticker 上传
type DiscordStickerUpload struct {
	GuildID     string `json:"guildId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	MediaURL    string `json:"mediaUrl"`
}

// DiscordChannelCreate 频道创建
type DiscordChannelCreate struct {
	GuildID  string `json:"guildId"`
	Name     string `json:"name"`
	Type     *int   `json:"type,omitempty"`
	ParentID string `json:"parentId,omitempty"`
	Topic    string `json:"topic,omitempty"`
	Position *int   `json:"position,omitempty"`
	NSFW     *bool  `json:"nsfw,omitempty"`
}

// DiscordChannelEdit 频道编辑
type DiscordChannelEdit struct {
	ChannelID        string  `json:"channelId"`
	Name             string  `json:"name,omitempty"`
	Topic            *string `json:"topic,omitempty"`
	Position         *int    `json:"position,omitempty"`
	ParentID         *string `json:"parentId,omitempty"`
	NSFW             *bool   `json:"nsfw,omitempty"`
	RateLimitPerUser *int    `json:"rateLimitPerUser,omitempty"`
}

// DiscordChannelMove 频道移动
type DiscordChannelMove struct {
	GuildID   string  `json:"guildId"`
	ChannelID string  `json:"channelId"`
	ParentID  *string `json:"parentId,omitempty"`
	Position  *int    `json:"position,omitempty"`
}

// DiscordChannelPermissionSet 频道权限设置
type DiscordChannelPermissionSet struct {
	ChannelID  string `json:"channelId"`
	TargetID   string `json:"targetId"`
	TargetType int    `json:"targetType"` // 0=role, 1=member
	Allow      string `json:"allow,omitempty"`
	Deny       string `json:"deny,omitempty"`
}
