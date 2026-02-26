package channels

import (
	"context"
	"errors"
	"fmt"
)

// ErrUnsupportedAction sentinel error 表示动作不被支持，让调用方可以区分"不匹配"与其他错误
var ErrUnsupportedAction = errors.New("unsupported action")

// Discord Guild Admin 动作处理器 — 继承自 handle-action.guild-admin.ts (438L)

// DiscordGuildAdminActionType Guild 管理动作类型
type DiscordGuildAdminActionType string

const (
	DiscordAdminMemberInfo     DiscordGuildAdminActionType = "member-info"
	DiscordAdminRoleInfo       DiscordGuildAdminActionType = "role-info"
	DiscordAdminEmojiList      DiscordGuildAdminActionType = "emoji-list"
	DiscordAdminEmojiUpload    DiscordGuildAdminActionType = "emoji-upload"
	DiscordAdminStickerUpload  DiscordGuildAdminActionType = "sticker-upload"
	DiscordAdminRoleAdd        DiscordGuildAdminActionType = "role-add"
	DiscordAdminRoleRemove     DiscordGuildAdminActionType = "role-remove"
	DiscordAdminChannelInfo    DiscordGuildAdminActionType = "channel-info"
	DiscordAdminChannelList    DiscordGuildAdminActionType = "channel-list"
	DiscordAdminChannelCreate  DiscordGuildAdminActionType = "channel-create"
	DiscordAdminChannelEdit    DiscordGuildAdminActionType = "channel-edit"
	DiscordAdminChannelDelete  DiscordGuildAdminActionType = "channel-delete"
	DiscordAdminChannelMove    DiscordGuildAdminActionType = "channel-move"
	DiscordAdminCategoryCreate DiscordGuildAdminActionType = "category-create"
	DiscordAdminCategoryEdit   DiscordGuildAdminActionType = "category-edit"
	DiscordAdminCategoryDelete DiscordGuildAdminActionType = "category-delete"
	DiscordAdminVoiceStatus    DiscordGuildAdminActionType = "voice-status"
	DiscordAdminEventList      DiscordGuildAdminActionType = "event-list"
	DiscordAdminEventCreate    DiscordGuildAdminActionType = "event-create"
	DiscordAdminTimeout        DiscordGuildAdminActionType = "timeout"
	DiscordAdminKick           DiscordGuildAdminActionType = "kick"
	DiscordAdminBan            DiscordGuildAdminActionType = "ban"
	DiscordAdminThreadList     DiscordGuildAdminActionType = "thread-list"
	DiscordAdminThreadReply    DiscordGuildAdminActionType = "thread-reply"
	DiscordAdminSearch         DiscordGuildAdminActionType = "search"
)

// IsDiscordGuildAdminAction 判断是否为 Guild Admin 动作
func IsDiscordGuildAdminAction(action string) bool {
	switch DiscordGuildAdminActionType(action) {
	case DiscordAdminMemberInfo, DiscordAdminRoleInfo,
		DiscordAdminEmojiList, DiscordAdminEmojiUpload,
		DiscordAdminStickerUpload,
		DiscordAdminRoleAdd, DiscordAdminRoleRemove,
		DiscordAdminChannelInfo, DiscordAdminChannelList,
		DiscordAdminChannelCreate, DiscordAdminChannelEdit,
		DiscordAdminChannelDelete, DiscordAdminChannelMove,
		DiscordAdminCategoryCreate, DiscordAdminCategoryEdit,
		DiscordAdminCategoryDelete,
		DiscordAdminVoiceStatus,
		DiscordAdminEventList, DiscordAdminEventCreate,
		DiscordAdminTimeout, DiscordAdminKick, DiscordAdminBan,
		DiscordAdminThreadList, DiscordAdminThreadReply,
		DiscordAdminSearch:
		return true
	}
	return false
}

