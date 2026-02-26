package autoreply

import "strings"

// TS 对照: auto-reply/reply/commands-context.ts (45L)

// BuildCommandContextParams buildCommandContext 参数。
type BuildCommandContextParams struct {
	Ctx                   *MsgContext
	AgentID               string
	SessionKey            string
	IsGroup               bool
	TriggerBodyNormalized string
	CommandAuthorized     bool
	OwnerList             []string
	MentionPatterns       []string // 提及正则模式（用于 StripMentions）
}

// BuildCommandContext 从 MsgContext + 授权信息构建 CommandContext。
// TS 对照: commands-context.ts buildCommandContext
func BuildCommandContext(params *BuildCommandContextParams) *CommandContext {
	ctx := params.Ctx
	if ctx == nil {
		return &CommandContext{}
	}

	// 解析授权
	auth := ResolveCommandAuthorization(&CommandAuthParams{
		SenderID:    ctx.SenderID,
		ChannelType: ctx.ChannelType,
		IsGroup:     params.IsGroup,
		IsBotOwner:  params.CommandAuthorized,
	})

	// 判断 senderIsOwner
	senderIsOwner := false
	senderID := ctx.SenderID
	for _, owner := range params.OwnerList {
		if strings.EqualFold(owner, senderID) {
			senderIsOwner = true
			break
		}
	}

	// 构建 rawBodyNormalized
	rawBodyNormalized := strings.TrimSpace(params.TriggerBodyNormalized)

	// 构建 commandBodyNormalized
	// 群组模式下先去除提及信息
	bodyForCommand := rawBodyNormalized
	if params.IsGroup && len(params.MentionPatterns) > 0 {
		bodyForCommand = StripMentionsFromBody(bodyForCommand, params.MentionPatterns)
	}
	commandBodyNormalized := NormalizeCommandBody(bodyForCommand, nil)

	return &CommandContext{
		Surface:               ctx.Surface,
		Channel:               ctx.ChannelType,
		ChannelID:             ctx.ChannelID,
		OwnerList:             params.OwnerList,
		SenderIsOwner:         senderIsOwner,
		IsAuthorizedSender:    auth.Authorized,
		SenderID:              senderID,
		AbortKey:              params.SessionKey,
		RawBodyNormalized:     rawBodyNormalized,
		CommandBodyNormalized: commandBodyNormalized,
		From:                  ctx.From,
		To:                    ctx.To,
	}
}

// StripMentionsFromBody 从消息体中移除提及信息。
// 封装 reply.StripMentions，避免跨包引用。
// TS 对照: mentions.ts stripMentions
func StripMentionsFromBody(body string, mentionPatterns []string) string {
	if len(mentionPatterns) == 0 {
		return body
	}
	result := body
	for _, pattern := range mentionPatterns {
		result = strings.ReplaceAll(result, pattern, " ")
	}
	// 合并空白
	fields := strings.Fields(result)
	return strings.Join(fields, " ")
}
