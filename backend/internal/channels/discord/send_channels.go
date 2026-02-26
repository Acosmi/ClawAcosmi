package discord

import (
	"context"
	"encoding/json"
	"fmt"
)

// Discord 频道管理 — 继承自 src/discord/send.channels.ts (115L)

// CreateChannelDiscord 创建频道
func CreateChannelDiscord(ctx context.Context, payload DiscordChannelCreate, token string) (json.RawMessage, error) {
	body := map[string]interface{}{"name": payload.Name}
	if payload.Type != nil {
		body["type"] = *payload.Type
	}
	if payload.ParentID != "" {
		body["parent_id"] = payload.ParentID
	}
	if payload.Topic != "" {
		body["topic"] = payload.Topic
	}
	if payload.Position != nil {
		body["position"] = *payload.Position
	}
	if payload.NSFW != nil {
		body["nsfw"] = *payload.NSFW
	}
	return discordPOST(ctx, fmt.Sprintf("/guilds/%s/channels", payload.GuildID), token, body)
}

// EditChannelDiscord 编辑频道
func EditChannelDiscord(ctx context.Context, payload DiscordChannelEdit, token string) (json.RawMessage, error) {
	body := map[string]interface{}{}
	if payload.Name != "" {
		body["name"] = payload.Name
	}
	if payload.Topic != nil {
		body["topic"] = *payload.Topic
	}
	if payload.Position != nil {
		body["position"] = *payload.Position
	}
	if payload.ParentID != nil {
		body["parent_id"] = *payload.ParentID
	}
	if payload.NSFW != nil {
		body["nsfw"] = *payload.NSFW
	}
	if payload.RateLimitPerUser != nil {
		body["rate_limit_per_user"] = *payload.RateLimitPerUser
	}
	return discordPATCH(ctx, fmt.Sprintf("/channels/%s", payload.ChannelID), token, body)
}

// DeleteChannelDiscord 删除频道
func DeleteChannelDiscord(ctx context.Context, channelID, token string) error {
	return discordDELETE(ctx, fmt.Sprintf("/channels/%s", channelID), token)
}

// MoveChannelDiscord 移动频道
func MoveChannelDiscord(ctx context.Context, payload DiscordChannelMove, token string) error {
	entry := map[string]interface{}{"id": payload.ChannelID}
	if payload.ParentID != nil {
		entry["parent_id"] = *payload.ParentID
	}
	if payload.Position != nil {
		entry["position"] = *payload.Position
	}
	_, err := discordPATCH(ctx, fmt.Sprintf("/guilds/%s/channels", payload.GuildID), token, []interface{}{entry})
	return err
}

// SetChannelPermissionDiscord 设置频道权限覆盖
func SetChannelPermissionDiscord(ctx context.Context, payload DiscordChannelPermissionSet, token string) error {
	body := map[string]interface{}{"type": payload.TargetType}
	if payload.Allow != "" {
		body["allow"] = payload.Allow
	}
	if payload.Deny != "" {
		body["deny"] = payload.Deny
	}
	_, err := discordPUT(ctx, fmt.Sprintf("/channels/%s/permissions/%s", payload.ChannelID, payload.TargetID), token, body)
	return err
}

// RemoveChannelPermissionDiscord 移除频道权限覆盖
func RemoveChannelPermissionDiscord(ctx context.Context, channelID, targetID, token string) error {
	return discordDELETE(ctx, fmt.Sprintf("/channels/%s/permissions/%s", channelID, targetID), token)
}
