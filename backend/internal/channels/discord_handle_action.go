package channels

import (
	"context"
	"encoding/json"
	"fmt"
)

// Discord 消息动作处理器 — 继承自 src/channels/plugins/actions/discord/handle-action.ts (249L)

// DiscordActionType 所有 Discord 消息动作类型
type DiscordActionType string

const (
	DiscordActionSend         DiscordActionType = "send"
	DiscordActionPoll         DiscordActionType = "poll"
	DiscordActionReact        DiscordActionType = "react"
	DiscordActionReactions    DiscordActionType = "reactions"
	DiscordActionRead         DiscordActionType = "read"
	DiscordActionEdit         DiscordActionType = "edit"
	DiscordActionDelete       DiscordActionType = "delete"
	DiscordActionPin          DiscordActionType = "pin"
	DiscordActionUnpin        DiscordActionType = "unpin"
	DiscordActionListPins     DiscordActionType = "list-pins"
	DiscordActionPermissions  DiscordActionType = "permissions"
	DiscordActionThreadCreate DiscordActionType = "thread-create"
	DiscordActionSticker      DiscordActionType = "sticker"
	DiscordActionSetPresence  DiscordActionType = "set-presence"
)

// DiscordActionConfig 运行时配置，用于传递 token / 重试策略等信息
type DiscordActionConfig struct {
	Token          string      `json:"-"` // 不序列化
	MaxLinesPerMsg int         `json:"-"`
	RetryConfig    interface{} `json:"-"`
}

// DiscordActionRequest 统一的 Discord 动作请求
type DiscordActionRequest struct {
	Action    string                 `json:"action"`
	AccountID string                 `json:"accountId,omitempty"`
	To        string                 `json:"to,omitempty"`
	ChannelID string                 `json:"channelId,omitempty"`
	MessageID string                 `json:"messageId,omitempty"`
	Content   string                 `json:"content,omitempty"`
	MediaURL  string                 `json:"mediaUrl,omitempty"`
	ReplyTo   string                 `json:"replyTo,omitempty"`
	Embeds    []interface{}          `json:"embeds,omitempty"`
	Extra     map[string]interface{} `json:"extra,omitempty"`
	Config    *DiscordActionConfig   `json:"-"` // 运行时配置，不序列化
}

// ReadParentIDParam 读取 parentId 参数，支持 clearParent 标志
func ReadParentIDParam(params map[string]interface{}) *string {
	if clearParent, ok := params["clearParent"].(bool); ok && clearParent {
		empty := ""
		return &empty
	}
	if params["parentId"] == nil {
		return nil
	}
	if v, ok := params["parentId"].(string); ok {
		return &v
	}
	return nil
}

