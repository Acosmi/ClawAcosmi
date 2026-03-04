package slack

// Slack 入站消息分发 — 继承自 src/slack/monitor/message-handler/dispatch.ts (197L)
// Phase 9 实现：session key + 路由 + 历史 + auto-reply 分发。

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/internal/autoreply/reply"
)

// SlackMessageHandlerResult 消息处理结果
type SlackMessageHandlerResult struct {
	Handled bool
	Reason  string
}

// DispatchSlackMessage 分发预处理完成的消息到 auto-reply 管线。
func DispatchSlackMessage(ctx context.Context, monCtx *SlackMonitorContext, msg *SlackInboundMessage) SlackMessageHandlerResult {
	if monCtx.Deps == nil || monCtx.Deps.ResolveAgentRoute == nil {
		log.Printf("[slack:%s] dispatch: ResolveAgentRoute not available (DI stub)", monCtx.AccountID)
		return SlackMessageHandlerResult{Handled: false, Reason: "no-agent-route-di"}
	}

	// 推断 peer 信息
	isDM := msg.ChannelType == SlackChannelTypeIM
	isMPIM := msg.ChannelType == SlackChannelTypeMPIM
	var peerKind, peerID string
	switch {
	case isDM:
		peerKind = "direct"
		peerID = msg.SenderID
	case isMPIM:
		peerKind = "group"
		peerID = msg.ChannelID
	default:
		peerKind = "channel"
		peerID = msg.ChannelID
	}

	// Agent 路由
	route, err := monCtx.Deps.ResolveAgentRoute(SlackAgentRouteParams{
		Channel:   "slack",
		AccountID: monCtx.AccountID,
		PeerKind:  peerKind,
		PeerID:    peerID,
	})
	if err != nil {
		log.Printf("[slack:%s] dispatch: resolve agent route failed: %v", monCtx.AccountID, err)
		return SlackMessageHandlerResult{Handled: false, Reason: "route-error"}
	}

	// 构建地址标识
	slackTo := buildSlackAddress(msg.ChannelType, msg.ChannelID, msg.SenderID)
	fromField := slackTo

	chatType := "channel"
	switch {
	case isDM:
		chatType = "direct"
	case isMPIM:
		chatType = "group"
	}

	// 频道/会话标签
	conversationLabel := msg.ChannelName
	if isDM {
		conversationLabel = msg.SenderName
	}

	// 信封格式化
	body := formatSlackEnvelope("Slack", conversationLabel, msg.Text, chatType)

	// 构建 MsgContext
	msgCtx := &autoreply.MsgContext{
		Body:               body,
		RawBody:            msg.Text,
		CommandBody:        msg.Text,
		From:               fromField,
		To:                 slackTo,
		SessionKey:         route.SessionKey,
		AccountID:          route.AccountID,
		ChatType:           chatType,
		ConversationLabel:  conversationLabel,
		SenderName:         msg.SenderName,
		SenderID:           msg.SenderID,
		Provider:           "slack",
		Surface:            "slack",
		IsGroup:            !isDM,
		OriginatingChannel: "slack",
		OriginatingTo:      slackTo,
		MessageSid:         msg.Ts,
		MessageThreadID:    msg.ThreadTs,
		CommandAuthorized:  true,
		Timestamp:          time.Now().UnixMilli(),
		ChannelID:          msg.ChannelID,
	}

	if msg.WasMentioned {
		msgCtx.WasMentioned = "true"
	}
	if msg.SystemPrompt != "" {
		msgCtx.GroupSystemPrompt = msg.SystemPrompt
	}
	if len(msg.Files) > 0 {
		msgCtx.HasAttachments = true
		msgCtx.MediaCount = len(msg.Files)
	}

	reply.FinalizeInboundContext(msgCtx, nil)

	// 记录入站会话
	if monCtx.Deps.RecordInboundSession != nil {
		var lastRoute *SlackLastRouteUpdate
		if isDM {
			lastRoute = &SlackLastRouteUpdate{
				SessionKey: route.MainSessionKey,
				Channel:    "slack",
				To:         msg.SenderID,
				AccountID:  route.AccountID,
			}
		}
		storePath := ""
		if monCtx.Deps.ResolveStorePath != nil {
			storePath = monCtx.Deps.ResolveStorePath(route.AgentID)
		}
		if err := monCtx.Deps.RecordInboundSession(SlackRecordSessionParams{
			StorePath:       storePath,
			SessionKey:      msgCtx.SessionKey,
			Ctx:             msgCtx,
			UpdateLastRoute: lastRoute,
		}); err != nil {
			log.Printf("[slack:%s] dispatch: record session failed: %v", monCtx.AccountID, err)
		}
	}

	// 分发到 auto-reply
	if monCtx.Deps.DispatchInboundMessage == nil {
		log.Printf("[slack:%s] dispatch: DispatchInboundMessage not available (DI stub)", monCtx.AccountID)
		return SlackMessageHandlerResult{Handled: false, Reason: "no-dispatch-di"}
	}

	_, dispatchErr := monCtx.Deps.DispatchInboundMessage(ctx, SlackDispatchParams{
		Ctx:        msgCtx,
		Dispatcher: nil,
	})
	if dispatchErr != nil {
		log.Printf("[slack:%s] dispatch: failed: %v", monCtx.AccountID, dispatchErr)
		return SlackMessageHandlerResult{Handled: false, Reason: "dispatch-error"}
	}

	return SlackMessageHandlerResult{Handled: true, Reason: "dispatched"}
}

// buildSlackAddress 构建 Slack 地址标识。
func buildSlackAddress(channelType SlackChannelType, channelID, senderID string) string {
	switch channelType {
	case SlackChannelTypeIM:
		return "slack:" + senderID
	case SlackChannelTypeMPIM:
		return "slack:group:" + channelID
	default:
		return "slack:channel:" + channelID
	}
}

// formatSlackEnvelope 格式化 Slack 入站消息信封。
func formatSlackEnvelope(channel, from, body, chatType string) string {
	var parts []string
	if channel != "" {
		parts = append(parts, fmt.Sprintf("[%s]", channel))
	}
	if from != "" {
		parts = append(parts, from)
	}
	if chatType != "" {
		parts = append(parts, fmt.Sprintf("(%s)", chatType))
	}
	header := strings.Join(parts, " ")
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return header
	}
	return header + ": " + trimmed
}
