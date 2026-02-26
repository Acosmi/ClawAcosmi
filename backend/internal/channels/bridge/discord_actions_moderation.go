package bridge

import (
	"context"
	"fmt"
)

// Discord moderation action handler — 继承自 src/agents/tools/discord-actions-moderation.ts (106L)

func handleDiscordModerationAction(ctx context.Context, action string, params map[string]interface{}, actionGate ActionGate, deps DiscordActionDeps) (ToolResult, error) {
	token := ResolveDiscordToken(params)

	if !actionGate("moderation") {
		return ToolResult{}, fmt.Errorf("Discord moderation is disabled")
	}

	switch action {
	case "timeout":
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		userID, err := ReadStringParam(params, "userId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		durationMinutes := 0
		if v, ok := ReadIntParam(params, "durationMinutes"); ok {
			durationMinutes = v
		}
		until, _ := ReadStringParam(params, "until", false)
		reason, _ := ReadStringParam(params, "reason", false)

		member, err := deps.TimeoutMember(ctx, guildID, userID, token, durationMinutes, until, reason)
		if err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true, "member": member}), nil

	case "kick":
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		userID, err := ReadStringParam(params, "userId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		reason, _ := ReadStringParam(params, "reason", false)

		if err := deps.KickMember(ctx, guildID, userID, token, reason); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	case "ban":
		guildID, err := ReadStringParam(params, "guildId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		userID, err := ReadStringParam(params, "userId", true)
		if err != nil {
			return ErrorResult(err), err
		}
		reason, _ := ReadStringParam(params, "reason", false)
		deleteMessageDays := 0
		if v, ok := ReadIntParam(params, "deleteMessageDays"); ok {
			deleteMessageDays = v
		}

		if err := deps.BanMember(ctx, guildID, userID, token, reason, deleteMessageDays); err != nil {
			return ErrorResult(err), err
		}
		return OkResult(map[string]interface{}{"ok": true}), nil

	default:
		return ToolResult{}, fmt.Errorf("unknown discord moderation action: %s", action)
	}
}
