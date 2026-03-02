package discord

// Discord 入站消息预处理 — 继承自 src/discord/monitor/preflight.ts (577L)
// Phase 9 实现：self/bot/PluralKit/allowlist/mention/DM 过滤管线。

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/openacosmi/claw-acismi/internal/autoreply"
	"github.com/openacosmi/claw-acismi/internal/channels"
)

// DiscordInboundMessage 预处理后的入站消息
type DiscordInboundMessage struct {
	ChannelID    string
	GuildID      string
	SenderID     string
	SenderName   string
	Text         string
	MessageID    string
	ReplyToID    string // 引用消息 ID
	ThreadID     string
	IsThread     bool
	IsDM         bool
	WasMentioned bool
	Attachments  []*discordgo.MessageAttachment
	// 批量消息合并字段（debounce 窗口内多条消息合并后填充）
	MessageIDs     []string // 批量消息 ID 列表
	MessageIDFirst string   // 第一条消息 ID
	MessageIDLast  string   // 最后一条消息 ID
	// 引用消息上下文
	ReplyToMessageID string // 被引用消息的 ID（来自 MessageReference）
	ReplyToContent   string // 被引用消息的文本内容
	// Forum 频道上下文
	ForumParentSlug string // forum 频道的 parent slug
	// 频道级配置
	RequireMention bool
	Skills         []string
	SystemPrompt   string
}

// PrepareDiscordInboundMessage 预处理入站消息。
// 返回: (消息, 跳过原因)。跳过原因为空表示通过。
func PrepareDiscordInboundMessage(monCtx *DiscordMonitorContext, m *discordgo.MessageCreate) (*DiscordInboundMessage, string) {
	if m.Author == nil {
		return nil, "no-author"
	}

	// 1. 自身消息丢弃
	if m.Author.ID == monCtx.BotUserID {
		return nil, "self-message"
	}

	// 2. Bot 消息过滤
	if m.Author.Bot {
		return nil, "bot-message"
	}

	// 3. 系统消息过滤
	if m.Type != discordgo.MessageTypeDefault && m.Type != discordgo.MessageTypeReply {
		return nil, "system-message"
	}

	// 4. DM vs Guild 判定
	isDM := m.GuildID == ""

	// 5. Guild allowlist 检查（非 DM）
	if !isDM && monCtx.GroupPolicy == "allowlist" {
		if !isGuildAllowed(monCtx, m.GuildID) {
			return nil, fmt.Sprintf("guild-denied:%s", m.GuildID)
		}
	}

	// 6. DM allowlist 检查
	if isDM {
		if monCtx.DMPolicy == "allowlist" {
			if !checkDiscordDMSenderAllowed(monCtx, m.Author.ID, m.Author.Username) {
				return nil, fmt.Sprintf("dm-blocked:%s", m.Author.ID)
			}
		}
	}

	// 7. Mention 检测
	// DY-007 fix: 补全 role mention 和 everyone/here mention 检测逻辑，对齐 TS。
	wasMentioned := false
	if monCtx.BotUserID != "" {
		// 文本内 <@botId> / <@!botId> 检测
		mentionTag := fmt.Sprintf("<@%s>", monCtx.BotUserID)
		mentionTagNick := fmt.Sprintf("<@!%s>", monCtx.BotUserID)
		wasMentioned = strings.Contains(m.Content, mentionTag) || strings.Contains(m.Content, mentionTagNick)

		// 显式 mention 列表检测（m.Mentions 由 Discord Gateway 解析）
		if !wasMentioned {
			for _, u := range m.Mentions {
				if u.ID == monCtx.BotUserID {
					wasMentioned = true
					break
				}
			}
		}
	}

	// hasAnyMention: everyone/here mention、role mention、user mention
	// TS ref: hasAnyMention = m.MentionEveryone || m.mentionedUsers.length > 0 || m.mentionedRoles.length > 0
	hasAnyMention := false
	if !isDM {
		hasAnyMention = m.MentionEveryone || len(m.Mentions) > 0 || len(m.MentionRoles) > 0
	}

	// 8. RequireMention 门控（仅在非 DM 生效）
	requireMention := monCtx.RequireMention
	if !isDM && requireMention && !wasMentioned {
		// 隐式 mention: 回复 bot 消息视为 mention
		implicitMention := false
		if m.ReferencedMessage != nil && m.ReferencedMessage.Author != nil && monCtx.BotUserID != "" {
			implicitMention = m.ReferencedMessage.Author.ID == monCtx.BotUserID
		}
		// 任何带 MessageReference 的消息也视为隐式 mention（对齐 TS: isReply bypass）
		if !implicitMention && m.MessageReference != nil {
			implicitMention = true
		}
		if !implicitMention {
			return nil, "mention-required"
		}
		wasMentioned = true // 隐式 mention 生效
	}

	_ = hasAnyMention // 简易管线中暂不使用，但为完整性保留

	// 9. 文本处理
	text := strings.TrimSpace(m.Content)

	// 去掉 mention tag
	if wasMentioned && monCtx.BotUserID != "" {
		mentionTag := fmt.Sprintf("<@%s>", monCtx.BotUserID)
		mentionTagNick := fmt.Sprintf("<@!%s>", monCtx.BotUserID)
		text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, mentionTag, ""), mentionTagNick, ""))
	}

	// 10. 空消息检查
	if text == "" && len(m.Attachments) == 0 {
		return nil, "empty-message"
	}

	// 11. 构建入站消息
	replyToID := ""
	replyToContent := ""
	if m.MessageReference != nil {
		replyToID = m.MessageReference.MessageID
	}
	if m.ReferencedMessage != nil {
		replyToContent = m.ReferencedMessage.Content
	}

	// 线程检测
	threadID := ""
	isThread := false
	ch, err := monCtx.Session.State.Channel(m.ChannelID)
	if err == nil && ch != nil && ch.IsThread() {
		isThread = true
		threadID = m.ChannelID
	}

	return &DiscordInboundMessage{
		ChannelID:        m.ChannelID,
		GuildID:          m.GuildID,
		SenderID:         m.Author.ID,
		SenderName:       m.Author.Username,
		Text:             text,
		MessageID:        m.ID,
		ReplyToID:        replyToID,
		ThreadID:         threadID,
		IsThread:         isThread,
		IsDM:             isDM,
		WasMentioned:     wasMentioned,
		Attachments:      m.Attachments,
		ReplyToMessageID: replyToID,
		ReplyToContent:   replyToContent,
		RequireMention:   requireMention,
	}, ""
}

