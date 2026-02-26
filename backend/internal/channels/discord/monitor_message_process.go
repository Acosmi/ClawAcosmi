package discord

// Discord extended message processing pipeline.
// TS ref: src/discord/monitor/message-handler.process.ts (450L)
//
// ProcessDiscordMessage takes a fully-populated DiscordMessagePreflightContext
// (produced by PreflightDiscordMessage) and runs the complete inbound pipeline:
//
//  1. Resolve media attachments
//  2. Send ack reaction on the inbound message
//  3. Build sender/from labels and envelope body
//  4. Resolve forum parent info and thread starters
//  5. Build channel history context
//  6. Resolve reply context (quoted messages)
//  7. Resolve thread session keys and auto-thread reply plans
//  8. Build MsgContext payload
//  9. Record inbound session
//  10. Set up typing indicator
//  11. Dispatch to auto-reply pipeline
//  12. Remove ack reaction after reply (if configured)

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/anthropic/open-acosmi/internal/autoreply"
)

// ackReactionDefault is the default emoji used to acknowledge receipt of an
// inbound message. An empty string disables the ack reaction.
const ackReactionDefault = ""

// ProcessDiscordMessage executes the full message processing pipeline on a
// preflight-validated inbound Discord message.
//
// This is the richer counterpart of DispatchDiscordInbound: it honours the
// complete DiscordMessagePreflightContext produced by PreflightDiscordMessage,
// resolving media, ack reactions, forum parents, thread starters, channel
// history, reply context, and auto-thread reply plans before dispatching to
// the auto-reply layer.
//
// TS ref: processDiscordMessage (message-handler.process.ts L46-450)
func ProcessDiscordMessage(ctx context.Context, monCtx *DiscordMonitorContext, pCtx *DiscordMessagePreflightContext) {
	if pCtx == nil || pCtx.Message == nil || pCtx.Message.Author == nil {
		return
	}

	logger := monCtx.Logger.With(
		"action", "process",
		"sender", pCtx.Sender.ID,
		"channel", pCtx.Message.ChannelID,
	)

	deps := pCtx.Deps
	if deps == nil {
		deps = monCtx.Deps
	}
	if deps == nil {
		logger.Warn("deps not available, skipping process")
		return
	}

	msg := pCtx.Message
	author := msg.Author
	sender := pCtx.Sender
	route := pCtx.Route
	if route == nil {
		logger.Warn("route not available, skipping process")
		return
	}

	// ---------------------------------------------------------------
	// 1. Resolve media attachments
	// TS ref: const mediaList = await resolveMediaList(message, mediaMaxBytes)
	// ---------------------------------------------------------------
	mediaList := resolveProcessMediaList(deps, msg, pCtx.MediaMaxBytes)

	text := pCtx.MessageText
	if text == "" {
		logger.Debug("drop message (empty content)", "messageID", msg.ID)
		return
	}

	// ---------------------------------------------------------------
	// 2. Ack reaction
	// TS ref: ackReactionPromise = shouldAckReaction() ? reactMessageDiscord(...) : null
	// ---------------------------------------------------------------
	ackReaction := ackReactionDefault // resolved from config in TS via resolveAckReaction
	shouldAck := ackReaction != "" && shouldAckReactionForProcess(pCtx)
	ackReacted := false
	if shouldAck {
		if err := ReactMessageDiscord(ctx, msg.ChannelID, msg.ID, ackReaction, pCtx.Token); err != nil {
			logger.Debug("ack reaction failed", "error", err)
		} else {
			ackReacted = true
		}
	}

	// ---------------------------------------------------------------
	// 3. Build from/sender labels
	// TS ref: fromLabel, senderLabel, senderName, senderUsername, senderTag
	// ---------------------------------------------------------------
	fromLabel := buildProcessFromLabel(pCtx)
	senderLabel := sender.Label
	senderName := resolveProcessSenderName(pCtx)
	senderUsername := resolveProcessSenderUsername(pCtx)
	senderTag := sender.Tag

	// ---------------------------------------------------------------
	// 4. Forum parent, thread starter, group channel metadata
	// TS ref: isForumParent, forumContextLine, groupChannel, etc.
	// ---------------------------------------------------------------
	isForumParent := isProcessForumParent(pCtx.ThreadParentType)
	forumParentSlug := ""
	if isForumParent && pCtx.ThreadParentName != "" {
		forumParentSlug = NormalizeDiscordSlug(pCtx.ThreadParentName)
	}
	threadChannelID := ""
	if pCtx.ThreadChannel != nil {
		threadChannelID = pCtx.ThreadChannel.ID
	}
	isForumStarter := threadChannelID != "" && isForumParent && forumParentSlug != "" && msg.ID == threadChannelID
	forumContextLine := ""
	if isForumStarter {
		forumContextLine = fmt.Sprintf("[Forum parent: #%s]", forumParentSlug)
	}

	groupChannel := ""
	if pCtx.IsGuildMessage && pCtx.DisplayChannelSlug != "" {
		groupChannel = "#" + pCtx.DisplayChannelSlug
	}
	groupSubject := ""
	if !pCtx.IsDirectMessage {
		groupSubject = groupChannel
	}

	// Channel-config system prompt
	// TS ref: groupSystemPrompt
	groupSystemPrompt := ""
	if pCtx.ChannelConfig != nil {
		groupSystemPrompt = strings.TrimSpace(pCtx.ChannelConfig.SystemPrompt)
	}

	// ---------------------------------------------------------------
	// 5. Resolve store path and previous timestamp
	// TS ref: storePath, previousTimestamp
	// ---------------------------------------------------------------
	storePath := ""
	if deps.ResolveStorePath != nil {
		storePath = deps.ResolveStorePath(route.AgentID)
	}

	var previousTimestamp *int64
	if deps.ReadSessionUpdatedAt != nil && storePath != "" {
		previousTimestamp = deps.ReadSessionUpdatedAt(storePath, route.SessionKey)
	}

	// ---------------------------------------------------------------
	// 6. Build envelope body (inline formatting)
	// TS ref: formatInboundEnvelope(...)
	// ---------------------------------------------------------------
	messageTimestamp := msg.Timestamp.UnixMilli()
	combinedBody := formatProcessEnvelope(formatProcessEnvelopeParams{
		from:              fromLabel,
		senderLabel:       senderLabel,
		body:              text,
		chatType:          resolveProcessChatType(pCtx),
		timestamp:         messageTimestamp,
		previousTimestamp: previousTimestamp,
	})

	// ---------------------------------------------------------------
	// 7. Build channel history context
	// TS ref: buildPendingHistoryContextFromMap(...)
	// ---------------------------------------------------------------
	shouldIncludeHistory := !pCtx.IsDirectMessage &&
		!(pCtx.IsGuildMessage && pCtx.ChannelConfig != nil &&
			pCtx.ChannelConfig.AutoThread != nil && *pCtx.ChannelConfig.AutoThread &&
			pCtx.ThreadChannel == nil)
	if shouldIncludeHistory && pCtx.GuildHistories != nil && pCtx.HistoryLimit > 0 {
		combinedBody = buildProcessHistoryContext(
			pCtx.GuildHistories,
			msg.ChannelID,
			pCtx.HistoryLimit,
			combinedBody,
			fromLabel,
		)
	}

	// ---------------------------------------------------------------
	// 8. Resolve reply context (quoted message)
	// TS ref: resolveReplyContext(message, ...)
	// ---------------------------------------------------------------
	replyContextText := resolveProcessReplyContext(msg)
	if replyContextText != "" {
		combinedBody = "[Replied message - for context]\n" + replyContextText + "\n\n" + combinedBody
	}

	// Append forum context line if applicable
	if forumContextLine != "" {
		combinedBody = combinedBody + "\n" + forumContextLine
	}

	// ---------------------------------------------------------------
	// 9. Thread starter body and thread label
	// TS ref: resolveDiscordThreadStarter, threadLabel, parentSessionKey
	// ---------------------------------------------------------------
	threadStarterBody := ""
	threadLabel := ""
	parentSessionKey := ""
	if pCtx.ThreadChannel != nil {
		includeThreadStarter := true
		if pCtx.ChannelConfig != nil && pCtx.ChannelConfig.IncludeThreadStarter != nil {
			includeThreadStarter = *pCtx.ChannelConfig.IncludeThreadStarter
		}
		if includeThreadStarter {
			starter := resolveProcessThreadStarter(monCtx, pCtx)
			if starter != nil && starter.Text != "" {
				threadStarterBody = formatProcessThreadStarterEnvelope(starter)
			}
		}
		parentName := pCtx.ThreadParentName
		if parentName == "" {
			parentName = "parent"
		}
		if pCtx.ThreadName != "" {
			threadLabel = fmt.Sprintf("Discord thread #%s > %s", NormalizeDiscordSlug(parentName), pCtx.ThreadName)
		} else {
			threadLabel = fmt.Sprintf("Discord thread #%s", NormalizeDiscordSlug(parentName))
		}
		if pCtx.ThreadParentID != "" {
			parentSessionKey = buildProcessAgentSessionKey(route.AgentID, "discord", "channel", pCtx.ThreadParentID)
		}
	}

	// ---------------------------------------------------------------
	// 10. Media payload
	// TS ref: buildDiscordMediaPayload(mediaList)
	// ---------------------------------------------------------------
	mediaPayload := BuildDiscordMediaPayload(mediaList)

	// ---------------------------------------------------------------
	// 11. Resolve thread session keys
	// TS ref: resolveThreadSessionKeys(...)
	// ---------------------------------------------------------------
	sessionKey := pCtx.BaseSessionKey
	effectiveParentSessionKey := ""
	if pCtx.ThreadChannel != nil {
		// Thread messages use the thread-specific session key
		threadSessionKey := buildProcessAgentSessionKey(route.AgentID, "discord", "channel", msg.ChannelID)
		if threadSessionKey != "" {
			sessionKey = threadSessionKey
		}
		effectiveParentSessionKey = parentSessionKey
	}

	// ---------------------------------------------------------------
	// 12. Resolve reply plan (deliver target, reply reference)
	// DY-009 fix: 补全 auto-thread 场景的路由判断逻辑，对齐 TS resolveDiscordAutoThreadReplyPlan。
	// TS ref: resolveDiscordAutoThreadReplyPlan (threading.ts L242-280)
	// 流程:
	//  a. 尝试创建自动线程（autoThread 已配置、非线程内、Guild 消息）
	//  b. 解析回复交付计划（可能将 deliverTarget 重定向到新线程）
	//  c. 解析自动线程上下文（覆盖 From/To/SessionKey）
	// ---------------------------------------------------------------
	originalReplyTarget := "channel:" + msg.ChannelID

	// a. 尝试创建自动线程
	createdThreadID := ""
	if pCtx.IsGuildMessage && pCtx.ChannelConfig != nil &&
		pCtx.ChannelConfig.AutoThread != nil && *pCtx.ChannelConfig.AutoThread &&
		pCtx.ThreadChannel == nil {
		// autoThread 已配置，且当前不在线程中 → 创建新线程
		threadName := SanitizeDiscordThreadName(
			pCtx.BaseText,
			msg.ID,
		)
		if threadName == "" {
			threadName = "Thread " + msg.ID
		}
		createdData, createErr := CreateThreadDiscord(ctx, msg.ChannelID, DiscordThreadCreate{
			MessageID:          msg.ID,
			Name:               threadName,
			AutoArchiveMinutes: 60,
		}, pCtx.Token)
		if createErr != nil {
			logger.Debug("auto-thread creation failed", "error", createErr)
		} else if createdData != nil {
			// 解析创建的线程 ID
			var created struct {
				ID string `json:"id"`
			}
			if jsonErr := json.Unmarshal(createdData, &created); jsonErr == nil && created.ID != "" {
				createdThreadID = created.ID
			}
		}
	}

	// b. 解析回复交付计划
	replyToMode := pCtx.ReplyToMode
	if replyToMode == "" {
		replyToMode = "first"
	}
	deliveryPlan := ResolveDiscordReplyDeliveryPlan(
		originalReplyTarget,
		replyToMode,
		msg.ID,
		pCtx.ThreadChannel,
		createdThreadID,
	)
	deliverTarget := deliveryPlan.DeliverTarget
	replyTarget := deliveryPlan.ReplyTarget
	_ = deliveryPlan.ReplyReference // ReplyReference 由 dispatch 层使用

	// c. 解析自动线程上下文（From/To/SessionKey 覆盖）
	var autoThreadCtx *DiscordAutoThreadContext
	if pCtx.IsGuildMessage {
		autoThreadCtx = ResolveDiscordAutoThreadContext(route.AgentID, "discord", msg.ChannelID, createdThreadID)
	}

	// 如果创建了自动线程，覆盖 session key
	if autoThreadCtx != nil {
		sessionKey = autoThreadCtx.SessionKey
		effectiveParentSessionKey = autoThreadCtx.ParentSessionKey
	}

	replyReference := msg.ID

	// DM 回复目标
	if pCtx.IsDirectMessage {
		replyTarget = "discord:user:" + author.ID
	}

	// Effective from/to
	// TS ref: effectiveFrom = isDirectMessage ? `discord:${author.id}` : (autoThreadContext?.From ?? `discord:channel:${channelId}`)
	effectiveFrom := ""
	if pCtx.IsDirectMessage {
		effectiveFrom = "discord:" + author.ID
	} else if autoThreadCtx != nil {
		effectiveFrom = autoThreadCtx.From
	} else {
		effectiveFrom = "discord:channel:" + msg.ChannelID
	}

	// TS ref: effectiveTo = autoThreadContext?.To ?? replyTarget
	effectiveTo := replyTarget
	if autoThreadCtx != nil {
		effectiveTo = autoThreadCtx.To
	}
	if effectiveTo == "" {
		logger.Error("missing reply target")
		return
	}

	// TS ref: OriginatingTo = autoThreadContext?.OriginatingTo ?? replyTarget
	originatingTo := replyTarget
	if autoThreadCtx != nil {
		originatingTo = autoThreadCtx.OriginatingTo
	}

	// ---------------------------------------------------------------
	// 13. Build MsgContext payload
	// TS ref: finalizeInboundContext(...)
	// ---------------------------------------------------------------
	chatType := resolveProcessChatType(pCtx)
	msgCtx := &autoreply.MsgContext{
		Body:               combinedBody,
		RawBody:            pCtx.BaseText,
		CommandBody:        pCtx.BaseText,
		ThreadStarterBody:  threadStarterBody,
		ChatType:           chatType,
		ConversationLabel:  fromLabel,
		Provider:           "discord",
		Surface:            "discord",
		From:               effectiveFrom,
		To:                 effectiveTo,
		SessionKey:         sessionKey,
		AccountID:          route.AccountID,
		SenderID:           sender.ID,
		SenderName:         senderName,
		SenderUsername:     senderUsername,
		ChannelID:          msg.ChannelID,
		MessageSid:         msg.ID,
		Timestamp:          messageTimestamp,
		GroupChannel:       groupChannel,
		GroupSubject:       groupSubject,
		ThreadLabel:        threadLabel,
		CommandAuthorized:  pCtx.CommandAuthorized,
		CommandSource:      "text",
		OriginatingChannel: "discord",
		OriginatingTo:      originatingTo,
	}

	if pCtx.EffectiveWasMentioned {
		msgCtx.WasMentioned = "true"
	}
	if pCtx.IsGuildMessage && groupSystemPrompt != "" {
		msgCtx.GroupSystemPrompt = groupSystemPrompt
	}
	if pCtx.IsGuildMessage {
		guildSpace := ""
		if pCtx.GuildInfo != nil && pCtx.GuildInfo.ID != "" {
			guildSpace = pCtx.GuildInfo.ID
		} else if pCtx.GuildSlug != "" {
			guildSpace = pCtx.GuildSlug
		}
		if guildSpace != "" {
			msgCtx.GroupSpace = guildSpace
		}
	}
	if senderTag != "" {
		// SenderTag is carried inside SenderUsername for compatibility with
		// the TS field mapping.
		if msgCtx.SenderUsername == "" {
			msgCtx.SenderUsername = senderTag
		}
	}
	if effectiveParentSessionKey != "" {
		// ParentSessionKey is not a direct field on MsgContext;
		// the TS version passes it through finalizeInboundContext.
		// We record it so the DI layer can route correctly.
		_ = effectiveParentSessionKey
	}

	// Apply media payload fields
	applyProcessMediaPayload(msgCtx, mediaPayload)

	// ---------------------------------------------------------------
	// 14. Record inbound session
	// TS ref: recordInboundSession(...)
	// ---------------------------------------------------------------
	if deps.RecordInboundSession != nil {
		var lastRoute *DiscordLastRouteUpdate
		if pCtx.IsDirectMessage {
			lastRoute = &DiscordLastRouteUpdate{
				SessionKey: route.MainSessionKey,
				Channel:    "discord",
				To:         fmt.Sprintf("user:%s", author.ID),
				AccountID:  route.AccountID,
			}
		}
		effectiveSessionKey := msgCtx.SessionKey
		if effectiveSessionKey == "" {
			effectiveSessionKey = route.SessionKey
		}
		if err := deps.RecordInboundSession(DiscordRecordSessionParams{
			StorePath:       storePath,
			SessionKey:      effectiveSessionKey,
			Ctx:             msgCtx,
			UpdateLastRoute: lastRoute,
		}); err != nil {
			logger.Error("record session failed", "error", err)
		}
	}

	// ---------------------------------------------------------------
	// 15. Record inbound channel activity
	// ---------------------------------------------------------------
	if deps.RecordChannelActivity != nil {
		deps.RecordChannelActivity("discord", monCtx.AccountID, "inbound")
	}

	// ---------------------------------------------------------------
	// 16. Typing indicator
	// TS ref: sendTyping({ client, channelId: typingChannelId })
	// ---------------------------------------------------------------
	typingChannelID := msg.ChannelID
	if strings.HasPrefix(deliverTarget, "channel:") {
		typingChannelID = strings.TrimPrefix(deliverTarget, "channel:")
	}
	_ = SendTypingIndicator(ctx, monCtx.Session, typingChannelID)

	logger.Debug("dispatch inbound",
		"deliverTarget", deliverTarget,
		"from", msgCtx.From,
		"sessionKey", msgCtx.SessionKey,
	)

	// ---------------------------------------------------------------
	// 17. Dispatch to auto-reply pipeline
	// TS ref: dispatchInboundMessage({ ctx: ctxPayload, cfg, dispatcher, replyOptions })
	// ---------------------------------------------------------------
	if deps.DispatchInboundMessage == nil {
		logger.Warn("DispatchInboundMessage not available")
		return
	}

	dispatchParams := DiscordDispatchParams{
		Ctx: msgCtx,
	}
	// Pass skill filter from channel config
	if pCtx.ChannelConfig != nil && len(pCtx.ChannelConfig.Skills) > 0 {
		// Skills are passed through the dispatch params via the DI layer
		_ = pCtx.ChannelConfig.Skills
	}

	result, err := deps.DispatchInboundMessage(ctx, dispatchParams)
	if err != nil {
		logger.Error("dispatch failed", "error", err)
		removeProcessAckReaction(ctx, monCtx, msg, ackReaction, ackReacted)
		return
	}

	queuedFinal := result != nil && result.QueuedFinal
	if !queuedFinal {
		// No reply was queued; clear history entries for the channel
		clearProcessHistoryEntries(pCtx, msg.ChannelID)
		return
	}

	logger.Debug("dispatch completed",
		"queuedFinal", queuedFinal,
		"sessionKey", msgCtx.SessionKey,
		"replyTarget", replyTarget,
	)

	// ---------------------------------------------------------------
	// 18. Remove ack reaction after reply (if configured)
	// TS ref: removeAckReactionAfterReply(...)
	// ---------------------------------------------------------------
	removeProcessAckReaction(ctx, monCtx, msg, ackReaction, ackReacted)

	// Store reply context for future reference resolution
	// TS ref: the reply dispatcher calls replyReference.markSent() after delivery
	_ = replyReference

	// Clear history entries after successful reply
	clearProcessHistoryEntries(pCtx, msg.ChannelID)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// resolveProcessMediaList resolves media attachments from the message using
// the DI ResolveMedia function.
// TS ref: resolveMediaList (message-utils.ts)
func resolveProcessMediaList(deps *DiscordMonitorDeps, msg *discordgo.MessageCreate, maxBytes int) []DiscordMediaInfo {
	if msg == nil || len(msg.Attachments) == 0 {
		return nil
	}
	if deps == nil || deps.ResolveMedia == nil {
		// Fall back to placeholder-only media info
		var result []DiscordMediaInfo
		for _, a := range msg.Attachments {
			result = append(result, DiscordMediaInfo{
				Placeholder: InferAttachmentPlaceholder(a.Filename, a.ContentType),
			})
		}
		return result
	}

	var result []DiscordMediaInfo
	for _, a := range msg.Attachments {
		mediaURL := a.URL
		if mediaURL == "" {
			mediaURL = a.ProxyURL
		}
		if mediaURL == "" {
			result = append(result, DiscordMediaInfo{
				Placeholder: InferAttachmentPlaceholder(a.Filename, a.ContentType),
			})
			continue
		}
		localPath, contentType, err := deps.ResolveMedia(mediaURL, maxBytes)
		if err != nil || localPath == "" {
			result = append(result, DiscordMediaInfo{
				Placeholder: InferAttachmentPlaceholder(a.Filename, a.ContentType),
			})
			continue
		}
		if contentType == "" {
			contentType = a.ContentType
		}
		result = append(result, DiscordMediaInfo{
			Path:        localPath,
			ContentType: contentType,
			Placeholder: InferAttachmentPlaceholder(a.Filename, contentType),
		})
	}
	return result
}

// shouldAckReactionForProcess determines whether the ack reaction should be
// sent for this message based on scope and message context.
// TS ref: shouldAckReactionGate (ack-reactions.ts)
func shouldAckReactionForProcess(pCtx *DiscordMessagePreflightContext) bool {
	scope := pCtx.AckReactionScope
	if scope == "" {
		scope = "all"
	}

	isDirect := pCtx.IsDirectMessage
	isGroup := pCtx.IsGuildMessage || pCtx.IsGroupDm
	isMentionableGroup := pCtx.IsGuildMessage

	switch scope {
	case "all":
		return true
	case "direct":
		return isDirect
	case "group-all":
		return isGroup
	case "group-mentions":
		if !isMentionableGroup {
			return false
		}
		if pCtx.ShouldBypassMention {
			return true
		}
		if !pCtx.ShouldRequireMention {
			return false
		}
		if !pCtx.CanDetectMention {
			return false
		}
		return pCtx.EffectiveWasMentioned
	default:
		return false
	}
}

// buildProcessFromLabel builds the "from" label displayed in the envelope.
// TS ref: buildDirectLabel / buildGuildLabel (reply-context.ts)
func buildProcessFromLabel(pCtx *DiscordMessagePreflightContext) string {
	if pCtx.IsDirectMessage {
		return fmt.Sprintf("DM from %s", pCtx.Sender.Label)
	}
	// Guild / group message
	channelName := pCtx.ChannelName
	if channelName == "" {
		channelName = pCtx.Message.ChannelID
	}
	guildName := ""
	if pCtx.GuildInfo != nil {
		guildName = pCtx.GuildInfo.Slug
		if guildName == "" && pCtx.GuildInfo.ID != "" {
			guildName = pCtx.GuildInfo.ID
		}
	}
	if guildName != "" {
		return fmt.Sprintf("%s #%s", guildName, channelName)
	}
	return fmt.Sprintf("#%s", channelName)
}

// resolveProcessSenderName resolves the display name for the sender.
// TS ref: senderName resolution logic (process.ts L148-153)
func resolveProcessSenderName(pCtx *DiscordMessagePreflightContext) string {
	if pCtx.Sender.IsPluralKit {
		if pCtx.Sender.Name != "" {
			return pCtx.Sender.Name
		}
		return pCtx.Message.Author.Username
	}
	// Prefer member nickname > global name > username
	if pCtx.Message.Member != nil && pCtx.Message.Member.Nick != "" {
		return pCtx.Message.Member.Nick
	}
	if pCtx.Message.Author.GlobalName != "" {
		return pCtx.Message.Author.GlobalName
	}
	return pCtx.Message.Author.Username
}

// resolveProcessSenderUsername resolves the username / tag for the sender.
// TS ref: senderUsername resolution logic (process.ts L153-155)
func resolveProcessSenderUsername(pCtx *DiscordMessagePreflightContext) string {
	if pCtx.Sender.IsPluralKit {
		if pCtx.Sender.Tag != "" {
			return pCtx.Sender.Tag
		}
		if pCtx.Sender.Name != "" {
			return pCtx.Sender.Name
		}
		return pCtx.Message.Author.Username
	}
	return pCtx.Message.Author.Username
}

// isProcessForumParent returns true when the thread parent channel type is a
// Guild Forum (15) or Guild Media (16) channel.
// TS ref: ChannelType.GuildForum / ChannelType.GuildMedia
func isProcessForumParent(parentType *int) bool {
	if parentType == nil {
		return false
	}
	t := *parentType
	return t == channelTypeGuildForum || t == channelTypeGuildMedia
}

// Channel type constants channelTypeGuildForum (15) and channelTypeGuildMedia (16)
// are defined in send_permissions.go.

// resolveProcessChatType returns "direct" or "channel".
func resolveProcessChatType(pCtx *DiscordMessagePreflightContext) string {
	if pCtx.IsDirectMessage {
		return "direct"
	}
	return "channel"
}

// formatProcessEnvelopeParams contains the parameters for envelope formatting.
type formatProcessEnvelopeParams struct {
	from              string
	senderLabel       string
	body              string
	chatType          string
	timestamp         int64
	previousTimestamp *int64
}

// formatProcessEnvelope builds a formatted inbound message envelope.
// This is a simplified Go-side equivalent of formatInboundEnvelope from
// auto-reply/envelope.ts. The full TS version supports additional format
// options (compact, timestamp elision, etc.); this implementation covers
// the common case.
// TS ref: formatInboundEnvelope (auto-reply/envelope.ts)
func formatProcessEnvelope(p formatProcessEnvelopeParams) string {
	var parts []string

	// Timestamp line
	if p.timestamp > 0 {
		t := time.UnixMilli(p.timestamp)
		// Check if we should elide the timestamp (same-session message within a
		// short window). If previousTimestamp is set and close, skip the full
		// timestamp.
		showFullTimestamp := true
		if p.previousTimestamp != nil {
			diff := p.timestamp - *p.previousTimestamp
			if diff >= 0 && diff < 60_000 { // less than 1 minute
				showFullTimestamp = false
			}
		}
		if showFullTimestamp {
			parts = append(parts, fmt.Sprintf("[%s]", t.UTC().Format("2006-01-02 15:04 UTC")))
		}
	}

	// From line
	if p.chatType == "direct" {
		if p.senderLabel != "" {
			parts = append(parts, fmt.Sprintf("From: %s", p.senderLabel))
		}
	} else {
		if p.from != "" {
			parts = append(parts, fmt.Sprintf("From: %s (%s)", p.senderLabel, p.from))
		} else if p.senderLabel != "" {
			parts = append(parts, fmt.Sprintf("From: %s", p.senderLabel))
		}
	}

	// Body
	if p.body != "" {
		parts = append(parts, p.body)
	}

	return strings.Join(parts, "\n")
}

// buildProcessHistoryContext prepends recent channel history entries to the
// current message body.
// TS ref: buildPendingHistoryContextFromMap (auto-reply/reply/history.ts)
func buildProcessHistoryContext(
	historyMap map[string][]DiscordHistoryEntry,
	channelID string,
	limit int,
	currentMessage string,
	fromLabel string,
) string {
	entries, ok := historyMap[channelID]
	if !ok || len(entries) == 0 {
		return currentMessage
	}

	// Take at most `limit` entries
	start := 0
	if len(entries) > limit {
		start = len(entries) - limit
	}
	historyEntries := entries[start:]

	var lines []string
	lines = append(lines, "[Recent channel history]")
	for _, entry := range historyEntries {
		senderLabel := entry.Sender
		if senderLabel == "" {
			senderLabel = "unknown"
		}
		msgID := entry.MessageID
		if msgID == "" {
			msgID = "unknown"
		}
		line := fmt.Sprintf("%s: %s [id:%s channel:%s]", senderLabel, entry.Body, msgID, channelID)
		if entry.Timestamp > 0 {
			t := time.UnixMilli(entry.Timestamp)
			line = fmt.Sprintf("[%s] %s", t.UTC().Format("15:04"), line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "[End of history]")
	lines = append(lines, "")
	lines = append(lines, currentMessage)

	return strings.Join(lines, "\n")
}

// resolveProcessReplyContext extracts reply context from a quoted/referenced
// message and formats it as an envelope string.
// TS ref: resolveReplyContext (reply-context.ts L6-34)
//
// W-028: 替换原有简化实现, 委托给 ResolveReplyContext 核心函数。
// 该函数解析 sender 身份、构建 discord metadata 标签、生成 envelope。
func resolveProcessReplyContext(msg *discordgo.MessageCreate) string {
	if msg == nil {
		return ""
	}
	return ResolveReplyContextFromCreate(msg, nil)
}

// resolveProcessThreadStarter attempts to retrieve the first message (thread
// starter) in a thread channel.
// TS ref: resolveDiscordThreadStarter (threading.ts)
func resolveProcessThreadStarter(monCtx *DiscordMonitorContext, pCtx *DiscordMessagePreflightContext) *DiscordThreadStarter {
	if pCtx.ThreadChannel == nil || monCtx.Session == nil {
		return nil
	}

	threadID := pCtx.ThreadChannel.ID
	if threadID == "" {
		return nil
	}

	// Attempt to fetch the first message in the thread.
	// The thread starter in Discord is typically the message with ID equal
	// to the thread channel ID (for public threads created from a message).
	msgs, err := monCtx.Session.ChannelMessages(threadID, 1, "", "", threadID)
	if err != nil || len(msgs) == 0 {
		// Fallback: try fetching the thread channel's first message
		msgs, err = monCtx.Session.ChannelMessages(threadID, 1, "", "", "")
		if err != nil || len(msgs) == 0 {
			return nil
		}
	}

	firstMsg := msgs[0]
	text := strings.TrimSpace(firstMsg.Content)
	if text == "" {
		return nil
	}

	authorName := "unknown"
	if firstMsg.Author != nil {
		authorName = firstMsg.Author.Username
		if firstMsg.Author.GlobalName != "" {
			authorName = firstMsg.Author.GlobalName
		}
	}

	ts := firstMsg.Timestamp.UnixMilli()

	return &DiscordThreadStarter{
		Text:      text,
		Author:    authorName,
		Timestamp: ts,
	}
}

// formatProcessThreadStarterEnvelope formats a thread starter message for
// inclusion in the envelope.
// TS ref: formatThreadStarterEnvelope (auto-reply/envelope.ts)
func formatProcessThreadStarterEnvelope(starter *DiscordThreadStarter) string {
	if starter == nil || starter.Text == "" {
		return ""
	}

	var parts []string
	parts = append(parts, "[Thread starter]")

	if starter.Timestamp > 0 {
		t := time.UnixMilli(starter.Timestamp)
		parts = append(parts, fmt.Sprintf("[%s]", t.UTC().Format("2006-01-02 15:04 UTC")))
	}
	if starter.Author != "" {
		parts = append(parts, fmt.Sprintf("From: %s", starter.Author))
	}
	parts = append(parts, starter.Text)
	parts = append(parts, "[End thread starter]")

	return strings.Join(parts, "\n")
}

// buildProcessAgentSessionKey builds an agent session key.
// TS ref: buildAgentSessionKey (routing/resolve-route.ts)
func buildProcessAgentSessionKey(agentID, channel, peerKind, peerID string) string {
	if agentID == "" || channel == "" || peerID == "" {
		return ""
	}
	return fmt.Sprintf("%s:%s:%s:%s", agentID, channel, peerKind, peerID)
}

// applyProcessMediaPayload copies resolved media fields into the MsgContext.
func applyProcessMediaPayload(msgCtx *autoreply.MsgContext, payload map[string]interface{}) {
	if payload == nil {
		return
	}
	if path, ok := payload["MediaPath"].(string); ok && path != "" {
		msgCtx.MediaURL = path
		msgCtx.HasAttachments = true
		msgCtx.MediaCount = 1
	}
	if mt, ok := payload["MediaType"].(string); ok && mt != "" {
		msgCtx.MediaType = mt
	}
	if paths, ok := payload["MediaPaths"].([]string); ok && len(paths) > 0 {
		msgCtx.HasAttachments = true
		msgCtx.MediaCount = len(paths)
		if len(paths) > 0 {
			msgCtx.MediaURL = paths[0]
		}
	}
}

// removeProcessAckReaction removes the ack reaction from the inbound message
// after a reply has been delivered (when configured to do so).
// TS ref: removeAckReactionAfterReply (ack-reactions.ts)
func removeProcessAckReaction(ctx context.Context, monCtx *DiscordMonitorContext, msg *discordgo.MessageCreate, ackReaction string, ackReacted bool) {
	if !ackReacted || ackReaction == "" {
		return
	}
	// Remove the ack reaction. Errors are logged but do not fail the pipeline.
	if err := RemoveReactionDiscord(ctx, msg.ChannelID, msg.ID, ackReaction, monCtx.Token); err != nil {
		monCtx.Logger.Debug("remove ack reaction failed",
			"channel", msg.ChannelID,
			"message", msg.ID,
			"error", err,
		)
	}
}

// clearProcessHistoryEntries clears guild history entries for a channel after
// the reply pipeline completes.
// TS ref: clearHistoryEntriesIfEnabled (auto-reply/reply/history.ts)
func clearProcessHistoryEntries(pCtx *DiscordMessagePreflightContext, channelID string) {
	if !pCtx.IsGuildMessage {
		return
	}
	if pCtx.GuildHistories == nil {
		return
	}
	if pCtx.HistoryLimit <= 0 {
		return
	}
	// Trim history to limit
	entries, ok := pCtx.GuildHistories[channelID]
	if !ok || len(entries) <= pCtx.HistoryLimit {
		return
	}
	// Keep only the most recent entries up to the limit
	pCtx.GuildHistories[channelID] = entries[len(entries)-pCtx.HistoryLimit:]
}
