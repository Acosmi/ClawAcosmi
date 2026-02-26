package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
)

// Discord 消息操作 — 继承自 src/discord/send.messages.ts (172L)

// ReadMessagesDiscord 读取频道消息
func ReadMessagesDiscord(ctx context.Context, channelID string, query DiscordMessageQuery, token string) ([]json.RawMessage, error) {
	params := url.Values{}
	if query.Limit != nil {
		limit := int(math.Min(math.Max(float64(*query.Limit), 1), 100))
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if query.Before != "" {
		params.Set("before", query.Before)
	}
	if query.After != "" {
		params.Set("after", query.After)
	}
	if query.Around != "" {
		params.Set("around", query.Around)
	}
	path := fmt.Sprintf("/channels/%s/messages", channelID)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	resp, err := discordGET(ctx, path, token)
	if err != nil {
		return nil, err
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(resp, &messages); err != nil {
		return nil, fmt.Errorf("parse messages: %w", err)
	}
	return messages, nil
}

// FetchMessageDiscord 获取单条消息
func FetchMessageDiscord(ctx context.Context, channelID, messageID, token string) (json.RawMessage, error) {
	return discordGET(ctx, fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID), token)
}

// EditMessageDiscord 编辑消息
func EditMessageDiscord(ctx context.Context, channelID, messageID string, payload DiscordMessageEdit, token string) (json.RawMessage, error) {
	body := map[string]interface{}{"content": payload.Content}
	return discordPATCH(ctx, fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID), token, body)
}

// DeleteMessageDiscord 删除消息
func DeleteMessageDiscord(ctx context.Context, channelID, messageID, token string) error {
	return discordDELETE(ctx, fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID), token)
}

// PinMessageDiscord 置顶消息
func PinMessageDiscord(ctx context.Context, channelID, messageID, token string) error {
	_, err := discordPUT(ctx, fmt.Sprintf("/channels/%s/pins/%s", channelID, messageID), token, nil)
	return err
}

// UnpinMessageDiscord 取消置顶消息
func UnpinMessageDiscord(ctx context.Context, channelID, messageID, token string) error {
	return discordDELETE(ctx, fmt.Sprintf("/channels/%s/pins/%s", channelID, messageID), token)
}

// ListPinsDiscord 列出置顶消息
func ListPinsDiscord(ctx context.Context, channelID, token string) ([]json.RawMessage, error) {
	resp, err := discordGET(ctx, fmt.Sprintf("/channels/%s/pins", channelID), token)
	if err != nil {
		return nil, err
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(resp, &messages); err != nil {
		return nil, fmt.Errorf("parse pins: %w", err)
	}
	return messages, nil
}

// CreateThreadDiscord 创建线程
func CreateThreadDiscord(ctx context.Context, channelID string, payload DiscordThreadCreate, token string) (json.RawMessage, error) {
	body := map[string]interface{}{
		"name": payload.Name,
	}
	if payload.AutoArchiveMinutes > 0 {
		body["auto_archive_duration"] = payload.AutoArchiveMinutes
	}

	// 检测频道类型（无 messageId 时）
	if payload.MessageID == "" {
		chData, err := discordGET(ctx, fmt.Sprintf("/channels/%s", channelID), token)
		if err == nil {
			var ch struct {
				Type *int `json:"type,omitempty"`
			}
			if json.Unmarshal(chData, &ch) == nil && ch.Type != nil {
				t := *ch.Type
				if t == channelTypeGuildForum || t == channelTypeGuildMedia {
					content := strings.TrimSpace(payload.Content)
					if content == "" {
						content = payload.Name
					}
					body["message"] = map[string]string{"content": content}
				}
			}
		}
	}

	var path string
	if payload.MessageID != "" {
		path = fmt.Sprintf("/channels/%s/messages/%s/threads", channelID, payload.MessageID)
	} else {
		path = fmt.Sprintf("/channels/%s/threads", channelID)
	}
	return discordPOST(ctx, path, token, body)
}

// ListThreadsDiscord 列出线程
func ListThreadsDiscord(ctx context.Context, payload DiscordThreadList, token string) (json.RawMessage, error) {
	if payload.IncludeArchived {
		if payload.ChannelID == "" {
			return nil, fmt.Errorf("channelId required to list archived threads")
		}
		params := url.Values{}
		if payload.Before != "" {
			params.Set("before", payload.Before)
		}
		if payload.Limit > 0 {
			params.Set("limit", fmt.Sprintf("%d", payload.Limit))
		}
		path := fmt.Sprintf("/channels/%s/threads/archived/public", payload.ChannelID)
		if len(params) > 0 {
			path += "?" + params.Encode()
		}
		return discordGET(ctx, path, token)
	}
	return discordGET(ctx, fmt.Sprintf("/guilds/%s/threads/active", payload.GuildID), token)
}

// SearchMessagesDiscord 搜索消息
func SearchMessagesDiscord(ctx context.Context, query DiscordSearchQuery, token string) (json.RawMessage, error) {
	params := url.Values{}
	params.Set("content", query.Content)
	for _, chID := range query.ChannelIDs {
		params.Add("channel_id", chID)
	}
	for _, authorID := range query.AuthorIDs {
		params.Add("author_id", authorID)
	}
	if query.Limit > 0 {
		limit := int(math.Min(math.Max(float64(query.Limit), 1), 25))
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := fmt.Sprintf("/guilds/%s/messages/search?%s", query.GuildID, params.Encode())
	return discordGET(ctx, path, token)
}
