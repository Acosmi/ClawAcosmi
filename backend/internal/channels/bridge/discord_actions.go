package bridge

import (
	"context"
	"encoding/json"
	"fmt"
)

// Discord action 路由 — 继承自 src/agents/tools/discord-actions.ts (77L)
// 实际逻辑拆分至 discord_actions_messaging.go / _guild.go / _moderation.go / _presence.go

// DiscordActionDeps Discord bridge action 依赖接口。
// 调用方注入 channel API 实现以避免循环导入。
type DiscordActionDeps interface {
	// ── Messaging ──────────────────────────────────────────────────────
	SendMessage(ctx context.Context, to, text, token string, opts DiscordBridgeSendOpts) (messageID, channelID string, err error)
	EditMessage(ctx context.Context, channelID, messageID, content, token string) (json.RawMessage, error)
	DeleteMessage(ctx context.Context, channelID, messageID, token string) error
	ReadMessages(ctx context.Context, channelID, token string, limit *int, before, after, around string) ([]json.RawMessage, error)
	FetchMessage(ctx context.Context, channelID, messageID, token string) (json.RawMessage, error)
	ReactMessage(ctx context.Context, channelID, messageID, emoji, token string) error
	RemoveReaction(ctx context.Context, channelID, messageID, emoji, token string) error
	RemoveOwnReactions(ctx context.Context, channelID, messageID, token string) ([]string, error)
	FetchReactions(ctx context.Context, channelID, messageID, token string, limit int) (interface{}, error)
	SendSticker(ctx context.Context, to string, stickerIDs []string, token, content string) (messageID, channelID string, err error)
	SendPoll(ctx context.Context, to string, poll interface{}, token, content string) (messageID, channelID string, err error)
	FetchPermissions(ctx context.Context, channelID, token string) (interface{}, error)
	PinMessage(ctx context.Context, channelID, messageID, token string) error
	UnpinMessage(ctx context.Context, channelID, messageID, token string) error
	ListPins(ctx context.Context, channelID, token string) ([]json.RawMessage, error)
	CreateThread(ctx context.Context, channelID string, payload DiscordBridgeThreadCreate, token string) (json.RawMessage, error)
	ListThreads(ctx context.Context, guildID, channelID, token string, includeArchived bool, before string, limit int) (json.RawMessage, error)
	SearchMessages(ctx context.Context, guildID, content, token string, channelIDs, authorIDs []string, limit int) (json.RawMessage, error)

	// ── Guild ──────────────────────────────────────────────────────────
	FetchMemberInfo(ctx context.Context, guildID, userID, token string) (interface{}, error)
	FetchRoleInfo(ctx context.Context, guildID, token string) (interface{}, error)
	AddRole(ctx context.Context, guildID, userID, roleID, token string) error
	RemoveRole(ctx context.Context, guildID, userID, roleID, token string) error
	FetchChannelInfo(ctx context.Context, channelID, token string) (json.RawMessage, error)
	ListChannels(ctx context.Context, guildID, token string) (interface{}, error)
	FetchVoiceStatus(ctx context.Context, guildID, token string) (interface{}, error)
	CreateChannel(ctx context.Context, guildID, name, token string, channelType *int, parentID, topic string) (json.RawMessage, error)
	EditChannel(ctx context.Context, channelID, token string, edits map[string]interface{}) (json.RawMessage, error)
	DeleteChannel(ctx context.Context, channelID, token string) error
	MoveChannel(ctx context.Context, guildID, channelID, token string, parentID *string, position *int) error
	SetChannelPermission(ctx context.Context, channelID, targetID, token string, targetType int, allow, deny string) error
	RemoveChannelPermission(ctx context.Context, channelID, targetID, token string) error
	UploadEmoji(ctx context.Context, guildID, name, mediaURL, token string, roleIDs []string) (json.RawMessage, error)
	UploadSticker(ctx context.Context, guildID, name, description, tags, mediaURL, token string) (json.RawMessage, error)
	ListScheduledEvents(ctx context.Context, guildID, token string) (interface{}, error)

	// ── Moderation ─────────────────────────────────────────────────────
	TimeoutMember(ctx context.Context, guildID, userID, token string, durationMinutes int, until, reason string) (interface{}, error)
	KickMember(ctx context.Context, guildID, userID, token, reason string) error
	BanMember(ctx context.Context, guildID, userID, token, reason string, deleteMessageDays int) error

	// ── Presence ───────────────────────────────────────────────────────
	SetPresence(ctx context.Context, status string, activities []DiscordBridgeActivity) error
	IsGatewayConnected(ctx context.Context) bool
}

// DiscordBridgeSendOpts 发送选项
type DiscordBridgeSendOpts struct {
	MediaURL  string
	EmbedJSON json.RawMessage
	ThreadID  string
	ReplyToID string
}

// DiscordBridgeThreadCreate 线程创建参数
type DiscordBridgeThreadCreate struct {
	MessageID          string
	Name               string
	AutoArchiveMinutes int
	Content            string
}

// DiscordBridgeActivity presence activity
type DiscordBridgeActivity struct {
	Name  string `json:"name"`
	Type  int    `json:"type"`
	URL   string `json:"url,omitempty"`
	State string `json:"state,omitempty"`
}

// discord action 分类集合
var discordMessagingActions = map[string]bool{
	"sendMessage": true, "editMessage": true, "deleteMessage": true,
	"readMessages": true, "fetchMessage": true, "searchMessages": true,
	"react": true, "reactions": true,
	"sendSticker": true, "sendPoll": true,
	"channelPermissions": true,
	"pinMessage":         true, "unpinMessage": true, "listPins": true,
	"createThread": true, "listThreads": true,
	"parseMessageLink": true,
}

var discordGuildActions = map[string]bool{
	"memberInfo": true, "roleInfo": true,
	"addRole": true, "removeRole": true,
	"channelInfo": true, "listChannels": true,
	"voiceStatus": true, "scheduledEvents": true,
	"createChannel": true, "editChannel": true, "deleteChannel": true,
	"moveChannel":          true,
	"setChannelPermission": true, "removeChannelPermission": true,
	"uploadEmoji": true, "uploadSticker": true,
}

var discordModerationActions = map[string]bool{
	"timeout": true, "kick": true, "ban": true,
}

var discordPresenceActions = map[string]bool{
	"setPresence": true,
}

// ResolveDiscordToken 从 params 或 actionGate 获取 bot token。
// 调用方负责注入 token，bridge 层仅做透传。
func ResolveDiscordToken(params map[string]interface{}) string {
	token, _ := ReadStringParam(params, "_token", false)
	return token
}

// HandleDiscordAction Discord action 分发路由
func HandleDiscordAction(ctx context.Context, params map[string]interface{}, actionGate ActionGate, deps DiscordActionDeps) (ToolResult, error) {
	action, err := ReadStringParam(params, "action", true)
	if err != nil {
		return ErrorResult(err), err
	}

	if discordMessagingActions[action] {
		return handleDiscordMessagingAction(ctx, action, params, actionGate, deps)
	}
	if discordGuildActions[action] {
		return handleDiscordGuildAction(ctx, action, params, actionGate, deps)
	}
	if discordModerationActions[action] {
		return handleDiscordModerationAction(ctx, action, params, actionGate, deps)
	}
	if discordPresenceActions[action] {
		return handleDiscordPresenceAction(ctx, action, params, actionGate, deps)
	}

	return ToolResult{}, fmt.Errorf("unknown discord action: %s", action)
}
