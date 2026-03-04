package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Telegram 消息分发 — 继承自 src/telegram/bot-message-dispatch.ts (358L)
// 核心功能：
// - 调用 agent 引擎生成回复
// - 管理草稿流（打字预览）
// - 投递回复
// - 处理贴纸视觉理解
// - 空回复回退 (empty response fallback)
// - ACK 反应移除

const emptyResponseFallback = "No response generated. Please try again."

// DispatchTelegramMessageParams 消息分发参数
type DispatchTelegramMessageParams struct {
	Context                 *TelegramMessageContext
	Client                  *http.Client
	Token                   string
	Config                  *types.OpenAcosmiConfig
	ReplyToMode             string
	StreamMode              string
	TextLimit               int
	ResolveBotTopicsEnabled func() bool
	Deps                    *TelegramMonitorDeps
}

// DispatchTelegramMessage 分发消息到 agent 并投递回复。
// 完整管线: routing → MsgContext → typing → dispatch → deliver
func DispatchTelegramMessage(ctx context.Context, params DispatchTelegramMessageParams) error {
	msgCtx := params.Context
	if msgCtx == nil || !msgCtx.Allowed {
		return nil
	}

	slog.Debug("telegram dispatch",
		"chatId", msgCtx.ChatID,
		"sender", msgCtx.SenderLabel,
		"text", truncateForLog(msgCtx.Text, 100),
		"isGroup", msgCtx.IsGroup,
		"mentioned", msgCtx.WasMentioned,
	)

	deps := params.Deps
	if deps == nil {
		slog.Warn("telegram dispatch: no deps configured, skipping")
		return nil
	}

	// 步骤 1: 解析 Agent 路由
	// 优先使用 context building 阶段已解析的路由结果
	agentID := msgCtx.AgentID
	sessionKey := msgCtx.SessionKey

	if agentID == "" && sessionKey == "" {
		// 回退：context building 未注入 Deps 时，在此处重新解析
		peerKind := "direct"
		if msgCtx.IsGroup {
			peerKind = "group"
		}
		if deps.ResolveAgentRoute != nil {
			threadID := ""
			if msgCtx.ThreadID > 0 {
				threadID = strconv.Itoa(msgCtx.ThreadID)
			}
			route, err := deps.ResolveAgentRoute(TelegramAgentRouteParams{
				Channel:   "telegram",
				AccountID: msgCtx.AccountID,
				PeerKind:  peerKind,
				PeerID:    strconv.FormatInt(msgCtx.ChatID, 10),
				ThreadID:  threadID,
			})
			if err != nil {
				return fmt.Errorf("telegram dispatch: resolve route: %w", err)
			}
			if route != nil {
				agentID = route.AgentID
				sessionKey = route.SessionKey
			}
		}
	}

	if sessionKey == "" {
		sessionKey = fmt.Sprintf("telegram:%s:%d", msgCtx.AccountID, msgCtx.ChatID)
	}
	storePath := ""
	if deps.ResolveStorePath != nil && agentID != "" {
		storePath = deps.ResolveStorePath(agentID)
	}

	// 步骤 2: 构建 MsgContext
	arCtx := buildAutoReplyMsgContext(msgCtx, agentID, sessionKey)

	// 步骤 3: 记录入站会话
	if deps.RecordInboundSession != nil {
		recordParams := TelegramRecordSessionParams{
			StorePath:  storePath,
			SessionKey: sessionKey,
			Ctx:        arCtx,
		}
		if !msgCtx.IsGroup {
			recordParams.UpdateLastRoute = &TelegramLastRouteUpdate{
				SessionKey: sessionKey,
				Channel:    "telegram",
				To:         strconv.FormatInt(msgCtx.ChatID, 10),
				AccountID:  msgCtx.AccountID,
			}
		}
		if err := deps.RecordInboundSession(recordParams); err != nil {
			slog.Warn("telegram dispatch: record session failed", "err", err)
		}
	}

	// 步骤 4: 发送 typing indicator
	go sendTypingAction(ctx, params.Client, params.Token, msgCtx.ChatID, msgCtx.ResolvedThread)

	// 步骤 5: 可选 — 创建草稿流
	var draftStream *TelegramDraftStream
	if params.StreamMode == "draft" {
		draftStream = NewTelegramDraftStream(DraftStreamConfig{
			Client:   params.Client,
			Token:    params.Token,
			ChatID:   msgCtx.ChatID,
			Thread:   msgCtx.ResolvedThread,
			MaxChars: params.TextLimit,
		})
	}

	// 步骤 5b: 贴纸视觉理解 (TS L191-243)
	// 对未缓存的贴纸，在分发前获取专用视觉描述
	if len(msgCtx.MediaRefs) > 0 && msgCtx.MediaRefs[0].StickerMetadata != nil {
		sm := msgCtx.MediaRefs[0].StickerMetadata
		if sm.FileID != "" && sm.FileUniqueID != "" && sm.CachedDescription == "" {
			// 尝试通过 DI 回调获取描述
			if deps.DescribeImage != nil && len(msgCtx.MediaRefs[0].Data) > 0 {
				desc, descErr := deps.DescribeImage(ctx, msgCtx.MediaRefs[0].Data, msgCtx.MediaRefs[0].ContentType)
				if descErr != nil {
					slog.Warn("telegram dispatch: sticker describe failed", "err", descErr)
				} else if desc != "" {
					sm.CachedDescription = desc
					// 格式化描述并更新 context
					stickerCtx := formatStickerContext(sm)
					formattedDesc := fmt.Sprintf("[Sticker%s] %s", stickerCtx, desc)
					arCtx.Body = formattedDesc
					arCtx.BodyForAgent = formattedDesc
					// 清除媒体路径，防止原生视觉重复处理
					arCtx.MediaURL = ""
					arCtx.MediaType = ""

					// 缓存描述
					CacheSticker(&CachedSticker{
						FileID:       sm.FileID,
						FileUniqueID: sm.FileUniqueID,
						Emoji:        sm.Emoji,
						SetName:      sm.SetName,
						Description:  desc,
						ReceivedFrom: arCtx.From,
					})
					slog.Debug("telegram dispatch: cached sticker description", "fileUniqueId", sm.FileUniqueID)
				}
			}
		}
	}

	// 步骤 6: 分发到 auto-reply 管线
	if deps.DispatchInboundMessage == nil {
		slog.Warn("telegram dispatch: DispatchInboundMessage not configured")
		return nil
	}

	dispatchResult, err := deps.DispatchInboundMessage(ctx, TelegramDispatchParams{
		Ctx: arCtx,
		OnModelSelected: func(model string) {
			slog.Debug("telegram dispatch: model selected", "model", model)
		},
	})

	// 停止草稿流
	if draftStream != nil {
		draftStream.Stop()
	}

	if err != nil {
		return fmt.Errorf("telegram dispatch: agent reply: %w", err)
	}

	// 步骤 7: 投递回复
	delivered := false
	if dispatchResult != nil && len(dispatchResult.Replies) > 0 {
		replies := convertAutoReplyPayloads(dispatchResult.Replies)

		var deliverErr error
		delivered, deliverErr = DeliverReplies(ctx, DeliverRepliesParams{
			Client:           params.Client,
			Token:            params.Token,
			ChatID:           strconv.FormatInt(msgCtx.ChatID, 10),
			Replies:          replies,
			ReplyToMode:      params.ReplyToMode,
			ReplyToMessageID: msgCtx.MessageID,
			TextLimit:        params.TextLimit,
			Thread:           msgCtx.ResolvedThread,
			ReplyQuoteText:   msgCtx.ReplyQuoteText,
		})
		if deliverErr != nil {
			slog.Warn("telegram dispatch: delivery failed", "err", deliverErr)
		}
		if delivered {
			slog.Debug("telegram dispatch: reply delivered",
				"chatId", msgCtx.ChatID,
				"replyCount", len(replies),
			)
		}
	}

	// 步骤 7b: 空回复回退 (TS L311-328)
	sentFallback := false
	if !delivered && dispatchResult != nil && dispatchResult.QueuedFinal {
		// agent 返回了 final 但投递为空，发送 fallback
		fallbackDelivered, fallbackErr := DeliverReplies(ctx, DeliverRepliesParams{
			Client:           params.Client,
			Token:            params.Token,
			ChatID:           strconv.FormatInt(msgCtx.ChatID, 10),
			Replies:          []ReplyPayload{{Text: emptyResponseFallback, TextMode: "text"}},
			ReplyToMode:      params.ReplyToMode,
			ReplyToMessageID: msgCtx.MessageID,
			TextLimit:        params.TextLimit,
			Thread:           msgCtx.ResolvedThread,
			ReplyQuoteText:   msgCtx.ReplyQuoteText,
		})
		if fallbackErr != nil {
			slog.Warn("telegram dispatch: fallback delivery failed", "err", fallbackErr)
		}
		sentFallback = fallbackDelivered
	}

	// 步骤 8: 清理群组 pending history (TS L332-335, L354-356)
	hasFinalResponse := (dispatchResult != nil && dispatchResult.QueuedFinal) || sentFallback
	if msgCtx.IsGroup && hasFinalResponse {
		// 群组 pending history 在 context building 阶段的 GroupHistories map 中管理
		// 由 BuildTelegramMessageContext 中 buildPendingHistoryContext 处理
		// 此处仅做标记，实际清理在 context building 消费时已完成
	}

	// 步骤 9: ACK 反应移除 (TS L337-353)
	if hasFinalResponse && msgCtx.ShouldAckReaction && msgCtx.AckReaction != "" {
		// 移除 ACK 反应（如果配置了 removeAckAfterReply）
		if params.Config != nil && params.Config.Messages != nil {
			removeAfter := params.Config.Messages.RemoveAckAfterReply != nil && *params.Config.Messages.RemoveAckAfterReply
			if removeAfter {
				go removeAckReaction(ctx, params.Client, params.Token, msgCtx.ChatID, msgCtx.MessageID)
			}
		}
	}

	return nil
}

