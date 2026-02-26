package autoreply

import "context"

// TS 对照: auto-reply/reply/commands-plugin.ts (53L)

// HandlePluginCommand 插件命令处理器。
// 在内置命令之前执行，给插件优先权。
// TS 对照: commands-plugin.ts handlePluginCommand
func HandlePluginCommand(ctx context.Context, params *HandleCommandsParams, allowTextCommands bool) (*CommandHandlerResult, error) {
	if !allowTextCommands {
		return nil, nil
	}

	// 需要 PluginMatcher DI
	if params.PluginMatcher == nil {
		return nil, nil
	}

	match := params.PluginMatcher.MatchPluginCommand(params.Command.CommandBodyNormalized)
	if match == nil {
		return nil, nil
	}

	reply, err := params.PluginMatcher.ExecutePluginCommand(ctx, match, map[string]any{
		"channel":    params.Command.Channel,
		"surface":    params.Command.Surface,
		"senderId":   params.Command.SenderID,
		"sessionKey": params.SessionKey,
		"agentId":    params.AgentID,
	})
	if err != nil {
		return &CommandHandlerResult{
			ShouldContinue: false,
			Reply:          &ReplyPayload{Text: "❌ Plugin error: " + err.Error()},
		}, nil
	}

	return &CommandHandlerResult{
		ShouldContinue: false,
		Reply:          reply,
	}, nil
}
