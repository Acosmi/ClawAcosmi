package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

// Discord messaging action handler — 继承自 src/agents/tools/discord-actions-messaging.ts (451L)

var discordMessageLinkRegex = regexp.MustCompile(`https?://(?:ptb\.|canary\.)?discord\.com/channels/(\d+)/(\d+)/(\d+)`)

// ParseDiscordMessageLink 解析 Discord 消息链接
func ParseDiscordMessageLink(link string) (guildID, channelID, messageID string, ok bool) {
	m := discordMessageLinkRegex.FindStringSubmatch(link)
	if len(m) < 4 {
		return "", "", "", false
	}
	return m[1], m[2], m[3], true
}

func handleDiscordMessagingAction(ctx context.Context, action string, params map[string]interface{}, actionGate ActionGate, deps DiscordActionDeps) (ToolResult, error) {
	token := ResolveDiscordToken(params)

	switch action {
	case "sendMessage":
		if !actionGate("sendMessage") {
			return ToolResult{}, fmt.Errorf("Discord sendMessage is disabled")
		}
		to, err := ReadStringParam(params, "to", true)
		if err != nil {
			return ErrorResult(err), err
		}
		content, _ := ReadStringParam(params, "content", false)
		mediaURL, _ := ReadStringParam(params, "mediaUrl", false)
		threadID, _ := ReadStringParam(params, "threadId", false)
		replyToID, _ := ReadStringParam(params, "replyToId", false)

		var embedJSON json.RawMessage
		if raw, ok := params["embed"]; ok && raw != nil {
			data, _ := json.Marshal(raw)
			embedJSON = data
		}

		msgID, chanID, err := deps.SendMessage(ctx, to, content, token, DiscordBridgeSendOpts{
			MediaURL:  mediaURL,
			EmbedJSON: embedJSON,
			ThreadID:  threadID,
			ReplyToID: replyToID,
		})
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "messageId": msgID, "channelId": chanID}), nil

	case "editMessage":
		if !actionGate("editMessage") {
			return ToolResult{}, fmt.Errorf("Discord editMessage is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageID, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		content, err := ReadStringParam(params, "content", true)
		if err != nil {
			return ErrorResult(err), err
		}
		result, err := deps.EditMessage(ctx, channelID, messageID, content, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "message": json.RawMessage(result)}), nil

	case "deleteMessage":
		if !actionGate("deleteMessage") {
			return ToolResult{}, fmt.Errorf("Discord deleteMessage is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageID, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		if err := deps.DeleteMessage(ctx, channelID, messageID, token); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "deleted": true}), nil

	case "readMessages":
		if !actionGate("readMessages") {
			return ToolResult{}, fmt.Errorf("Discord readMessages is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		var limit *int
		if v, ok := ReadIntParam(params, "limit"); ok {
			limit = &v
		}
		before, _ := ReadStringParam(params, "before", false)
		after, _ := ReadStringParam(params, "after", false)
		around, _ := ReadStringParam(params, "around", false)

		messages, err := deps.ReadMessages(ctx, channelID, token, limit, before, after, around)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "messages": messages}), nil

	case "fetchMessage":
		if !actionGate("readMessages") {
			return ToolResult{}, fmt.Errorf("Discord readMessages is disabled")
		}
		// 支持消息链接或 channelId+messageId
		link, _ := ReadStringParam(params, "link", false)
		var channelID, messageID string
		if link != "" {
			_, cid, mid, ok := ParseDiscordMessageLink(link)
			if !ok {
				return ToolResult{}, fmt.Errorf("invalid Discord message link: %s", link)
			}
			channelID, messageID = cid, mid
		} else {
			var err error
			channelID, err = ReadStringParam(params, "channelId", true)
			if err != nil {
				return ErrorResult(err), err
			}
			messageID, err = ReadStringParam(params, "messageId", true)
			if err != nil {
				return ErrorResult(err), err
			}
		}
		msg, err := deps.FetchMessage(ctx, channelID, messageID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "message": json.RawMessage(msg)}), nil

	case "searchMessages":
		if !actionGate("searchMessages") {
			return ToolResult{}, fmt.Errorf("Discord searchMessages is disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		content, err := ReadStringParam(params, "content", true)
		if err != nil {
			return ErrorResult(err), err
		}
		channelIDs := ReadStringArrayParam(params, "channelIds")
		authorIDs := ReadStringArrayParam(params, "authorIds")
		limit := 25
		if v, ok := ReadIntParam(params, "limit"); ok && v > 0 {
			limit = v
		}
		result, err := deps.SearchMessages(ctx, guildID, content, token, channelIDs, authorIDs, limit)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": json.RawMessage(result)}), nil

	case "react":
		if !actionGate("reactions") {
			return ToolResult{}, fmt.Errorf("Discord reactions are disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageID, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		reaction := ReadReactionParams(params)
		if reaction.Remove {
			if reaction.Emoji == "" {
				return ToolResult{}, fmt.Errorf("emoji is required to remove a Discord reaction")
			}
			if err := deps.RemoveReaction(ctx, channelID, messageID, reaction.Emoji, token); err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true, "removed": reaction.Emoji}), nil
		}
		if reaction.IsEmpty {
			removed, err := deps.RemoveOwnReactions(ctx, channelID, messageID, token)
			if err != nil {
				return ErrorResult(err), err
			}
			return OkResult(map[string]interface{}{"ok": true, "removed": removed}), nil
		}
		if err := deps.ReactMessage(ctx, channelID, messageID, reaction.Emoji, token); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "added": reaction.Emoji}), nil

	case "reactions":
		if !actionGate("reactions") {
			return ToolResult{}, fmt.Errorf("Discord reactions are disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageID, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		limit := 100
		if v, ok := ReadIntParam(params, "limit"); ok && v > 0 {
			limit = v
		}
		reactions, err := deps.FetchReactions(ctx, channelID, messageID, token, limit)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "reactions": reactions}), nil

	case "sendSticker":
		if !actionGate("sticker") {
			return ToolResult{}, fmt.Errorf("Discord sticker is disabled")
		}
		to, err := ReadStringParam(params, "to", true)
		if err != nil {
			return ErrorResult(err), err
		}
		stickerIDs := ReadStringArrayParam(params, "stickerIds")
		if len(stickerIDs) == 0 {
			return ToolResult{}, fmt.Errorf("stickerIds is required")
		}
		content, _ := ReadStringParam(params, "content", false)
		msgID, chanID, err := deps.SendSticker(ctx, to, stickerIDs, token, content)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "messageId": msgID, "channelId": chanID}), nil

	case "sendPoll":
		if !actionGate("poll") {
			return ToolResult{}, fmt.Errorf("Discord poll is disabled")
		}
		to, err := ReadStringParam(params, "to", true)
		if err != nil {
			return ErrorResult(err), err
		}
		pollRaw, ok := params["poll"]
		if !ok || pollRaw == nil {
			return ToolResult{}, fmt.Errorf("poll object is required")
		}
		content, _ := ReadStringParam(params, "content", false)
		msgID, chanID, err := deps.SendPoll(ctx, to, pollRaw, token, content)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "messageId": msgID, "channelId": chanID}), nil

	case "channelPermissions":
		if !actionGate("channelPermissions") {
			return ToolResult{}, fmt.Errorf("Discord channelPermissions is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		perms, err := deps.FetchPermissions(ctx, channelID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "permissions": perms}), nil

	case "pinMessage":
		if !actionGate("pin") {
			return ToolResult{}, fmt.Errorf("Discord pin is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageID, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		if err := deps.PinMessage(ctx, channelID, messageID, token); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "unpinMessage":
		if !actionGate("pin") {
			return ToolResult{}, fmt.Errorf("Discord pin is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		messageID, err := ReadStringParam(params, "messageId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		if err := deps.UnpinMessage(ctx, channelID, messageID, token); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "listPins":
		if !actionGate("pin") {
			return ToolResult{}, fmt.Errorf("Discord pin is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		pins, err := deps.ListPins(ctx, channelID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "pins": pins}), nil

	case "createThread":
		if !actionGate("thread") {
			return ToolResult{}, fmt.Errorf("Discord thread is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		name, err := ReadStringParam(params, "name", true)
		if err != nil {
			return ErrorResult(err), err
		}
		msgID, _ := ReadStringParam(params, "messageId", false)
		autoArchive := 0
		if v, ok := ReadIntParam(params, "autoArchiveMinutes"); ok {
			autoArchive = v
		}
		content, _ := ReadStringParam(params, "content", false)

		result, err := deps.CreateThread(ctx, channelID, DiscordBridgeThreadCreate{
			MessageID:          msgID,
			Name:               name,
			AutoArchiveMinutes: autoArchive,
			Content:            content,
		}, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "thread": json.RawMessage(result)}), nil

	case "listThreads":
		if !actionGate("thread") {
			return ToolResult{}, fmt.Errorf("Discord thread is disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		channelID, _ := ReadStringParam(params, "channelId", false)
		includeArchived := ReadBoolParam(params, "includeArchived", false)
		before, _ := ReadStringParam(params, "before", false)
		limit := 0
		if v, ok := ReadIntParam(params, "limit"); ok {
			limit = v
		}

		result, err := deps.ListThreads(ctx, guildID, channelID, token, includeArchived, before, limit)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "result": json.RawMessage(result)}), nil

	case "parseMessageLink":
		link, err := ReadStringParam(params, "link", true)
		if err != nil {
			return ErrorResult(err), err
		}
		guildID, channelID, messageID, ok := ParseDiscordMessageLink(link)
		if !ok {
			return ToolResult{}, fmt.Errorf("invalid Discord message link")
		}
		return OkResult(map[string]interface{}{
			"ok": true, "guildId": guildID, "channelId": channelID, "messageId": messageID,
		}), nil

	default:
		return ToolResult{}, fmt.Errorf("unknown discord messaging action: %s", action)
	}
}