// formatStickerContext 格式化贴纸上下文字符串
func formatStickerContext(sm *StickerMetadata) string {
	var parts []string
	if sm.Emoji != "" {
		parts = append(parts, sm.Emoji)
	}
	if sm.SetName != "" {
		parts = append(parts, fmt.Sprintf("from \"%s\"", sm.SetName))
	}
	if len(parts) > 0 {
		return " " + strings.Join(parts, " ")
	}
	return ""
}

// removeAckReaction 移除 ACK 反应
func removeAckReaction(ctx context.Context, client *http.Client, token string, chatID int64, messageID int) {
	removeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := callTelegramAPI(removeCtx, client, token, "setMessageReaction",
		map[string]interface{}{
			"chat_id":    chatID,
			"message_id": messageID,
			"reaction":   []interface{}{},
		})
	if err != nil {
		slog.Debug("telegram dispatch: remove ack reaction failed", "err", err)
	}
}

// sendTypingAction 发送 typing indicator
func sendTypingAction(ctx context.Context, client *http.Client, token string, chatID int64, thread *TelegramThreadSpec) {
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	params := map[string]interface{}{
		"chat_id": chatID,
		"action":  "typing",
	}
	applyThreadParams(params, thread)
	_, _ = callTelegramAPI(sendCtx, client, token, "sendChatAction", params)
}