// BuildDiscordGuildAdminRequest 构建 Guild Admin 动作请求
func BuildDiscordGuildAdminRequest(ctx context.Context, action string, params map[string]interface{}, accountID string) (*DiscordActionRequest, error) {
	req := &DiscordActionRequest{
		AccountID: accountID,
		Extra:     make(map[string]interface{}),
	}

	switch DiscordGuildAdminActionType(action) {
	case DiscordAdminMemberInfo:
		req.Action = "memberInfo"
		guildID := ReadStringParam(params, "guildId")
		if guildID == "" {
			return nil, fmt.Errorf("parameter \"guildId\" is required")
		}
		userID := ReadStringParam(params, "userId")
		if userID == "" {
			return nil, fmt.Errorf("parameter \"userId\" is required")
		}
		req.Extra["guildId"] = guildID
		req.Extra["userId"] = userID
	case DiscordAdminRoleInfo:
		req.Action = "roleInfo"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
	case DiscordAdminEmojiList:
		req.Action = "emojiList"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
	case DiscordAdminEmojiUpload:
		req.Action = "emojiUpload"
		guildID := ReadStringParam(params, "guildId")
		if guildID == "" {
			return nil, fmt.Errorf("parameter \"guildId\" is required")
		}
		req.Extra["guildId"] = guildID
		emojiName := ReadStringParam(params, "emojiName")
		if emojiName == "" {
			return nil, fmt.Errorf("parameter \"emojiName\" is required")
		}
		req.Extra["name"] = emojiName
		media := ReadStringParam(params, "media")
		if media == "" {
			return nil, fmt.Errorf("parameter \"media\" is required")
		}
		req.Extra["mediaUrl"] = media
		req.Extra["roleIds"] = ReadStringArrayParam(params, "roleIds")
	case DiscordAdminStickerUpload:
		req.Action = "stickerUpload"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.Extra["name"] = ReadStringParam(params, "stickerName")
		req.Extra["description"] = ReadStringParam(params, "stickerDesc")
		req.Extra["tags"] = ReadStringParam(params, "stickerTags")
		req.Extra["mediaUrl"] = ReadStringParam(params, "media")
	case DiscordAdminRoleAdd:
		req.Action = "roleAdd"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.Extra["userId"] = ReadStringParam(params, "userId")
		req.Extra["roleId"] = ReadStringParam(params, "roleId")
	case DiscordAdminRoleRemove:
		req.Action = "roleRemove"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.Extra["userId"] = ReadStringParam(params, "userId")
		req.Extra["roleId"] = ReadStringParam(params, "roleId")
	case DiscordAdminChannelInfo:
		req.Action = "channelInfo"
		req.ChannelID = ReadStringParam(params, "channelId")
	case DiscordAdminChannelList:
		req.Action = "channelList"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
	case DiscordAdminChannelCreate:
		req.Action = "channelCreate"
		guildID := ReadStringParam(params, "guildId")
		if guildID == "" {
			return nil, fmt.Errorf("parameter \"guildId\" is required")
		}
		req.Extra["guildId"] = guildID
		name := ReadStringParam(params, "name")
		if name == "" {
			return nil, fmt.Errorf("parameter \"name\" is required")
		}
		req.Extra["name"] = name
		if t := ReadIntParam(params, "type"); t != nil {
			req.Extra["type"] = *t
		}
		parentID := ReadParentIDParam(params)
		if parentID != nil {
			req.Extra["parentId"] = *parentID
		}
		req.Extra["topic"] = ReadStringParam(params, "topic")
		if pos := ReadIntParam(params, "position"); pos != nil {
			req.Extra["position"] = *pos
		}
		if nsfw, ok := params["nsfw"].(bool); ok {
			req.Extra["nsfw"] = nsfw
		}
	case DiscordAdminChannelEdit:
		req.Action = "channelEdit"
		channelID := ReadStringParam(params, "channelId")
		if channelID == "" {
			return nil, fmt.Errorf("parameter \"channelId\" is required")
		}
		req.ChannelID = channelID
		req.Extra["name"] = ReadStringParam(params, "name")
		req.Extra["topic"] = ReadStringParam(params, "topic")
		if pos := ReadIntParam(params, "position"); pos != nil {
			req.Extra["position"] = *pos
		}
		parentID := ReadParentIDParam(params)
		if parentID != nil {
			req.Extra["parentId"] = *parentID
		}
		if nsfw, ok := params["nsfw"].(bool); ok {
			req.Extra["nsfw"] = nsfw
		}
		if rateLimit := ReadIntParam(params, "rateLimitPerUser"); rateLimit != nil {
			req.Extra["rateLimitPerUser"] = *rateLimit
		}
	case DiscordAdminChannelDelete:
		req.Action = "channelDelete"
		channelID := ReadStringParam(params, "channelId")
		if channelID == "" {
			return nil, fmt.Errorf("parameter \"channelId\" is required")
		}
		req.ChannelID = channelID
	case DiscordAdminChannelMove:
		req.Action = "channelMove"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.ChannelID = ReadStringParam(params, "channelId")
		parentID := ReadParentIDParam(params)
		if parentID != nil {
			req.Extra["parentId"] = *parentID
		}
		if pos := ReadIntParam(params, "position"); pos != nil {
			req.Extra["position"] = *pos
		}
	case DiscordAdminCategoryCreate:
		req.Action = "categoryCreate"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.Extra["name"] = ReadStringParam(params, "name")
		if pos := ReadIntParam(params, "position"); pos != nil {
			req.Extra["position"] = *pos
		}
	case DiscordAdminCategoryEdit:
		req.Action = "categoryEdit"
		req.Extra["categoryId"] = ReadStringParam(params, "categoryId")
		req.Extra["name"] = ReadStringParam(params, "name")
		if pos := ReadIntParam(params, "position"); pos != nil {
			req.Extra["position"] = *pos
		}
	case DiscordAdminCategoryDelete:
		req.Action = "categoryDelete"
		req.Extra["categoryId"] = ReadStringParam(params, "categoryId")
	case DiscordAdminVoiceStatus:
		req.Action = "voiceStatus"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.Extra["userId"] = ReadStringParam(params, "userId")
	case DiscordAdminEventList:
		req.Action = "eventList"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
	case DiscordAdminEventCreate:
		req.Action = "eventCreate"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.Extra["name"] = ReadStringParam(params, "eventName")
		req.Extra["startTime"] = ReadStringParam(params, "startTime")
		req.Extra["endTime"] = ReadStringParam(params, "endTime")
		req.Extra["description"] = ReadStringParam(params, "desc")
		req.Extra["channelId"] = ReadStringParam(params, "channelId")
		req.Extra["location"] = ReadStringParam(params, "location")
		req.Extra["entityType"] = ReadStringParam(params, "eventType")
	case DiscordAdminTimeout, DiscordAdminKick, DiscordAdminBan:
		req.Action = action
		guildID := ReadStringParam(params, "guildId")
		if guildID == "" {
			return nil, fmt.Errorf("parameter \"guildId\" is required")
		}
		req.Extra["guildId"] = guildID
		userID := ReadStringParam(params, "userId")
		if userID == "" {
			return nil, fmt.Errorf("parameter \"userId\" is required")
		}
		req.Extra["userId"] = userID
		if durMin := ReadIntParam(params, "durationMin"); durMin != nil {
			req.Extra["durationMinutes"] = *durMin
		}
		req.Extra["until"] = ReadStringParam(params, "until")
		req.Extra["reason"] = ReadStringParam(params, "reason")
		if deleteDays := ReadIntParam(params, "deleteDays"); deleteDays != nil {
			req.Extra["deleteMessageDays"] = *deleteDays
		}
	case DiscordAdminThreadList:
		req.Action = "threadList"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.ChannelID = ReadStringParam(params, "channelId")
		if archived, ok := params["includeArchived"].(bool); ok {
			req.Extra["includeArchived"] = archived
		}
		req.Extra["before"] = ReadStringParam(params, "before")
		if limit := ReadIntParam(params, "limit"); limit != nil {
			req.Extra["limit"] = *limit
		}
	case DiscordAdminThreadReply:
		req.Action = "threadReply"
		threadID := ReadStringParam(params, "threadId")
		if threadID != "" {
			req.ChannelID = threadID
		} else {
			chID := ReadStringParam(params, "channelId")
			if chID == "" {
				chID = ReadStringParam(params, "to")
			}
			req.ChannelID = chID
		}
		req.Content = ReadStringParam(params, "message")
		req.MediaURL = ReadStringParam(params, "media")
		req.ReplyTo = ReadStringParam(params, "replyTo")
	case DiscordAdminSearch:
		req.Action = "searchMessages"
		req.Extra["guildId"] = ReadStringParam(params, "guildId")
		req.Content = ReadStringParam(params, "query")
		req.ChannelID = ReadStringParam(params, "channelId")
		req.Extra["channelIds"] = ReadStringArrayParam(params, "channelIds")
		req.Extra["authorId"] = ReadStringParam(params, "authorId")
		req.Extra["authorIds"] = ReadStringArrayParam(params, "authorIds")
		if limit := ReadIntParam(params, "limit"); limit != nil {
			req.Extra["limit"] = *limit
		}
	default:
		return nil, fmt.Errorf("%w: guild admin action %s", ErrUnsupportedAction, action)
	}

	return req, nil
}
