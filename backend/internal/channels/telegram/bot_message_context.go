package telegram

// Telegram 消息上下文构建 — 继承自 src/telegram/bot-message-context.ts (703L)
// 核心功能：从 Telegram 消息中提取完整的入站消息上下文
// 包含 DM 访问控制/配对流、提及门控、群组/话题启用检查、ACK 反应、
// 富回复上下文、转发前缀、信封格式化。

import (
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/internal/autoreply/reply"
	"github.com/anthropic/open-acosmi/internal/channels"
	"github.com/anthropic/open-acosmi/internal/routing"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// TelegramMediaRef 入站媒体引用
type TelegramMediaRef struct {
	Path            string
	ContentType     string
	StickerMetadata *StickerMetadata
	Data            []byte // 下载后的原始文件数据
	FileName        string // 原始文件名
}

// TelegramMessageContextOptions 消息处理选项
type TelegramMessageContextOptions struct {
	ForceWasMentioned bool
	MessageIDOverride string
}

// TelegramMessageContext 完整的入站消息上下文
type TelegramMessageContext struct {
	// 发送者
	SenderID       string
	SenderUsername string
	SenderLabel    string
	SenderName     string
	IsGroup        bool
	IsForum        bool
	ChatID         int64
	MessageID      int
	ThreadID       int
	ResolvedThread *TelegramThreadSpec

	// 路由
	AccountID  string
	AgentID    string
	SessionKey string

	// 内容
	Text      string
	RawBody   string
	Body      string // 格式化后的信封 body
	MediaRefs []TelegramMediaRef
	Location  *TelegramLocationInfo

	// 单媒体字段 (第一个媒体)
	MediaPath string
	MediaType string
	MediaUrl  string

	// 多媒体字段
	MediaPaths []string
	MediaTypes []string
	MediaUrls  []string

	// 位置上下文
	LocationLatitude  *float64
	LocationLongitude *float64

	// 贴纸上下文
	StickerSetName string
	StickerEmoji   string

	// 上下文
	WasMentioned     bool
	IsReply          bool
	ReplyToMessageID int
	ReplyToID        string
	ReplyToBody      string
	ReplyToSender    string
	ReplyToIsQuote   bool
	ForwardContext   string
	ReplyQuoteText   string

	// 转发详细上下文
	ForwardedFrom          string
	ForwardedFromType      string
	ForwardedFromID        string
	ForwardedFromUsername  string
	ForwardedFromTitle     string
	ForwardedFromSignature string
	ForwardedFromChatType  string
	ForwardedFromMessageID int
	ForwardedDate          int64

	// 元数据
	ReceivedAt        time.Time
	Timestamp         int64
	BotUsername       string
	GroupLabel        string
	ConversationLabel string
	ParentPeer        string
	MessageSid        string

	// 群组/话题
	GroupSubject      string
	GroupSystemPrompt string
	SkillFilter       []string

	// 控制
	StoreAllowFrom    []string
	Allowed           bool
	CommandAuthorized bool
	CommandBody       string

	// ACK 反应
	AckReaction       string
	ShouldAckReaction bool

	// 配置(来自群组)
	GroupConfig *types.TelegramGroupConfig
	TopicConfig *types.TelegramTopicConfig
}

// TelegramLocationInfo 位置信息
type TelegramLocationInfo struct {
	Latitude  float64
	Longitude float64
	Text      string
}

// TelegramHistoryEntry 群组历史条目（用于 pending history）
type TelegramHistoryEntry struct {
	Sender    string
	Body      string
	Timestamp int64
	MessageID string
}

// BuildTelegramMessageContextParams 构建消息上下文所需参数
type BuildTelegramMessageContextParams struct {
	Msg            *TelegramMessage
	AllMedia       []TelegramMediaRef
	StoreAllowFrom []string
	Options        *TelegramMessageContextOptions
	BotID          int64
	BotUsername    string
	Config         *types.OpenAcosmiConfig
	AccountID      string
	AllowFrom      NormalizedAllowFrom
	GroupAllowFrom NormalizedAllowFrom

	// 扩展字段（从 TS BuildTelegramMessageContextParams 移植）
	RequireMention   bool
	DMPolicy         string // "pairing" | "allowlist" | "open" | "disabled"
	GroupPolicy      string // "open" | "disabled" | "allowlist" | ...
	AckReactionScope string // "off" | "group-mentions" | "group-all" | "direct" | "all"
	HistoryLimit     int
	MediaMaxBytes    int
	NativeEnabled    bool
	StreamMode       string // "off" | "partial" | "block"

	// DI 依赖
	Deps *TelegramMonitorDeps

	// 群组/话题配置解析
	GroupConfig *types.TelegramGroupConfig
	TopicConfig *types.TelegramTopicConfig

	// 群组历史缓存 (peer -> entries)
	GroupHistories map[string][]TelegramHistoryEntry

	// 提及正则模式 (从 cfg.messages.groupChat.mentionPatterns 构建)
	MentionRegexes []*regexp.Regexp

	// 提及门控函数（可选覆盖）
	ResolveGroupRequireMention func(chatID int64) bool
	ResolveGroupActivation     func(chatID int64, threadID *int, sessionKey string) *bool
}

// BuildTelegramMessageContext 从原始消息构建完整上下文。
// 路由/会话字段（AgentID, SessionKey）通过 Deps.ResolveAgentRoute DI 解析。
// TS 对照: buildTelegramMessageContext (bot-message-context.ts L128-698)
func BuildTelegramMessageContext(params BuildTelegramMessageContextParams) *TelegramMessageContext {
	msg := params.Msg
	if msg == nil {
		return nil
	}

	chatID := msg.Chat.ID
	isGroup := msg.Chat.Type == "group" || msg.Chat.Type == "supergroup"
	isForum := msg.Chat.IsForum

	// 话题/线程
	threadSpec := ResolveTelegramThreadSpec(isGroup, isForum, msg.MessageThreadID)
	resolvedThreadID := threadSpec.ID
	if threadSpec.Scope == "forum" {
		resolvedThreadID = threadSpec.ID
	} else {
		resolvedThreadID = nil
	}
	threadID := 0
	if threadSpec.ID != nil {
		threadID = *threadSpec.ID
	}

	// -----------------------------------------------------------
	// Agent 路由解析 (TS L168-185)
	// -----------------------------------------------------------
	peerId := ""
	if isGroup {
		peerId = BuildTelegramGroupPeerID(chatID, resolvedThreadID)
	} else {
		peerId = strconv.FormatInt(chatID, 10)
	}
	parentPeer := BuildTelegramParentPeer(isGroup, resolvedThreadID, chatID)

	var agentID string
	var baseSessionKey string
	var routeAccountID string
	var mainSessionKey string

	if params.Deps != nil && params.Deps.ResolveAgentRoute != nil {
		routeThreadID := ""
		if resolvedThreadID != nil {
			routeThreadID = strconv.Itoa(*resolvedThreadID)
		}
		// parentPeer 用于 context 中的 ParentPeer 字段
		route, routeErr := params.Deps.ResolveAgentRoute(TelegramAgentRouteParams{
			Channel:   "telegram",
			AccountID: params.AccountID,
			PeerKind: func() string {
				if isGroup {
					return "group"
				}
				return "direct"
			}(),
			PeerID:   peerId,
			ThreadID: routeThreadID,
		})
		if routeErr != nil {
			slog.Warn("telegram: agent route resolution failed", "err", routeErr)
		} else if route != nil {
			agentID = route.AgentID
			baseSessionKey = route.SessionKey
			routeAccountID = route.AccountID
			mainSessionKey = route.MainSessionKey
		}
	}

	// 回退默认 session key
	if baseSessionKey == "" {
		baseSessionKey = fmt.Sprintf("telegram:%s:%d", params.AccountID, chatID)
	}
	if routeAccountID == "" {
		routeAccountID = params.AccountID
	}

	// DM 线程 session key 解析 (TS L180-185)
	dmThreadID := func() *int {
		if threadSpec.Scope == "dm" {
			return threadSpec.ID
		}
		return nil
	}()

	sessionKey := baseSessionKey
	if dmThreadID != nil {
		threadKeys := routing.ResolveThreadSessionKeys(
			baseSessionKey,
			strconv.Itoa(*dmThreadID),
			mainSessionKey,
			true, // useSuffix: DM threads append :thread:<id>
		)
		sessionKey = threadKeys.SessionKey
	}

	// 群组/话题配置
	groupConfig := params.GroupConfig
	topicConfig := params.TopicConfig

	// -----------------------------------------------------------
	// 3. Group/Topic Enabled Checks (TS L195-204)
	// -----------------------------------------------------------
	if isGroup && groupConfig != nil && groupConfig.Enabled != nil && !*groupConfig.Enabled {
		return nil
	}
	if isGroup && topicConfig != nil && topicConfig.Enabled != nil && !*topicConfig.Enabled {
		return nil
	}

	// 发送者
	senderID := ""
	senderUsername := ""
	senderLabel := ""
	senderName := ""
	if msg.From != nil {
		senderID = fmt.Sprintf("%d", msg.From.ID)
		senderUsername = msg.From.Username
		senderLabel = BuildSenderLabel(msg, senderID)
		senderName = BuildSenderName(msg)
	}

	// -----------------------------------------------------------
	// 1. DM Access Control / Pairing Flow (TS L226-306)
	// -----------------------------------------------------------
	dmPolicy := params.DMPolicy
	if dmPolicy == "" {
		dmPolicy = "pairing" // 默认安全策略
	}

	if !isGroup {
		switch dmPolicy {
		case "disabled":
			// DM 完全禁用，跳过
			return nil

		case "open":
			// 允许所有 DM，无需检查

		case "allowlist", "pairing":
			// 检查发送者是否在允许列表中
			candidate := fmt.Sprintf("%d", chatID)
			allowMatch := ResolveSenderAllowMatch(params.AllowFrom, candidate, senderUsername)
			allowed := params.AllowFrom.HasWildcard ||
				(params.AllowFrom.HasEntries && allowMatch.Allowed)

			if !allowed {
				if dmPolicy == "pairing" {
					// 配对流程：生成配对码并发送回复
					handlePairingRequest(params, msg, chatID, candidate, senderUsername)
				}
				return nil
			}
		}
	}

	// -----------------------------------------------------------
	// 群组 allowFrom 覆盖检查 (TS L311-323)
	// -----------------------------------------------------------
	if isGroup {
		// 检查 topic/group 级 allowFrom 覆盖
		var groupAllowOverride []interface{}
		if topicConfig != nil && topicConfig.AllowFrom != nil {
			groupAllowOverride = topicConfig.AllowFrom
		} else if groupConfig != nil && groupConfig.AllowFrom != nil {
			groupAllowOverride = groupConfig.AllowFrom
		}
		if groupAllowOverride != nil {
			overrideAllow := normalizeInterfaceAllowFrom(groupAllowOverride)
			if !IsSenderAllowed(overrideAllow, senderID, senderUsername) {
				return nil
			}
		}
	}

	// 文本
	rawTextSource := msg.Text
	if rawTextSource == "" {
		rawTextSource = msg.Caption
	}

	// 扩展 text_links
	entities := msg.Entities
	if len(entities) == 0 {
		entities = msg.CaptionEntities
	}
	text := ExpandTextLinks(rawTextSource, entities)
	text = strings.TrimSpace(text)

	// 位置
	var locInfo *TelegramLocationInfo
	var locLat, locLon *float64
	locationData := ExtractTelegramLocation(msg)
	locationText := ""
	if locationData != nil {
		locationText = FormatLocationText(locationData)
		locInfo = &TelegramLocationInfo{
			Latitude:  locationData.Latitude,
			Longitude: locationData.Longitude,
			Text:      locationText,
		}
		lat := locationData.Latitude
		lon := locationData.Longitude
		locLat = &lat
		locLon = &lon
	}

	// -----------------------------------------------------------
	// 媒体占位符 (TS L343-371)
	// -----------------------------------------------------------
	placeholder := resolveMediaPlaceholder(msg, params.AllMedia)

	// 构建 rawBody
	rawBody := joinNonEmpty("\n", text, locationText)
	rawBody = strings.TrimSpace(rawBody)
	if rawBody == "" {
		rawBody = placeholder
	}
	if rawBody == "" && len(params.AllMedia) == 0 {
		return nil
	}

	bodyText := rawBody
	if bodyText == "" && len(params.AllMedia) > 0 {
		if len(params.AllMedia) > 1 {
			bodyText = fmt.Sprintf("<media:image> (%d images)", len(params.AllMedia))
		} else {
			bodyText = "<media:image>"
		}
	}

	// -----------------------------------------------------------
	// 提及检测 (TS L388-401)
	// -----------------------------------------------------------
	botUsername := strings.ToLower(params.BotUsername)
	hasAnyMention := hasAnyMentionEntity(msg)
	explicitlyMentioned := false
	if botUsername != "" {
		explicitlyMentioned = HasBotMention(msg, botUsername)
	}

	// 使用 regex 模式匹配提及
	computedWasMentioned := reply.MatchesMentionWithExplicit(
		rawTextSource,
		params.MentionRegexes,
		&reply.ExplicitMentionSignal{
			HasAnyMention:         hasAnyMention,
			IsExplicitlyMentioned: explicitlyMentioned,
			CanResolveExplicit:    botUsername != "",
		},
	)

	wasMentioned := computedWasMentioned
	if params.Options != nil && params.Options.ForceWasMentioned {
		wasMentioned = true
	}

	// 回复目标与隐式提及
	replyTarget := DescribeReplyTarget(msg)
	implicitMention := false
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil &&
		msg.ReplyToMessage.From.ID == params.BotID {
		implicitMention = true
	}

	// DM 消息总是被视为提及
	if !isGroup {
		wasMentioned = true
	}

	// -----------------------------------------------------------
	// 2. Mention Gating with Bypass (TS L411-459)
	// -----------------------------------------------------------
	requireMention := params.RequireMention
	// DY-012: 修正 requireMention 优先级顺序（对齐 TS firstDefined 语义）
	// 使用 last-wins 模式实现 TS firstDefined 优先级:
	//   groupActivation > topicConfig > groupConfig > resolveGroupRequireMention > base
	// last-wins 中后赋值的优先级更高，因此按优先级从低到高排列。
	if params.ResolveGroupRequireMention != nil {
		requireMention = params.ResolveGroupRequireMention(chatID)
	}
	if groupConfig != nil && groupConfig.RequireMention != nil {
		requireMention = *groupConfig.RequireMention
	}
	if topicConfig != nil && topicConfig.RequireMention != nil {
		requireMention = *topicConfig.RequireMention
	}
	// session-based activation 覆盖（最高优先级）
	if params.ResolveGroupActivation != nil {
		if override := params.ResolveGroupActivation(chatID, resolvedThreadID, sessionKey); override != nil {
			requireMention = *override
		}
	}

	// -----------------------------------------------------------
	// Control Command Gating (TS L330-340)
	// -----------------------------------------------------------
	allow := params.AllowFrom
	if isGroup {
		allow = params.GroupAllowFrom
	}
	senderAllowedForCommands := IsSenderAllowed(allow, senderID, senderUsername)
	useAccessGroups := true
	if params.Config != nil && params.Config.Commands != nil && params.Config.Commands.UseAccessGroups != nil {
		useAccessGroups = *params.Config.Commands.UseAccessGroups
	}
	rawTextForCommand := msg.Text
	if rawTextForCommand == "" {
		rawTextForCommand = msg.Caption
	}
	hasControlCommandInMessage := autoreply.HasControlCommand(rawTextForCommand)
	commandGate := channels.ResolveControlCommandGate(channels.ControlCommandGateParams{
		UseAccessGroups:   useAccessGroups,
		Authorizers:       []channels.ControlCommandAuthorizer{{Configured: allow.HasEntries, Allowed: senderAllowedForCommands}},
		AllowTextCommands: true,
		HasControlCommand: hasControlCommandInMessage,
	})
	commandAuthorized := commandGate.CommandAuthorized

	// 未授权的控制命令在群组中直接丢弃 (TS L402-410)
	if isGroup && commandGate.ShouldBlock {
		slog.Debug("telegram: blocked unauthorized control command",
			"chatId", chatID, "sender", senderID)
		return nil
	}

	canDetectMention := botUsername != "" || len(params.MentionRegexes) > 0

	mentionGate := channels.ResolveMentionGatingWithBypass(channels.MentionGateWithBypassParams{
		IsGroup:           isGroup,
		RequireMention:    requireMention,
		CanDetectMention:  canDetectMention,
		WasMentioned:      wasMentioned,
		ImplicitMention:   isGroup && requireMention && implicitMention,
		HasAnyMention:     hasAnyMention,
		AllowTextCommands: true,
		HasControlCommand: hasControlCommandInMessage,
		CommandAuthorized: commandAuthorized,
	})

	effectiveWasMentioned := mentionGate.EffectiveWasMentioned

	if isGroup && requireMention && canDetectMention && mentionGate.ShouldSkip {
		// 记录到 pending history
		historyKey := BuildTelegramGroupPeerID(chatID, resolvedThreadID)
		if params.GroupHistories != nil && params.HistoryLimit > 0 {
			recordPendingHistoryEntry(
				params.GroupHistories,
				historyKey,
				params.HistoryLimit,
				TelegramHistoryEntry{
					Sender:    BuildSenderLabel(msg, senderID),
					Body:      rawBody,
					Timestamp: int64(msg.Date) * 1000,
					MessageID: strconv.Itoa(msg.MessageID),
				},
			)
		}
		return nil
	}

	// -----------------------------------------------------------
	// 8. ACK Reaction Logic (TS L462-499)
	// -----------------------------------------------------------
	ackReactionScope := channels.AckReactionScope(params.AckReactionScope)
	ackReaction := ""
	if params.Config != nil {
		ackReaction = resolveAckReactionFromConfig(params.Config)
	}

	shouldAckReact := false
	if ackReaction != "" {
		shouldAckReact = channels.ShouldAckReaction(channels.AckReactionGateParams{
			Scope:                 ackReactionScope,
			IsDirect:              !isGroup,
			IsGroup:               isGroup,
			IsMentionableGroup:    isGroup,
			RequireMention:        requireMention,
			CanDetectMention:      canDetectMention,
			EffectiveWasMentioned: effectiveWasMentioned,
			ShouldBypassMention:   mentionGate.ShouldBypassMention,
		})
	}

	// -----------------------------------------------------------
	// 7. Rich Reply Target Resolution (TS L501-511)
	// -----------------------------------------------------------
	replySuffix := ""
	replyToMsgID := 0
	isReply := false
	replyToID := ""
	replyToBody := ""
	replyToSender := ""
	replyToIsQuote := false

	if msg.ReplyToMessage != nil {
		isReply = true
		replyToMsgID = msg.ReplyToMessage.MessageID
	}

	if replyTarget != nil {
		replyToID = replyTarget.ID
		replyToBody = replyTarget.Body
		replyToSender = replyTarget.Sender
		replyToIsQuote = replyTarget.Kind == "quote"

		idSuffix := ""
		if replyTarget.ID != "" {
			idSuffix = " id:" + replyTarget.ID
		}
		if replyTarget.Kind == "quote" {
			replySuffix = fmt.Sprintf("\n\n[Quoting %s%s]\n\"%s\"\n[/Quoting]",
				replyTarget.Sender, idSuffix, replyTarget.Body)
		} else {
			replySuffix = fmt.Sprintf("\n\n[Replying to %s%s]\n%s\n[/Replying]",
				replyTarget.Sender, idSuffix, replyTarget.Body)
		}
	}

	// -----------------------------------------------------------
	// 转发上下文 (TS L502, L512-516)
	// -----------------------------------------------------------
	forwardOrigin := NormalizeForwardedContext(msg)
	forwardCtx := ""
	forwardPrefix := ""
	var forwardedFrom, forwardedFromType, forwardedFromID string
	var forwardedFromUsername, forwardedFromTitle, forwardedFromSignature string
	var forwardedFromChatType string
	var forwardedFromMessageID int
	var forwardedDate int64

	if forwardOrigin != nil {
		forwardCtx = forwardOrigin.From
		forwardedFrom = forwardOrigin.From
		forwardedFromType = forwardOrigin.FromType
		forwardedFromID = forwardOrigin.FromID
		forwardedFromUsername = forwardOrigin.FromUsername
		forwardedFromTitle = forwardOrigin.FromTitle
		forwardedFromSignature = forwardOrigin.FromSignature
		forwardedFromChatType = forwardOrigin.FromChatType
		forwardedFromMessageID = forwardOrigin.FromMessageID
		if forwardOrigin.Date > 0 {
			forwardedDate = int64(forwardOrigin.Date) * 1000
		}
		dateStr := ""
		if forwardOrigin.Date > 0 {
			dateStr = fmt.Sprintf(" at %s",
				time.Unix(int64(forwardOrigin.Date), 0).UTC().Format(time.RFC3339))
		}
		forwardPrefix = fmt.Sprintf("[Forwarded from %s%s]\n", forwardOrigin.From, dateStr)
	}

	// 访问控制（复用上方 command gating 中已计算的 allow）
	allowed := IsSenderAllowed(allow, senderID, senderUsername)

	// 群组标签
	groupLabel := ""
	if isGroup {
		groupLabel = BuildGroupLabel(msg, chatID, resolvedThreadID)
	}

	// 会话标签
	conversationLabel := ""
	if isGroup {
		if groupLabel != "" {
			conversationLabel = groupLabel
		} else {
			conversationLabel = fmt.Sprintf("group:%d", chatID)
		}
	} else {
		conversationLabel = BuildSenderLabel(msg, senderID)
	}

	// 消息 ID
	messageSid := strconv.Itoa(msg.MessageID)
	if params.Options != nil && params.Options.MessageIDOverride != "" {
		messageSid = params.Options.MessageIDOverride
	}

	// 时间戳
	var timestamp int64
	if msg.Date > 0 {
		timestamp = int64(msg.Date) * 1000
	}

	// -----------------------------------------------------------
	// 5. Format Inbound Envelope (TS L530-543)
	// -----------------------------------------------------------
	envelopeBody := fmt.Sprintf("%s%s%s", forwardPrefix, bodyText, replySuffix)
	body := formatTelegramInboundEnvelope(formatTelegramInboundEnvelopeParams{
		channel:        "Telegram",
		from:           conversationLabel,
		body:           envelopeBody,
		chatType:       resolveChatType(isGroup),
		senderName:     senderName,
		senderUsername: senderUsername,
		senderID:       senderID,
		senderLabel:    senderLabel,
		timestamp:      timestamp,
	})

	// 群组历史上下文合并
	combinedBody := body
	if isGroup && params.GroupHistories != nil && params.HistoryLimit > 0 {
		historyKey := BuildTelegramGroupPeerID(chatID, resolvedThreadID)
		combinedBody = buildPendingHistoryContext(
			params.GroupHistories, historyKey, params.HistoryLimit, combinedBody,
			groupLabel, chatID, resolvedThreadID,
		)
	}

	// -----------------------------------------------------------
	// 6. Expand media fields (TS L606-624)
	// -----------------------------------------------------------
	var mediaPath, mediaType, mediaUrl string
	var mediaPaths, mediaTypes, mediaUrls []string
	var stickerSetName, stickerEmoji string

	if len(params.AllMedia) > 0 {
		mediaPath = params.AllMedia[0].Path
		mediaType = params.AllMedia[0].ContentType
		mediaUrl = params.AllMedia[0].Path

		mediaPaths = make([]string, len(params.AllMedia))
		mediaUrls = make([]string, len(params.AllMedia))
		mediaTypes = make([]string, 0, len(params.AllMedia))
		for i, m := range params.AllMedia {
			mediaPaths[i] = m.Path
			mediaUrls[i] = m.Path
			if m.ContentType != "" {
				mediaTypes = append(mediaTypes, m.ContentType)
			}
		}

		if params.AllMedia[0].StickerMetadata != nil {
			stickerSetName = params.AllMedia[0].StickerMetadata.SetName
			stickerEmoji = params.AllMedia[0].StickerMetadata.Emoji
		}
	}

	// SkillFilter (TS L564)
	var skillFilter []string
	if topicConfig != nil && topicConfig.Skills != nil {
		skillFilter = topicConfig.Skills
	} else if groupConfig != nil && groupConfig.Skills != nil {
		skillFilter = groupConfig.Skills
	}

	// GroupSystemPrompt (TS L565-570)
	groupSystemPrompt := ""
	if isGroup {
		var systemPromptParts []string
		if groupConfig != nil && strings.TrimSpace(groupConfig.SystemPrompt) != "" {
			systemPromptParts = append(systemPromptParts, strings.TrimSpace(groupConfig.SystemPrompt))
		}
		if topicConfig != nil && strings.TrimSpace(topicConfig.SystemPrompt) != "" {
			systemPromptParts = append(systemPromptParts, strings.TrimSpace(topicConfig.SystemPrompt))
		}
		if len(systemPromptParts) > 0 {
			groupSystemPrompt = strings.Join(systemPromptParts, "\n\n")
		}
	}

	// GroupSubject
	groupSubject := ""
	if isGroup && msg.Chat.Title != "" {
		groupSubject = msg.Chat.Title
	}

	// -----------------------------------------------------------
	// CommandBody 规范化 (TS L571)
	// -----------------------------------------------------------
	commandBody := autoreply.NormalizeCommandBody(rawBody, &autoreply.CommandNormalizeOptions{
		BotUsername: botUsername,
	})

	ts := threadSpec
	return &TelegramMessageContext{
		SenderID:               senderID,
		SenderUsername:         senderUsername,
		SenderLabel:            senderLabel,
		SenderName:             senderName,
		IsGroup:                isGroup,
		IsForum:                isForum,
		ChatID:                 chatID,
		MessageID:              msg.MessageID,
		ThreadID:               threadID,
		ResolvedThread:         &ts,
		AccountID:              routeAccountID,
		AgentID:                agentID,
		SessionKey:             sessionKey,
		Text:                   text,
		RawBody:                rawBody,
		Body:                   combinedBody,
		MediaRefs:              params.AllMedia,
		Location:               locInfo,
		MediaPath:              mediaPath,
		MediaType:              mediaType,
		MediaUrl:               mediaUrl,
		MediaPaths:             mediaPaths,
		MediaTypes:             mediaTypes,
		MediaUrls:              mediaUrls,
		LocationLatitude:       locLat,
		LocationLongitude:      locLon,
		StickerSetName:         stickerSetName,
		StickerEmoji:           stickerEmoji,
		WasMentioned:           effectiveWasMentioned,
		IsReply:                isReply,
		ReplyToMessageID:       replyToMsgID,
		ReplyToID:              replyToID,
		ReplyToBody:            replyToBody,
		ReplyToSender:          replyToSender,
		ReplyToIsQuote:         replyToIsQuote,
		ForwardContext:         forwardCtx,
		ReplyQuoteText:         replyTarget.quoteText(),
		ForwardedFrom:          forwardedFrom,
		ForwardedFromType:      forwardedFromType,
		ForwardedFromID:        forwardedFromID,
		ForwardedFromUsername:  forwardedFromUsername,
		ForwardedFromTitle:     forwardedFromTitle,
		ForwardedFromSignature: forwardedFromSignature,
		ForwardedFromChatType:  forwardedFromChatType,
		ForwardedFromMessageID: forwardedFromMessageID,
		ForwardedDate:          forwardedDate,
		ReceivedAt:             time.Now(),
		Timestamp:              timestamp,
		BotUsername:            params.BotUsername,
		GroupLabel:             groupLabel,
		ConversationLabel:      conversationLabel,
		ParentPeer: func() string {
			if parentPeer != nil {
				return parentPeer.Kind + ":" + parentPeer.ID
			}
			return ""
		}(),
		MessageSid:        messageSid,
		GroupSubject:      groupSubject,
		GroupSystemPrompt: groupSystemPrompt,
		SkillFilter:       skillFilter,
		StoreAllowFrom:    params.StoreAllowFrom,
		Allowed:           allowed,
		CommandAuthorized: commandAuthorized,
		CommandBody:       commandBody,
		AckReaction:       ackReaction,
		ShouldAckReaction: shouldAckReact,
		GroupConfig:       groupConfig,
		TopicConfig:       topicConfig,
	}
}

// -----------------------------------------------------------
// 5. formatTelegramInboundEnvelope (TS L530-543)
// -----------------------------------------------------------

type formatTelegramInboundEnvelopeParams struct {
	channel        string
	from           string
	body           string
	chatType       string
	senderName     string
	senderUsername string
	senderID       string
	senderLabel    string
	timestamp      int64
}

// formatTelegramInboundEnvelope 构建 Telegram 入站消息信封。
// TS 对照: formatInboundEnvelope (auto-reply/envelope.ts)
// 格式:
//
//	[Telegram] From: {senderLabel}
//	{messageBody}
func formatTelegramInboundEnvelope(p formatTelegramInboundEnvelopeParams) string {
	var parts []string

	// 时间戳
	if p.timestamp > 0 {
		t := time.UnixMilli(p.timestamp)
		parts = append(parts, fmt.Sprintf("[%s]", t.UTC().Format("2006-01-02 15:04 UTC")))
	}

	// Channel + From 标签
	fromLine := ""
	if p.chatType == "direct" {
		if p.senderLabel != "" {
			fromLine = fmt.Sprintf("[%s] From: %s", p.channel, p.senderLabel)
		} else {
			fromLine = fmt.Sprintf("[%s]", p.channel)
		}
	} else {
		// 群组消息：显示 senderLabel 和 group from
		if p.from != "" && p.senderLabel != "" {
			fromLine = fmt.Sprintf("[%s] From: %s in %s", p.channel, p.senderLabel, p.from)
		} else if p.senderLabel != "" {
			fromLine = fmt.Sprintf("[%s] From: %s", p.channel, p.senderLabel)
		} else if p.from != "" {
			fromLine = fmt.Sprintf("[%s] From: %s", p.channel, p.from)
		} else {
			fromLine = fmt.Sprintf("[%s]", p.channel)
		}
	}
	parts = append(parts, fromLine)

	// Body
	if p.body != "" {
		parts = append(parts, p.body)
	}

	return strings.Join(parts, "\n")
}

// -----------------------------------------------------------
// 辅助函数
// -----------------------------------------------------------

// handlePairingRequest 处理 DM 配对请求 (TS L245-297)
func handlePairingRequest(params BuildTelegramMessageContextParams, msg *TelegramMessage, chatID int64, candidate, senderUsername string) {
	if params.Deps == nil || params.Deps.UpsertPairingRequest == nil {
		return
	}

	meta := map[string]string{}
	telegramUserID := candidate
	if msg.From != nil {
		if msg.From.ID > 0 {
			telegramUserID = fmt.Sprintf("%d", msg.From.ID)
		}
		if msg.From.Username != "" {
			meta["username"] = msg.From.Username
		}
		if msg.From.FirstName != "" {
			meta["firstName"] = msg.From.FirstName
		}
		if msg.From.LastName != "" {
			meta["lastName"] = msg.From.LastName
		}
	}

	result, err := params.Deps.UpsertPairingRequest(TelegramPairingParams{
		Channel: "telegram",
		ID:      telegramUserID,
		Meta:    meta,
	})
	if err != nil {
		return
	}
	_ = result
	// 注意: 实际的回复消息发送（sendMessage with pairing code）
	// 需要在调用方使用 bot API 发送。此处仅创建配对记录。
	// TS 版本中 bot.api.sendMessage 是在此处直接调用的，
	// 但 Go 版本中 bot API 在上层注入，此处仅负责 pairing store 操作。
}

// resolveMediaPlaceholder 根据消息类型返回媒体占位符 (TS L343-371)
func resolveMediaPlaceholder(msg *TelegramMessage, allMedia []TelegramMediaRef) string {
	if len(msg.Photo) > 0 {
		return "<media:image>"
	}
	if msg.Video != nil || msg.VideoNote != nil {
		return "<media:video>"
	}
	if msg.Audio != nil || msg.Voice != nil {
		return "<media:audio>"
	}
	if msg.Document != nil {
		return "<media:document>"
	}
	if msg.Sticker != nil {
		// 检查是否有缓存的贴纸描述
		if len(allMedia) > 0 && allMedia[0].StickerMetadata != nil {
			sm := allMedia[0].StickerMetadata
			if sm.CachedDescription != "" {
				stickerContext := ""
				contextParts := []string{}
				if sm.Emoji != "" {
					contextParts = append(contextParts, sm.Emoji)
				}
				if sm.SetName != "" {
					contextParts = append(contextParts, fmt.Sprintf("from \"%s\"", sm.SetName))
				}
				if len(contextParts) > 0 {
					stickerContext = " " + strings.Join(contextParts, " ")
				}
				return fmt.Sprintf("[Sticker%s] %s", stickerContext, sm.CachedDescription)
			}
		}
		return "<media:sticker>"
	}
	return ""
}

// hasAnyMentionEntity 检查消息中是否包含任何 mention 实体 (TS L388)
func hasAnyMentionEntity(msg *TelegramMessage) bool {
	entities := msg.Entities
	if len(entities) == 0 {
		entities = msg.CaptionEntities
	}
	for _, ent := range entities {
		if ent.Type == "mention" {
			return true
		}
	}
	return false
}

// resolveAckReactionFromConfig 从配置中解析 ACK 反应 emoji (TS L462)
func resolveAckReactionFromConfig(cfg *types.OpenAcosmiConfig) string {
	if cfg.Messages != nil && cfg.Messages.AckReaction != "" {
		return strings.TrimSpace(cfg.Messages.AckReaction)
	}
	// 使用默认 emoji
	return ""
}

// normalizeInterfaceAllowFrom 将 []interface{} 格式的 allowFrom 转换为 NormalizedAllowFrom
func normalizeInterfaceAllowFrom(items []interface{}) NormalizedAllowFrom {
	strs := make([]string, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			strs = append(strs, v)
		case float64:
			strs = append(strs, fmt.Sprintf("%.0f", v))
		case int:
			strs = append(strs, strconv.Itoa(v))
		case int64:
			strs = append(strs, strconv.FormatInt(v, 10))
		}
	}
	return NormalizeAllowFrom(strs)
}

