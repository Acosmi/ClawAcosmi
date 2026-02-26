package bridge

import (
	"context"
	"encoding/json"
	"fmt"
)

// Discord guild action handler — 继承自 src/agents/tools/discord-actions-guild.ts (508L)

func handleDiscordGuildAction(ctx context.Context, action string, params map[string]interface{}, actionGate ActionGate, deps DiscordActionDeps) (ToolResult, error) {
	token := ResolveDiscordToken(params)

	switch action {
	case "memberInfo":
		if !actionGate("guild") {
			return ToolResult{}, fmt.Errorf("Discord guild actions are disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		userID, err := ReadStringParam(params, "userId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		info, err := deps.FetchMemberInfo(ctx, guildID, userID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "member": info}), nil

	case "roleInfo":
		if !actionGate("guild") {
			return ToolResult{}, fmt.Errorf("Discord guild actions are disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		info, err := deps.FetchRoleInfo(ctx, guildID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "roles": info}), nil

	case "addRole":
		if !actionGate("guild") {
			return ToolResult{}, fmt.Errorf("Discord guild actions are disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		userID, err := ReadStringParam(params, "userId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		roleID, err := ReadStringParam(params, "roleId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		if err := deps.AddRole(ctx, guildID, userID, roleID, token); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "removeRole":
		if !actionGate("guild") {
			return ToolResult{}, fmt.Errorf("Discord guild actions are disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		userID, err := ReadStringParam(params, "userId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		roleID, err := ReadStringParam(params, "roleId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		if err := deps.RemoveRole(ctx, guildID, userID, roleID, token); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "channelInfo":
		if !actionGate("guild") {
			return ToolResult{}, fmt.Errorf("Discord guild actions are disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		info, err := deps.FetchChannelInfo(ctx, channelID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "channel": json.RawMessage(info)}), nil

	case "listChannels":
		if !actionGate("guild") {
			return ToolResult{}, fmt.Errorf("Discord guild actions are disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		channels, err := deps.ListChannels(ctx, guildID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "channels": channels}), nil

	case "voiceStatus":
		if !actionGate("guild") {
			return ToolResult{}, fmt.Errorf("Discord guild actions are disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		status, err := deps.FetchVoiceStatus(ctx, guildID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "voiceStates": status}), nil

	case "scheduledEvents":
		if !actionGate("guild") {
			return ToolResult{}, fmt.Errorf("Discord guild actions are disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		events, err := deps.ListScheduledEvents(ctx, guildID, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "events": events}), nil

	case "createChannel":
		if !actionGate("channels") {
			return ToolResult{}, fmt.Errorf("Discord channel management is disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		name, err := ReadStringParam(params, "name", true)
		if err != nil {
			return ErrorResult(err), err
		}
		var chType *int
		if v, ok := ReadIntParam(params, "type"); ok {
			chType = &v
		}
		parentID, _ := ReadStringParam(params, "parentId", false)
		topic, _ := ReadStringParam(params, "topic", false)

		result, err := deps.CreateChannel(ctx, guildID, name, token, chType, parentID, topic)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "channel": json.RawMessage(result)}), nil

	case "editChannel":
		if !actionGate("channels") {
			return ToolResult{}, fmt.Errorf("Discord channel management is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		edits := map[string]interface{}{}
		if v, _ := ReadStringParam(params, "name", false); v != "" {
			edits["name"] = v
		}
		if v, ok := params["topic"]; ok {
			edits["topic"] = v
		}
		if v, ok := ReadIntParam(params, "position"); ok {
			edits["position"] = v
		}
		if v, ok := params["parentId"]; ok {
			edits["parent_id"] = v
		}
		if v, ok := params["nsfw"]; ok {
			edits["nsfw"] = v
		}
		if v, ok := ReadIntParam(params, "rateLimitPerUser"); ok {
			edits["rate_limit_per_user"] = v
		}
		result, err := deps.EditChannel(ctx, channelID, token, edits)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "channel": json.RawMessage(result)}), nil

	case "deleteChannel":
		if !actionGate("channels") {
			return ToolResult{}, fmt.Errorf("Discord channel management is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		if err := deps.DeleteChannel(ctx, channelID, token); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "deleted": true}), nil

	case "moveChannel":
		if !actionGate("channels") {
			return ToolResult{}, fmt.Errorf("Discord channel management is disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		var parentID *string
		if v, _ := ReadStringParam(params, "parentId", false); v != "" {
			parentID = &v
		}
		var position *int
		if v, ok := ReadIntParam(params, "position"); ok {
			position = &v
		}
		if err := deps.MoveChannel(ctx, guildID, channelID, token, parentID, position); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "setChannelPermission":
		if !actionGate("channels") {
			return ToolResult{}, fmt.Errorf("Discord channel management is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		targetID, err := ReadStringParam(params, "targetId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		targetType := 0 // 0=role, 1=member
		if v, ok := ReadIntParam(params, "targetType"); ok {
			targetType = v
		}
		allow, _ := ReadStringParam(params, "allow", false)
		deny, _ := ReadStringParam(params, "deny", false)

		if err := deps.SetChannelPermission(ctx, channelID, targetID, token, targetType, allow, deny); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "removeChannelPermission":
		if !actionGate("channels") {
			return ToolResult{}, fmt.Errorf("Discord channel management is disabled")
		}
		channelID, err := ReadStringParam(params, "channelId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		targetID, err := ReadStringParam(params, "targetId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		if err := deps.RemoveChannelPermission(ctx, channelID, targetID, token); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "uploadEmoji":
		if !actionGate("emoji") {
			return ToolResult{}, fmt.Errorf("Discord emoji upload is disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		name, err := ReadStringParam(params, "name", true)
		if err != nil {
			return ErrorResult(err), err
		}
		mediaURL, err := ReadStringParam(params, "mediaUrl", true)
		if err != nil {
			return ErrorResult(err), err
		}
		roleIDs := ReadStringArrayParam(params, "roleIds")
		result, err := deps.UploadEmoji(ctx, guildID, name, mediaURL, token, roleIDs)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "emoji": json.RawMessage(result)}), nil

	case "uploadSticker":
		if !actionGate("sticker") {
			return ToolResult{}, fmt.Errorf("Discord sticker upload is disabled")
		}
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		name, err := ReadStringParam(params, "name", true)
		if err != nil {
			return ErrorResult(err), err
		}
		desc, _ := ReadStringParam(params, "description", false)
		tags, err := ReadStringParam(params, "tags", true)
		if err != nil {
			return ErrorResult(err), err
		}
		mediaURL, err := ReadStringParam(params, "mediaUrl", true)
		if err != nil {
			return ErrorResult(err), err
		}
		result, err := deps.UploadSticker(ctx, guildID, name, desc, tags, mediaURL, token)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "sticker": json.RawMessage(result)}), nil

	default:
		return ToolResult{}, fmt.Errorf("unknown discord guild action: %s", action)
	}
}
