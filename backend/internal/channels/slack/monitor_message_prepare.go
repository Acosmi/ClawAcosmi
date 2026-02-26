package slack

// Slack 入站消息预处理 — 继承自 src/slack/monitor/message-handler/prepare.ts (583L)
// Phase 9 实现：bot 过滤 + allowlist + mention + 配对。

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// SlackInboundMessage 预处理后的入站消息
type SlackInboundMessage struct {
	ChannelID    string
	ChannelName  string
	ChannelType  SlackChannelType
	SenderID     string
	SenderName   string
	Text         string
	Ts           string
	ThreadTs     string
	WasMentioned bool
	IsReply      bool
	Files        []SlackFile
	// 频道级配置
	RequireMention bool
	AllowBots      *bool
	Users          []interface{}
	Skills         []string
	SystemPrompt   string
}

// PrepareSlackInboundMessage 预处理入站消息。
// 返回: (消息, 跳过原因)。跳过原因为空表示通过。
func PrepareSlackInboundMessage(ctx context.Context, monCtx *SlackMonitorContext, event SlackMessageEvent) (*SlackInboundMessage, string) {
	// 1. 自身消息丢弃
	if monCtx.BotUserID != "" && event.User == monCtx.BotUserID {
		return nil, "self-message"
	}

	// 2. Bot 消息过滤
	if event.BotID != "" {
		channelConfig := ResolveSlackChannelConfig(event.Channel, "", monCtx.ChannelConfigs, monCtx.RequireMention)
		allowBots := monCtx.AllowBots
		if channelConfig != nil && channelConfig.AllowBots != nil {
			allowBots = *channelConfig.AllowBots
		}
		if !allowBots {
			return nil, "bot-message"
		}
	}

	// 3. 频道类型推断
	channelType := NormalizeSlackChannelType(event.ChannelType, event.Channel)
	isDM := channelType == SlackChannelTypeIM
	isMPIM := channelType == SlackChannelTypeMPIM

	// 4. 频道策略门控
	if !monCtx.IsChannelAllowed(event.Channel, channelType) {
		return nil, fmt.Sprintf("channel-denied:%s", event.Channel)
	}

	// 5. 解析频道名和用户名
	channelName := monCtx.ResolveChannelName(event.Channel)
	senderName := monCtx.ResolveUserName(event.User)

	// 6. DM allowlist 检查
	if isDM {
		if !checkSlackDMSenderAllowed(monCtx, event.User, senderName) {
			// 配对处理
			if monCtx.DMPolicy == "pairing" {
				handleSlackPairing(monCtx, event, senderName)
				return nil, "pairing-request"
			}
			return nil, fmt.Sprintf("dm-blocked:%s", event.User)
		}
	}

	// 7. 频道级配置解析
	channelConfig := ResolveSlackChannelConfig(event.Channel, channelName, monCtx.ChannelConfigs, monCtx.RequireMention)
	requireMention := monCtx.RequireMention
	var channelAllowBots *bool
	var channelUsers []interface{}
	var channelSkills []string
	var channelSystemPrompt string
	if channelConfig != nil {
		requireMention = channelConfig.RequireMention
		channelAllowBots = channelConfig.AllowBots
		channelUsers = channelConfig.Users
		channelSkills = channelConfig.Skills
		channelSystemPrompt = channelConfig.SystemPrompt
	}

	// 8. Mention 检测
	wasMentioned := false
	if monCtx.BotUserID != "" {
		mentionTag := fmt.Sprintf("<@%s>", monCtx.BotUserID)
		wasMentioned = strings.Contains(event.Text, mentionTag)
	}

	// 9. RequireMention 门控（仅在非 DM 频道生效）
	if !isDM && !isMPIM && requireMention && !wasMentioned {
		isReply := event.ThreadTs != ""
		// 线程回复中可以不需要 mention（如果线程父消息被 mention 过）
		if !isReply {
			return nil, "mention-required"
		}
	}

	// 10. 用户级 allowlist（频道配置中的 users 字段）
	if len(channelUsers) > 0 {
		if !ResolveSlackUserAllowed(channelUsers, event.User, senderName) {
			return nil, fmt.Sprintf("channel-user-blocked:%s", event.User)
		}
	}

	// 11. 文本检查
	text := strings.TrimSpace(event.Text)
	if text == "" && len(event.Files) == 0 {
		return nil, "empty-message"
	}

	// 去掉 mention tag
	if wasMentioned && monCtx.BotUserID != "" {
		mentionTag := fmt.Sprintf("<@%s>", monCtx.BotUserID)
		text = strings.TrimSpace(strings.ReplaceAll(text, mentionTag, ""))
	}

	return &SlackInboundMessage{
		ChannelID:      event.Channel,
		ChannelName:    channelName,
		ChannelType:    channelType,
		SenderID:       event.User,
		SenderName:     senderName,
		Text:           text,
		Ts:             event.Ts,
		ThreadTs:       event.ThreadTs,
		WasMentioned:   wasMentioned,
		IsReply:        event.ThreadTs != "",
		Files:          event.Files,
		RequireMention: requireMention,
		AllowBots:      channelAllowBots,
		Users:          channelUsers,
		Skills:         channelSkills,
		SystemPrompt:   channelSystemPrompt,
	}, ""
}

// checkSlackDMSenderAllowed 检查 DM 发送者是否被允许。
func checkSlackDMSenderAllowed(monCtx *SlackMonitorContext, userID, userName string) bool {
	if monCtx.DMPolicy == "open" {
		return true
	}
	// 静态 allowlist
	if IsSlackSenderAllowListed(monCtx.AllowFrom, userID, userName) {
		return true
	}
	// 动态 allowlist（pairing store）
	if monCtx.Deps != nil && monCtx.Deps.ReadAllowFromStore != nil {
		dynamic, err := monCtx.Deps.ReadAllowFromStore("slack")
		if err == nil && IsSlackSenderAllowListed(dynamic, userID, userName) {
			return true
		}
	}
	return false
}

// handleSlackPairing 处理 Slack 配对请求。
func handleSlackPairing(monCtx *SlackMonitorContext, event SlackMessageEvent, senderName string) {
	if monCtx.Deps == nil || monCtx.Deps.UpsertPairingRequest == nil {
		log.Printf("[slack:%s] pairing needed but UpsertPairingRequest not available", monCtx.AccountID)
		return
	}

	meta := map[string]string{
		"sender":     event.User,
		"senderName": senderName,
	}
	result, err := monCtx.Deps.UpsertPairingRequest(SlackPairingRequestParams{
		Channel: "slack",
		ID:      event.User,
		Meta:    meta,
	})
	if err != nil {
		log.Printf("[slack:%s] pairing upsert failed: %v", monCtx.AccountID, err)
		return
	}
	if result.Created {
		replyText := fmt.Sprintf("👋 Hi! This Slack account is paired.\nYour Slack ID: %s\nPairing code: %s\nTo approve: /pair approve %s",
			event.User, result.Code, result.Code)
		_, _ = monCtx.Client.PostMessage(context.Background(), PostMessageParams{
			Channel: event.Channel,
			Text:    replyText,
		})
	}
}