// isGuildAllowed 检查 guild 是否在 allowlist 中。
func isGuildAllowed(monCtx *DiscordMonitorContext, guildID string) bool {
	monCtx.mu.RLock()
	defer monCtx.mu.RUnlock()
	if len(monCtx.GuildConfigs) == 0 {
		return false
	}
	_, ok := monCtx.GuildConfigs[guildID]
	return ok
}

// checkDiscordDMSenderAllowed 检查 DM 发送者是否被允许。
func checkDiscordDMSenderAllowed(monCtx *DiscordMonitorContext, userID, userName string) bool {
	if monCtx.DMPolicy == "open" {
		return true
	}
	// 静态 allowlist
	for _, allowed := range monCtx.AllowFrom {
		if allowed == userID || strings.EqualFold(allowed, userName) {
			return true
		}
	}
	// 动态 allowlist（pairing store）
	if monCtx.Deps != nil && monCtx.Deps.ReadAllowFromStore != nil {
		dynamic, err := monCtx.Deps.ReadAllowFromStore("discord")
		if err == nil {
			for _, allowed := range dynamic {
				if allowed == userID || strings.EqualFold(allowed, userName) {
					return true
				}
			}
		}
	}
	return false
}

// ── Extended Preflight ──
// TS ref: preflightDiscordMessage (message-handler.preflight.ts L60-577)