// recordPendingHistoryEntry 记录 pending history 条目 (TS L444-456)
func recordPendingHistoryEntry(
	historyMap map[string][]TelegramHistoryEntry,
	historyKey string,
	limit int,
	entry TelegramHistoryEntry,
) {
	if historyMap == nil || historyKey == "" || limit <= 0 {
		return
	}
	entries := historyMap[historyKey]
	entries = append(entries, entry)
	// 保持在 limit 之内
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	historyMap[historyKey] = entries
}

// buildPendingHistoryContext 从 pending 历史构建上下文前缀 (TS L545-562)
func buildPendingHistoryContext(
	historyMap map[string][]TelegramHistoryEntry,
	historyKey string,
	limit int,
	currentMessage string,
	groupLabel string,
	chatID int64,
	resolvedThreadID *int,
) string {
	if historyMap == nil || historyKey == "" {
		return currentMessage
	}
	entries, ok := historyMap[historyKey]
	if !ok || len(entries) == 0 {
		return currentMessage
	}

	// 取最后 limit 条
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	var historyParts []string
	for _, entry := range entries {
		label := groupLabel
		if label == "" {
			label = fmt.Sprintf("group:%d", chatID)
		}
		entryIDInfo := ""
		if entry.MessageID != "" {
			entryIDInfo = fmt.Sprintf(" [id:%s chat:%d]", entry.MessageID, chatID)
		}
		line := formatTelegramInboundEnvelope(formatTelegramInboundEnvelopeParams{
			channel:     "Telegram",
			from:        label,
			body:        entry.Body + entryIDInfo,
			chatType:    "group",
			senderLabel: entry.Sender,
			timestamp:   entry.Timestamp,
		})
		historyParts = append(historyParts, line)
	}

	// 清除已使用的历史
	delete(historyMap, historyKey)

	// 历史前缀 + 当前消息标记 + 当前消息
	return strings.Join(historyParts, "\n\n") +
		"\n\n" + reply.CurrentMessageMarker + "\n" + currentMessage
}

// resolveChatType 解析聊天类型字符串
func resolveChatType(isGroup bool) string {
	if isGroup {
		return "group"
	}
	return "direct"
}

// joinNonEmpty 将非空字符串用分隔符连接
func joinNonEmpty(sep string, parts ...string) string {
	var nonEmpty []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}
	return strings.Join(nonEmpty, sep)
}

// quoteText 是 TelegramReplyTarget 的辅助方法，返回引用文本
func (t *TelegramReplyTarget) quoteText() string {
	if t == nil {
		return ""
	}
	if t.Kind == "quote" {
		return t.Body
	}
	return ""
}

// Date 字段访问辅助：TelegramMessage.Date 存储为 int
// 如果未设置 Date 字段，需要在 TelegramMessage 中添加
// 此处假设 msg.Date 已存在（从 JSON 反序列化）