// buildAutoReplyMsgContext 从 TelegramMessageContext 构建 autoreply.MsgContext
func buildAutoReplyMsgContext(msgCtx *TelegramMessageContext, agentID, sessionKey string) *autoreply.MsgContext {
	wasMentioned := ""
	if msgCtx.WasMentioned {
		wasMentioned = "true"
	}

	chatType := "dm"
	if msgCtx.IsGroup {
		chatType = "group"
	}

	from := msgCtx.SenderID
	if msgCtx.IsGroup {
		from = BuildTelegramGroupFrom(msgCtx.ChatID, func() *int {
			if msgCtx.ThreadID > 0 {
				tid := msgCtx.ThreadID
				return &tid
			}
			return nil
		}())
	} else {
		from = fmt.Sprintf("telegram:%d", msgCtx.ChatID)
	}

	return &autoreply.MsgContext{
		Body:              msgCtx.Body,
		RawBody:           msgCtx.RawBody,
		CommandBody:       msgCtx.CommandBody,
		ChatType:          chatType,
		ChannelType:       "telegram",
		ChannelID:         strconv.FormatInt(msgCtx.ChatID, 10),
		From:              from,
		To:                fmt.Sprintf("telegram:%d", msgCtx.ChatID),
		SenderID:          msgCtx.SenderID,
		SenderName:        msgCtx.SenderName,
		SenderDisplayName: msgCtx.SenderLabel,
		SenderUsername:    msgCtx.SenderUsername,
		IsGroup:           msgCtx.IsGroup,
		SessionKey:        sessionKey,
		AccountID:         msgCtx.AccountID,
		WasMentioned:      wasMentioned,
		Timestamp:         msgCtx.ReceivedAt.UnixMilli(),
		MessageSid:        msgCtx.MessageSid,
		MessageThreadID:   strconv.Itoa(msgCtx.ThreadID),
		CommandAuthorized: msgCtx.CommandAuthorized,

		// 媒体
		HasAttachments: len(msgCtx.MediaRefs) > 0,
		MediaCount:     len(msgCtx.MediaRefs),
		MediaType:      msgCtx.MediaType,
		MediaURL:       msgCtx.MediaUrl,

		// 群组上下文
		ConversationLabel: msgCtx.ConversationLabel,
		GroupSubject:      msgCtx.GroupSubject,
		GroupSystemPrompt: msgCtx.GroupSystemPrompt,
	}
}

// convertAutoReplyPayloads 将 autoreply.ReplyPayload 转为 telegram ReplyPayload
func convertAutoReplyPayloads(payloads []autoreply.ReplyPayload) []ReplyPayload {
	replies := make([]ReplyPayload, 0, len(payloads))
	for _, p := range payloads {
		reply := ReplyPayload{
			Text:     p.Text,
			TextMode: "markdown",
		}
		if p.MediaURL != "" {
			reply.Media = append(reply.Media, ReplyMediaItem{URL: p.MediaURL})
		}
		for _, u := range p.MediaURLs {
			reply.Media = append(reply.Media, ReplyMediaItem{URL: u})
		}
		if p.AudioAsVoice && p.MediaURL != "" {
			reply.Voice = &VoicePayload{URL: p.MediaURL}
			reply.Media = nil
		}
		replies = append(replies, reply)
	}
	return replies
}

func truncateForLog(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}