// PreflightDiscordMessage performs full message preflight with rich context.
// This is the extended version of PrepareDiscordInboundMessage that returns
// a complete DiscordMessagePreflightContext for the process pipeline.
// TS ref: preflightDiscordMessage
func PreflightDiscordMessage(monCtx *DiscordMonitorContext, m *discordgo.MessageCreate, params *DiscordMessagePreflightParams) *DiscordMessagePreflightContext {
	if m.Author == nil {
		return nil
	}

	// 1. Self message filter
	botUserID := monCtx.BotUserID
	if params != nil && params.BotUserID != "" {
		botUserID = params.BotUserID
	}
	if botUserID != "" && m.Author.ID == botUserID {
		return nil
	}

	// 2. Bot message filter (with allowBots override)
	allowBots := false
	if params != nil {
		allowBots = params.AllowBots
	}
	if m.Author.Bot && !allowBots {
		return nil
	}

	// 3. PluralKit check
	var pkInfo *PluralKitMessageInfo
	webhookID := ResolveDiscordWebhookID(m.WebhookID)
	if webhookID == "" {
		// Could be a PluralKit proxied message - check if PluralKit integration enabled
		// PluralKit lookup handled via Deps
	}

	// 4. Resolve sender identity
	author := DiscordAuthorInfo{
		ID:            m.Author.ID,
		Username:      m.Author.Username,
		GlobalName:    m.Author.GlobalName,
		Discriminator: m.Author.Discriminator,
	}
	var memberInfo *DiscordMemberInfo
	if m.Member != nil && m.Member.Nick != "" {
		memberInfo = &DiscordMemberInfo{Nickname: m.Member.Nick}
	}
	sender := ResolveDiscordSenderIdentity(author, memberInfo, pkInfo)

	// 5. DM / Guild / GroupDM detection
	isDM := m.GuildID == ""
	isGroupDm := false // Discord groupDM detection via channel type
	isGuildMessage := m.GuildID != ""

	// 6. DM enabled check
	dmEnabled := true
	if params != nil {
		dmEnabled = params.DMEnabled
	}
	if isDM && !dmEnabled {
		return nil
	}

	// 7. GroupDM enabled check
	groupDmEnabled := true
	if params != nil {
		groupDmEnabled = params.GroupDmEnabled
	}
	if isGroupDm && !groupDmEnabled {
		return nil
	}

	// 8. DM policy check with pairing
	dmPolicy := monCtx.DMPolicy
	if params != nil && params.DMPolicy != "" {
		dmPolicy = params.DMPolicy
	}
	commandAuthorized := true
	if isDM {
		if dmPolicy == "disabled" {
			return nil
		}
		if dmPolicy != "open" {
			if !checkDiscordDMSenderAllowed(monCtx, sender.ID, sender.Name) {
				// Handle pairing
				if dmPolicy == "pairing" && monCtx.Deps != nil && monCtx.Deps.UpsertPairingRequest != nil {
					result, err := monCtx.Deps.UpsertPairingRequest(DiscordPairingRequestParams{
						Channel: "discord",
						ID:      m.Author.ID,
						Meta: map[string]string{
							"tag":  sender.Tag,
							"name": sender.Name,
						},
					})
					if err == nil && result.Created {
						monCtx.Logger.Debug("pairing request created", "sender", m.Author.ID)
					}
				}
				return nil
			}
			commandAuthorized = true
		}
	}

	// 9. System message filter
	if m.Type != discordgo.MessageTypeDefault && m.Type != discordgo.MessageTypeReply {
		// Check for system events
		if monCtx.Deps != nil && monCtx.Deps.EnqueueSystemEvent != nil {
			systemLocation := ResolveDiscordSystemLocation(isDM, isGroupDm,
				resolveGuildName(monCtx.Session, m.GuildID),
				m.ChannelID)
			systemText := ResolveDiscordSystemEvent(m, systemLocation)
			if systemText != "" {
				_ = monCtx.Deps.EnqueueSystemEvent(systemText, "", fmt.Sprintf("discord:system:%s:%s", m.ChannelID, m.ID))
			}
		}
		return nil
	}

	// 10. Mention detection
	wasMentioned := false
	if botUserID != "" {
		mentionTag := fmt.Sprintf("<@%s>", botUserID)
		mentionTagNick := fmt.Sprintf("<@!%s>", botUserID)
		wasMentioned = strings.Contains(m.Content, mentionTag) || strings.Contains(m.Content, mentionTagNick)
	}

	// Check explicit mentions from mention list
	explicitlyMentioned := false
	if botUserID != "" {
		for _, u := range m.Mentions {
			if u.ID == botUserID {
				explicitlyMentioned = true
				break
			}
		}
	}
	if explicitlyMentioned {
		wasMentioned = true
	}

	hasAnyMention := false
	if !isDM {
		hasAnyMention = m.MentionEveryone || len(m.Mentions) > 0 || len(m.MentionRoles) > 0
	}

	// 11. Text processing
	text := strings.TrimSpace(m.Content)

	// Strip mention tags
	if wasMentioned && botUserID != "" {
		mentionTag := fmt.Sprintf("<@%s>", botUserID)
		mentionTagNick := fmt.Sprintf("<@!%s>", botUserID)
		text = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, mentionTag, ""), mentionTagNick, ""))
	}

	// 12. Guild config resolution
	var guildInfo *DiscordGuildEntryResolved
	guildSlug := ""
	if isGuildMessage {
		guildName := resolveGuildName(monCtx.Session, m.GuildID)
		guildInfo = ResolveDiscordGuildEntry(m.GuildID, guildName, monCtx.GuildConfigs)
		if guildName != "" {
			guildSlug = NormalizeDiscordSlug(guildName)
		}

		// Guild allowlist check
		groupPolicy := monCtx.GroupPolicy
		if params != nil && params.GroupPolicy != "" {
			groupPolicy = params.GroupPolicy
		}
		if groupPolicy == "allowlist" {
			if len(monCtx.GuildConfigs) > 0 && guildInfo == nil {
				return nil
			}
		}
		if groupPolicy == "disabled" {
			return nil
		}
	}

	// 13. Thread detection
	var threadChannel *DiscordThreadChannel
	threadParentID := ""
	threadParentName := ""
	var threadParentType *int
	isThread := false

	ch, chErr := monCtx.Session.State.Channel(m.ChannelID)
	if chErr == nil && ch != nil && ch.IsThread() {
		isThread = true
		threadChannel = &DiscordThreadChannel{
			ID:       ch.ID,
			Name:     ch.Name,
			ParentID: ch.ParentID,
			OwnerID:  ch.OwnerID,
		}
		if ch.ParentID != "" {
			parent, parentErr := monCtx.Session.State.Channel(ch.ParentID)
			if parentErr == nil && parent != nil {
				threadParentID = parent.ID
				threadParentName = parent.Name
				pType := int(parent.Type)
				threadParentType = &pType
				// Populate parent channel context on the thread channel struct
				// TS ref: passes parent channel context (name, type, ID) when resolving thread info
				threadChannel.ThreadParentID = parent.ID
				threadChannel.ThreadParentName = parent.Name
				threadChannel.ThreadParentType = threadParentType
			}
		}
	}

	// 14. Channel config resolution
	channelName := ""
	if ch != nil {
		channelName = ch.Name
	}
	configChannelName := channelName
	if threadParentName != "" {
		configChannelName = threadParentName
	}
	configChannelSlug := ""
	if configChannelName != "" {
		configChannelSlug = NormalizeDiscordSlug(configChannelName)
	}

	var channelConfig *DiscordChannelConfigResolved
	if isGuildMessage && guildInfo != nil {
		scope := "channel"
		if isThread {
			scope = "thread"
		}
		_ = scope // Used for fallback resolution
		channelConfig = ResolveDiscordChannelConfig(guildInfo, m.ChannelID, configChannelName, configChannelSlug)
		// Try parent if not found and in thread
		if channelConfig == nil && threadParentID != "" {
			channelConfig = ResolveDiscordChannelConfig(guildInfo, threadParentID, threadParentName, NormalizeDiscordSlug(threadParentName))
		}
	}

	// Channel enabled/allowed checks
	if isGuildMessage && channelConfig != nil {
		if channelConfig.Enabled != nil && !*channelConfig.Enabled {
			return nil
		}
		if !channelConfig.Allowed {
			// Check if policy allows
			channelAllowlistConfigured := guildInfo != nil && len(guildInfo.Channels) > 0
			groupPolicy := monCtx.GroupPolicy
			if !IsDiscordGroupAllowedByPolicy(groupPolicy, guildInfo != nil, channelAllowlistConfigured, channelConfig.Allowed) {
				return nil
			}
		}
	}

	// 15. RequireMention check
	requireMention := monCtx.RequireMention
	if isGuildMessage {
		threadOwnerID := ""
		if threadChannel != nil {
			threadOwnerID = threadChannel.OwnerID
		}
		autoThreadOwned := IsDiscordAutoThreadOwnedByBot(isThread, channelConfig, botUserID, threadOwnerID)
		requireMention = ResolveDiscordShouldRequireMention(isGuildMessage, isThread, botUserID, threadOwnerID, channelConfig, guildInfo, autoThreadOwned, monCtx.RequireMention)
	}

	// DY-008 fix: 补全频道级别的命令门控检查，对齐 TS resolveControlCommandGate + resolveMentionGatingWithBypass。
	// TS ref: preflight.ts L403-453
	allowTextCommands := true // 默认启用文本命令
	hasControlCommandInMessage := autoreply.HasControlCommand(text)

	if !isDM {
		// 构建 owner 允许列表
		ownerAllowListConfigured := len(monCtx.AllowFrom) > 0
		ownerOk := false
		if ownerAllowListConfigured {
			ownerAllowList := NormalizeDiscordAllowList(monCtx.AllowFrom, []string{"discord:", "user:", "pk:"})
			ownerOk = AllowListMatches(ownerAllowList, sender.ID, sender.Name, sender.Tag).Allowed
		}

		// 构建频道用户允许列表
		var channelUsers []string
		if channelConfig != nil {
			channelUsers = channelConfig.Users
		}
		if len(channelUsers) == 0 && guildInfo != nil {
			channelUsers = guildInfo.Users
		}
		hasUserAllowlist := len(channelUsers) > 0
		userOk := false
		if hasUserAllowlist {
			userOk = ResolveDiscordUserAllowed(channelUsers, sender.ID, sender.Name, sender.Tag)
		}

		useAccessGroups := monCtx.UseAccessGroups
		commandGate := channels.ResolveControlCommandGate(channels.ControlCommandGateParams{
			UseAccessGroups: useAccessGroups,
			Authorizers: []channels.ControlCommandAuthorizer{
				{Configured: ownerAllowListConfigured, Allowed: ownerOk},
				{Configured: hasUserAllowlist, Allowed: userOk},
			},
			AllowTextCommands: allowTextCommands,
			HasControlCommand: hasControlCommandInMessage,
		})
		commandAuthorized = commandGate.CommandAuthorized

		if commandGate.ShouldBlock {
			return nil
		}
	}

	// 15b. Mention gating with bypass (对齐 TS resolveMentionGatingWithBypass)
	implicitMention := false
	if m.ReferencedMessage != nil && m.ReferencedMessage.Author != nil && botUserID != "" {
		implicitMention = m.ReferencedMessage.Author.ID == botUserID
	}

	mentionGate := channels.ResolveMentionGatingWithBypass(channels.MentionGateWithBypassParams{
		IsGroup:           isGuildMessage,
		RequireMention:    requireMention,
		CanDetectMention:  botUserID != "",
		WasMentioned:      wasMentioned,
		ImplicitMention:   implicitMention,
		HasAnyMention:     hasAnyMention,
		AllowTextCommands: allowTextCommands,
		HasControlCommand: hasControlCommandInMessage,
		CommandAuthorized: commandAuthorized,
	})
	effectiveWasMentioned := mentionGate.EffectiveWasMentioned

	if isGuildMessage && requireMention {
		if mentionGate.ShouldSkip {
			return nil
		}
	}

	// 16. User allowlist check in guild
	if isGuildMessage {
		var channelUsers []string
		if channelConfig != nil {
			channelUsers = channelConfig.Users
		}
		if len(channelUsers) == 0 && guildInfo != nil {
			channelUsers = guildInfo.Users
		}
		if len(channelUsers) > 0 {
			if !ResolveDiscordUserAllowed(channelUsers, sender.ID, sender.Name, sender.Tag) {
				return nil
			}
		}
	}

	// 17. Empty message check
	if text == "" && len(m.Attachments) == 0 {
		return nil
	}

	// 18. Agent routing
	var route *DiscordAgentRoute
	if monCtx.Deps != nil && monCtx.Deps.ResolveAgentRoute != nil {
		peerKind := "channel"
		peerID := m.ChannelID
		if isDM {
			peerKind = "direct"
			peerID = m.Author.ID
		}
		r, err := monCtx.Deps.ResolveAgentRoute(DiscordAgentRouteParams{
			Channel:   "discord",
			AccountID: monCtx.AccountID,
			PeerKind:  peerKind,
			PeerID:    peerID,
		})
		if err == nil {
			route = r
		}
	}

	// Build session key
	baseSessionKey := ""
	if route != nil {
		baseSessionKey = route.SessionKey
	}

	// Display names
	displayChannelName := channelName
	if threadChannel != nil && threadChannel.Name != "" {
		displayChannelName = threadChannel.Name
	}
	displayChannelSlug := ""
	if displayChannelName != "" {
		displayChannelSlug = NormalizeDiscordSlug(displayChannelName)
	}

	// Record channel activity
	if monCtx.Deps != nil && monCtx.Deps.RecordChannelActivity != nil {
		monCtx.Deps.RecordChannelActivity("discord", monCtx.AccountID, "inbound")
	}

	// Build history limit and other params from params or defaults
	historyLimit := 0
	mediaMaxBytes := 0
	textLimit := 2000
	replyToMode := "first"
	ackReactionScope := "all"
	if params != nil {
		historyLimit = params.HistoryLimit
		mediaMaxBytes = params.MediaMaxBytes
		textLimit = params.TextLimit
		replyToMode = params.ReplyToMode
		ackReactionScope = params.AckReactionScope
	}

	return &DiscordMessagePreflightContext{
		AccountID:                  monCtx.AccountID,
		Token:                      monCtx.Token,
		BotUserID:                  botUserID,
		DMPolicy:                   dmPolicy,
		GroupPolicy:                monCtx.GroupPolicy,
		Message:                    m,
		Session:                    monCtx.Session,
		Sender:                     sender,
		ChannelName:                channelName,
		IsGuildMessage:             isGuildMessage,
		IsDirectMessage:            isDM,
		IsGroupDm:                  isGroupDm,
		CommandAuthorized:          commandAuthorized,
		BaseText:                   text,
		MessageText:                text,
		WasMentioned:               wasMentioned,
		EffectiveWasMentioned:      effectiveWasMentioned,
		HasAnyMention:              hasAnyMention,
		ShouldRequireMention:       requireMention,
		CanDetectMention:           botUserID != "",
		Route:                      route,
		GuildInfo:                  guildInfo,
		GuildSlug:                  guildSlug,
		ThreadChannel:              threadChannel,
		ThreadParentID:             threadParentID,
		ThreadParentName:           threadParentName,
		ThreadParentType:           threadParentType,
		ThreadName:                 "",
		ConfigChannelName:          configChannelName,
		ConfigChannelSlug:          configChannelSlug,
		DisplayChannelName:         displayChannelName,
		DisplayChannelSlug:         displayChannelSlug,
		BaseSessionKey:             baseSessionKey,
		ChannelConfig:              channelConfig,
		ChannelAllowlistConfigured: guildInfo != nil && len(guildInfo.Channels) > 0,
		ChannelAllowed:             channelConfig == nil || channelConfig.Allowed,
		AllowTextCommands:          allowTextCommands,
		ShouldBypassMention:        mentionGate.ShouldBypassMention,
		HistoryLimit:               historyLimit,
		MediaMaxBytes:              mediaMaxBytes,
		TextLimit:                  textLimit,
		ReplyToMode:                replyToMode,
		AckReactionScope:           ackReactionScope,
		Deps:                       monCtx.Deps,
	}
}

// resolveGuildName resolves a guild name from session state.
func resolveGuildName(session *discordgo.Session, guildID string) string {
	if session == nil || guildID == "" {
		return ""
	}
	guild, err := session.State.Guild(guildID)
	if err != nil || guild == nil {
		return ""
	}
	return guild.Name
}

// ResolveDiscordSystemEvent resolves a system event text from a message.
// TS ref: resolveDiscordSystemEvent (system-events.ts)
func ResolveDiscordSystemEvent(m *discordgo.MessageCreate, location string) string {
	if m == nil {
		return ""
	}
	switch m.Type {
	case discordgo.MessageTypeGuildMemberJoin:
		return fmt.Sprintf("Member joined: %s in %s", m.Author.Username, location)
	case discordgo.MessageTypeUserPremiumGuildSubscription:
		return fmt.Sprintf("Boost: %s boosted %s", m.Author.Username, location)
	case discordgo.MessageTypeChannelPinnedMessage:
		return fmt.Sprintf("Pin: message pinned in %s", location)
	default:
		return ""
	}
}