// BuildDiscordActionRequest 从消息动作上下文构建 Discord 动作请求
func BuildDiscordActionRequest(ctx context.Context, action string, params map[string]interface{}, accountID string, cfg *DiscordActionConfig) (*DiscordActionRequest, error) {
	req := &DiscordActionRequest{
		AccountID: accountID,
	}
	if cfg != nil {
		req.Config = cfg
	}

	switch DiscordActionType(action) {
	case DiscordActionSend:
		req.Action = "sendMessage"
		to := ReadStringParam(params, "to")
		if to == "" {
			return nil, fmt.Errorf("parameter \"to\" is required")
		}
		req.To = to
		req.Content = ReadStringParam(params, "message")
		req.MediaURL = ReadStringParam(params, "media")
		req.ReplyTo = ReadStringParam(params, "replyTo")
		if embeds, ok := params["embeds"].([]interface{}); ok {
			req.Embeds = embeds
		}
	case DiscordActionPoll:
		req.Action = "poll"
		req.To = ReadStringParam(params, "to")
		req.Extra = map[string]interface{}{
			"question": ReadStringParam(params, "pollQuestion"),
			"answers":  ReadStringArrayParam(params, "pollOption"),
		}
		if multi, ok := params["pollMulti"].(bool); ok {
			req.Extra["allowMultiselect"] = multi
		}
		if hours := ReadIntParam(params, "pollDurationHours"); hours != nil {
			req.Extra["durationHours"] = *hours
		}
		req.Content = ReadStringParam(params, "message")
	case DiscordActionReact:
		req.Action = "react"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
		req.MessageID = ReadStringParam(params, "messageId")
		// emoji 允许空值，直接设置不经过 mergeExtra 过滤
		req.Extra = make(map[string]interface{})
		if emojiRaw, exists := params["emoji"]; exists {
			req.Extra["emoji"] = fmt.Sprintf("%v", emojiRaw)
		}
		if remove, ok := params["remove"].(bool); ok {
			req.Extra["remove"] = remove
		}
	case DiscordActionReactions:
		req.Action = "reactions"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
		req.MessageID = ReadStringParam(params, "messageId")
		if limit := ReadIntParam(params, "limit"); limit != nil {
			req.Extra = map[string]interface{}{"limit": *limit}
		}
	case DiscordActionRead:
		req.Action = "readMessages"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
		if limit := ReadIntParam(params, "limit"); limit != nil {
			req.Extra = map[string]interface{}{"limit": *limit}
		}
		req.Extra = mergeExtra(req.Extra, map[string]interface{}{
			"before": ReadStringParam(params, "before"),
			"after":  ReadStringParam(params, "after"),
			"around": ReadStringParam(params, "around"),
		})
	case DiscordActionEdit:
		req.Action = "editMessage"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
		req.MessageID = ReadStringParam(params, "messageId")
		req.Content = ReadStringParam(params, "message")
	case DiscordActionDelete:
		req.Action = "deleteMessage"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
		req.MessageID = ReadStringParam(params, "messageId")
	case DiscordActionPin:
		req.Action = "pinMessage"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
		req.MessageID = ReadStringParam(params, "messageId")
	case DiscordActionUnpin:
		req.Action = "unpinMessage"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
		req.MessageID = ReadStringParam(params, "messageId")
	case DiscordActionListPins:
		req.Action = "listPins"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
	case DiscordActionPermissions:
		req.Action = "permissions"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
	case DiscordActionThreadCreate:
		req.Action = "threadCreate"
		chID := ReadStringParam(params, "channelId")
		if chID == "" {
			chID = ReadStringParam(params, "to")
		}
		if chID == "" {
			return nil, fmt.Errorf("channelId or to is required")
		}
		req.ChannelID = chID
		req.Extra = map[string]interface{}{
			"name":      ReadStringParam(params, "threadName"),
			"messageId": ReadStringParam(params, "messageId"),
		}
		req.Content = ReadStringParam(params, "message")
		if archiveMin := ReadIntParam(params, "autoArchiveMin"); archiveMin != nil {
			req.Extra["autoArchiveMinutes"] = *archiveMin
		}
	case DiscordActionSticker:
		req.Action = "sticker"
		req.To = ReadStringParam(params, "to")
		req.Extra = map[string]interface{}{
			"stickerIds": ReadStringArrayParam(params, "stickerId"),
		}
		req.Content = ReadStringParam(params, "message")
	case DiscordActionSetPresence:
		req.Action = "setPresence"
		req.Extra = map[string]interface{}{
			"status":        ReadStringParam(params, "status"),
			"activityType":  ReadStringParam(params, "activityType"),
			"activityName":  ReadStringParam(params, "activityName"),
			"activityUrl":   ReadStringParam(params, "activityUrl"),
			"activityState": ReadStringParam(params, "activityState"),
		}
	default:
		// 两阶段分派：fallthrough 到 guild admin
		if IsDiscordGuildAdminAction(action) {
			return BuildDiscordGuildAdminRequest(ctx, action, params, accountID)
		}
		return nil, fmt.Errorf("unsupported discord action: %s", action)
	}

	return req, nil
}

// MarshalJSON 自定义序列化：将 Extra 字段展平到顶层，与 TS 版扁平 payload 对齐
func (r *DiscordActionRequest) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	m["action"] = r.Action
	if r.AccountID != "" {
		m["accountId"] = r.AccountID
	}
	if r.To != "" {
		m["to"] = r.To
	}
	if r.ChannelID != "" {
		m["channelId"] = r.ChannelID
	}
	if r.MessageID != "" {
		m["messageId"] = r.MessageID
	}
	if r.Content != "" {
		m["content"] = r.Content
	}
	if r.MediaURL != "" {
		m["mediaUrl"] = r.MediaURL
	}
	if r.ReplyTo != "" {
		m["replyTo"] = r.ReplyTo
	}
	if len(r.Embeds) > 0 {
		m["embeds"] = r.Embeds
	}
	for k, v := range r.Extra {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return json.Marshal(m)
}

// mergeExtra 合并 extra map
func mergeExtra(base, add map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{})
	}
	for k, v := range add {
		if v != nil && v != "" {
			base[k] = v
		}
	}
	return base
}
