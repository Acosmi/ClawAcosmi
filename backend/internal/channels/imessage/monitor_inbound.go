//go:build darwin

package imessage

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/internal/autoreply/reply"
	"github.com/anthropic/open-acosmi/internal/config"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// 核心入站消息处理管线 — 对标 TS monitor-provider.ts handleMessageNow() (L249-665)

// InboundPipelineParams 入站管线参数
type InboundPipelineParams struct {
	Ctx     context.Context
	Message *IMessagePayload
	Account ResolvedIMessageAccount
	Config  *types.OpenAcosmiConfig

	// 运行时状态
	SentCache      *SentMessageCache
	GroupHistories *GroupHistories
	Client         *IMessageRpcClient
	Deps           *MonitorDeps

	// 配置值
	AllowFrom          []string
	GroupAllowFrom     []string
	IncludeAttachments bool
	MediaMaxBytes      int
	TextLimit          int
	RemoteHost         string
	RequireMentionOpt  *bool

	// 日志
	LogInfo  func(string)
	LogError func(string)
}

// HandleInboundMessageFull 完整入站消息处理管线。
// 包含: allowFrom 过滤、群组策略、配对、路由、mention 检测、
// 信封格式化、会话记录、回复调度。
func HandleInboundMessageFull(p InboundPipelineParams) error {
	msg := p.Message
	cfg := p.Config
	account := p.Account
	imsgCfg := account.Config

	sender := ""
	if msg.Sender != nil {
		sender = strings.TrimSpace(*msg.Sender)
	}
	if sender == "" {
		return nil
	}
	senderNormalized := NormalizeIMessageHandle(sender)

	if msg.IsFromMe != nil && *msg.IsFromMe {
		return nil
	}

	// --- 群组策略 ---
	chatID := msg.ChatID
	chatGuid := ptrStr(msg.ChatGUID)
	chatIdentifier := ptrStr(msg.ChatIdentifier)
	groupIDCandidate := ""
	if chatID != nil {
		groupIDCandidate = fmt.Sprintf("%d", *chatID)
	}

	groupListPolicy := config.ChannelGroupPolicy{Allowed: true}
	if groupIDCandidate != "" {
		groupListPolicy = config.ResolveChannelGroupPolicy(
			cfg, "imessage", groupIDCandidate, account.AccountID,
		)
	}

	treatAsGroupByConfig := groupIDCandidate != "" &&
		groupListPolicy.AllowlistEnabled && groupListPolicy.GroupConfig != nil

	isGroup := (msg.IsGroup != nil && *msg.IsGroup) || treatAsGroupByConfig
	if isGroup && chatID == nil {
		return nil
	}

	groupID := ""
	if isGroup {
		groupID = groupIDCandidate
	}

	// --- allowFrom 合并 ---
	storeAllowFrom := readStoreAllowFrom(p.Deps)
	effectiveDmAllow := mergeUniq(p.AllowFrom, storeAllowFrom)
	effectiveGroupAllow := mergeUniq(p.GroupAllowFrom, storeAllowFrom)

	// --- 群组策略过滤 ---
	groupPolicy := resolvePolicy(string(imsgCfg.GroupPolicy), "open")
	dmPolicy := resolvePolicy(string(imsgCfg.DmPolicy), "pairing")

	if isGroup {
		if groupPolicy == "disabled" {
			p.LogInfo("imessage: blocked group message (groupPolicy: disabled)")
			return nil
		}
		if groupPolicy == "allowlist" {
			if len(effectiveGroupAllow) == 0 {
				p.LogInfo("imessage: blocked group message (no groupAllowFrom)")
				return nil
			}
			allowed := IsAllowedIMessageSender(IsAllowedParams{
				AllowFrom:      toInterfaceSlice(effectiveGroupAllow),
				Sender:         sender,
				ChatID:         chatID,
				ChatGUID:       chatGuid,
				ChatIdentifier: chatIdentifier,
			})
			if !allowed {
				p.LogInfo(fmt.Sprintf("imessage: blocked sender %s (not in groupAllowFrom)", sender))
				return nil
			}
		}
		if groupListPolicy.AllowlistEnabled && !groupListPolicy.Allowed {
			p.LogInfo(fmt.Sprintf("imessage: skipping group (%s) not in allowlist", groupID))
			return nil
		}
	}

	// --- DM 权限 + 配对 (IM-B) ---
	dmHasWildcard := sliceContains(effectiveDmAllow, "*")
	dmAuthorized := dmPolicy == "open" || dmHasWildcard ||
		(len(effectiveDmAllow) > 0 && IsAllowedIMessageSender(IsAllowedParams{
			AllowFrom:      toInterfaceSlice(effectiveDmAllow),
			Sender:         sender,
			ChatID:         chatID,
			ChatGUID:       chatGuid,
			ChatIdentifier: chatIdentifier,
		}))

	if !isGroup {
		if dmPolicy == "disabled" {
			return nil
		}
		if !dmAuthorized {
			return handlePairing(p, sender, senderNormalized, chatID)
		}
	}

	// --- Agent 路由 ---
	if p.Deps == nil || p.Deps.ResolveAgentRoute == nil {
		p.LogInfo("imessage: inbound received but ResolveAgentRoute not available (DI stub)")
		return nil
	}

	peerKind := "direct"
	peerID := senderNormalized
	if isGroup {
		peerKind = "group"
		peerID = groupID
		if peerID == "" {
			peerID = "unknown"
		}
	}

	route, err := p.Deps.ResolveAgentRoute(AgentRouteParams{
		Channel:   "imessage",
		AccountID: account.AccountID,
		PeerKind:  peerKind,
		PeerID:    peerID,
	})
	if err != nil {
		return fmt.Errorf("resolve agent route: %w", err)
	}

	// --- Mention 检测 ---
	messageText := ""
	if msg.Text != nil {
		messageText = strings.TrimSpace(*msg.Text)
	}

	// 回声检测
	echoScope := account.AccountID + ":" + func() string {
		if isGroup {
			return FormatIMessageChatTarget(chatID)
		}
		return "imessage:" + sender
	}()
	if messageText != "" && p.SentCache != nil && p.SentCache.Has(echoScope, messageText) {
		p.LogInfo("imessage: skipping echo message")
		return nil
	}

	mentionPatterns := resolveMentionPatterns(cfg, route.AgentID)
	mentioned := true
	if isGroup {
		mentioned = reply.MatchesMentionPatterns(messageText, mentionPatterns)
	}

	requireMention := config.ResolveChannelGroupRequireMention(
		cfg, "imessage", groupID, account.AccountID, p.RequireMentionOpt, "before-config",
	)
	canDetectMention := len(mentionPatterns) > 0

	// --- 控制命令门控 ---
	useAccessGroups := true
	if cfg.Commands != nil && cfg.Commands.UseAccessGroups != nil && !*cfg.Commands.UseAccessGroups {
		useAccessGroups = false
	}

	ownerAllowed := len(effectiveDmAllow) > 0 && IsAllowedIMessageSender(IsAllowedParams{
		AllowFrom: toInterfaceSlice(effectiveDmAllow), Sender: sender,
		ChatID: chatID, ChatGUID: chatGuid, ChatIdentifier: chatIdentifier,
	})
	groupAllowed := len(effectiveGroupAllow) > 0 && IsAllowedIMessageSender(IsAllowedParams{
		AllowFrom: toInterfaceSlice(effectiveGroupAllow), Sender: sender,
		ChatID: chatID, ChatGUID: chatGuid, ChatIdentifier: chatIdentifier,
	})

	hasCmd := autoreply.HasControlCommand(messageText)
	gate := ResolveControlCommandGate(ControlCommandGateParams{
		UseAccessGroups:   useAccessGroups,
		AllowTextCommands: true,
		HasControlCommand: hasCmd,
		Authorizers: []ControlCommandAuthorizer{
			{Configured: len(effectiveDmAllow) > 0, Allowed: ownerAllowed},
			{Configured: len(effectiveGroupAllow) > 0, Allowed: groupAllowed},
		},
	})

	commandAuthorized := gate.CommandAuthorized
	if !isGroup {
		commandAuthorized = dmAuthorized
	}
	if isGroup && gate.ShouldBlock {
		LogInboundDrop(p.LogInfo, "imessage", "control command (unauthorized)", sender)
		return nil
	}

	// --- Mention bypass ---
	shouldBypassMention := isGroup && requireMention && !mentioned &&
		commandAuthorized && hasCmd
	effectiveMentioned := mentioned || shouldBypassMention

	historyLimit := resolveHistoryLimit(imsgCfg, cfg)
	historyKey := ""
	if isGroup {
		historyKey = groupID
		if historyKey == "" {
			historyKey = chatGuid
			if historyKey == "" {
				historyKey = chatIdentifier
				if historyKey == "" {
					historyKey = "unknown"
				}
			}
		}
	}

	if isGroup && requireMention && canDetectMention && !mentioned && !shouldBypassMention {
		p.LogInfo("imessage: skipping group message (no mention)")
		if p.GroupHistories != nil && historyKey != "" {
			bodyText := messageText
			if bodyText == "" {
				bodyText = "<empty>"
			}
			var msgIDStr string
			if msg.ID != nil {
				msgIDStr = msg.ID.String()
			}
			var ts *int64
			if msg.CreatedAt != nil {
				t := parseTimestamp(*msg.CreatedAt)
				if t > 0 {
					ts = &t
				}
			}
			p.GroupHistories.RecordPendingHistoryEntry(historyKey, historyLimit, &HistoryEntry{
				Sender:    senderNormalized,
				Body:      bodyText,
				Timestamp: ts,
				MessageID: msgIDStr,
			})
		}
		return nil
	}

	// --- 附件 + 媒体占位符 ---
	attachments := resolveAttachments(msg, p.IncludeAttachments)
	firstMime := ""
	if len(attachments) > 0 && attachments[0].MimeType != nil {
		firstMime = *attachments[0].MimeType
	}
	kind := mediaKindFromMime(firstMime)
	placeholder := FormatMediaPlaceholder(kind, len(attachments))
	bodyText := messageText
	if bodyText == "" {
		bodyText = placeholder
	}
	if bodyText == "" {
		return nil
	}

	// --- 信封格式化 ---
	replyCtx := DescribeReplyContext(msg)
	var createdAt *int64
	if msg.CreatedAt != nil {
		t := parseTimestamp(*msg.CreatedAt)
		if t > 0 {
			createdAt = &t
		}
	}

	chatTarget := FormatIMessageChatTarget(chatID)
	chatName := ptrStr(msg.ChatName)
	fromLabel := FormatInboundFromLabel(FormatFromLabelParams{
		IsGroup:    isGroup,
		GroupLabel: chatName,
		GroupID: func() string {
			if chatID != nil {
				return fmt.Sprintf("%d", *chatID)
			}
			return "unknown"
		}(),
		GroupFallback: "Group",
		DirectLabel:   senderNormalized,
		DirectID:      sender,
	})

	replySuffix := FormatReplySuffix(replyCtx)
	body := FormatInboundEnvelope(FormatEnvelopeParams{
		Channel:   "iMessage",
		From:      fromLabel,
		Timestamp: createdAt,
		Body:      bodyText + replySuffix,
		ChatType: func() string {
			if isGroup {
				return "group"
			}
			return "direct"
		}(),
	})

	// 群组历史上下文
	combinedBody := body
	if isGroup && historyKey != "" && p.GroupHistories != nil {
		combinedBody = p.GroupHistories.BuildPendingHistoryContext(
			historyKey, historyLimit, combinedBody,
			func(entry HistoryEntry) string {
				return FormatInboundEnvelope(FormatEnvelopeParams{
					Channel:     "iMessage",
					SenderLabel: entry.Sender,
					Timestamp:   entry.Timestamp,
					Body:        entry.Body,
					ChatType:    "group",
				})
			},
		)
	}

	// --- 构建 MsgContext ---
	imessageTo := chatTarget
	if imessageTo == "" {
		imessageTo = "imessage:" + sender
	}
	fromField := "imessage:" + sender
	if isGroup {
		fromField = fmt.Sprintf("imessage:group:%s", groupID)
		if groupID == "" {
			fromField = "imessage:group:unknown"
		}
	}

	msgCtx := &autoreply.MsgContext{
		Body:        combinedBody,
		RawBody:     bodyText,
		CommandBody: bodyText,
		From:        fromField,
		To:          imessageTo,
		SessionKey:  route.SessionKey,
		AccountID:   route.AccountID,
		ChatType: func() string {
			if isGroup {
				return "group"
			}
			return "direct"
		}(),
		ConversationLabel:  fromLabel,
		SenderName:         senderNormalized,
		SenderID:           sender,
		Provider:           "imessage",
		Surface:            "imessage",
		IsGroup:            isGroup,
		OriginatingChannel: "imessage",
		OriginatingTo:      imessageTo,
	}

	if msg.ID != nil {
		msgCtx.MessageSid = msg.ID.String()
	}

	// 提及标记
	if effectiveMentioned {
		msgCtx.WasMentioned = "true"
	}
	msgCtx.CommandAuthorized = commandAuthorized

	// 群组元信息
	if isGroup {
		msgCtx.GroupSubject = chatName
		if msg.Participants != nil {
			var parts []string
			for _, p := range msg.Participants {
				if p != "" {
					parts = append(parts, p)
				}
			}
			msgCtx.GroupChannel = strings.Join(parts, ", ")
		}
	}

	reply.FinalizeInboundContext(msgCtx, nil)

	// --- 会话记录 ---
	if p.Deps.RecordInboundSession != nil {
		updateTarget := chatTarget
		if updateTarget == "" {
			updateTarget = sender
		}
		var lastRoute *LastRouteUpdate
		if !isGroup && updateTarget != "" {
			lastRoute = &LastRouteUpdate{
				SessionKey: route.MainSessionKey,
				Channel:    "imessage",
				To:         updateTarget,
				AccountID:  route.AccountID,
			}
		}
		storePath := ""
		if p.Deps.ResolveStorePath != nil {
			storePath = p.Deps.ResolveStorePath(route.AgentID)
		}
		if err := p.Deps.RecordInboundSession(RecordSessionParams{
			StorePath:       storePath,
			SessionKey:      msgCtx.SessionKey,
			Ctx:             msgCtx,
			UpdateLastRoute: lastRoute,
		}); err != nil {
			p.LogInfo(fmt.Sprintf("imessage: failed updating session meta: %s", err))
		}
	}

	p.LogInfo(fmt.Sprintf("imessage inbound: chatId=%s from=%s len=%d",
		func() string {
			if chatID != nil {
				return fmt.Sprintf("%d", *chatID)
			}
			return "unknown"
		}(),
		msgCtx.From, len(body)))

	// --- 回复调度 (IM-D: 分块集成) ---
	if p.Deps.DispatchInboundMessage == nil {
		p.LogInfo("imessage: inbound processed but DispatchInboundMessage not available (DI stub)")
		return nil
	}

	result, err := p.Deps.DispatchInboundMessage(p.Ctx, DispatchParams{
		Ctx:        msgCtx,
		Dispatcher: nil, // 由 DI 层构建 dispatcher
	})
	if err != nil {
		return fmt.Errorf("dispatch inbound: %w", err)
	}

	// 清除群组历史
	if result != nil && result.QueuedFinal && isGroup && historyKey != "" && p.GroupHistories != nil {
		p.GroupHistories.ClearHistoryEntries(historyKey, historyLimit)
	}

	return nil
}

// --- 辅助函数 ---

func handlePairing(p InboundPipelineParams, sender, senderNormalized string, chatID *int) error {
	imsgCfg := p.Account.Config
	dmPolicy := resolvePolicy(string(imsgCfg.DmPolicy), "pairing")
	if dmPolicy != "pairing" {
		p.LogInfo(fmt.Sprintf("imessage: blocked sender %s (dmPolicy=%s)", sender, dmPolicy))
		return nil
	}
	if p.Deps == nil || p.Deps.UpsertPairingRequest == nil {
		p.LogInfo("imessage: pairing needed but UpsertPairingRequest not available")
		return nil
	}

	meta := map[string]string{"sender": senderNormalized}
	if chatID != nil {
		meta["chatId"] = fmt.Sprintf("%d", *chatID)
	}
	result, err := p.Deps.UpsertPairingRequest(PairingRequestParams{
		Channel: "imessage",
		ID:      senderNormalized,
		Meta:    meta,
	})
	if err != nil {
		p.LogInfo(fmt.Sprintf("imessage: pairing upsert failed: %s", err))
		return nil
	}
	if result.Created {
		p.LogInfo(fmt.Sprintf("imessage pairing request sender=%s", senderNormalized))
		replyText := BuildPairingReply("imessage",
			fmt.Sprintf("Your iMessage sender id: %s", senderNormalized),
			result.Code)
		_, sendErr := SendMessageIMessage(p.Ctx, sender, replyText, IMessageSendOpts{
			Client:    p.Client,
			MaxBytes:  p.MediaMaxBytes,
			AccountID: p.Account.AccountID,
			ChatID:    chatID,
		}, p.Config)
		if sendErr != nil {
			p.LogInfo(fmt.Sprintf("imessage pairing reply failed for %s: %s", senderNormalized, sendErr))
		}
	}
	return nil
}

func readStoreAllowFrom(deps *MonitorDeps) []string {
	if deps == nil || deps.ReadAllowFromStore == nil {
		return nil
	}
	list, err := deps.ReadAllowFromStore("imessage")
	if err != nil {
		return nil
	}
	return list
}

func resolvePolicy(explicit, defaultVal string) string {
	if explicit != "" {
		return explicit
	}
	return defaultVal
}

func resolveHistoryLimit(imsgCfg types.IMessageAccountConfig, cfg *types.OpenAcosmiConfig) int {
	if imsgCfg.HistoryLimit != nil {
		return int(math.Max(0, float64(*imsgCfg.HistoryLimit)))
	}
	if cfg.Messages != nil && cfg.Messages.GroupChat != nil && cfg.Messages.GroupChat.HistoryLimit != nil {
		return int(math.Max(0, float64(*cfg.Messages.GroupChat.HistoryLimit)))
	}
	return DefaultGroupHistoryLimit
}

func resolveAttachments(msg *IMessagePayload, include bool) []IMessageAttachment {
	if !include || msg.Attachments == nil {
		return nil
	}
	var valid []IMessageAttachment
	for _, a := range msg.Attachments {
		if a.OriginalPath != nil && *a.OriginalPath != "" && (a.Missing == nil || !*a.Missing) {
			valid = append(valid, a)
		}
	}
	return valid
}

func resolveMentionPatterns(cfg *types.OpenAcosmiConfig, agentID string) []*regexp.Regexp {
	// 从 config messages.groupChat.mentionPatterns 中获取
	var patterns []string
	if cfg.Messages != nil && cfg.Messages.GroupChat != nil {
		patterns = cfg.Messages.GroupChat.MentionPatterns
	}
	if len(patterns) == 0 {
		return nil
	}
	return reply.BuildMentionRegexes(patterns)
}

func mergeUniq(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	var result []string
	for _, lists := range [][]string{a, b} {
		for _, v := range lists {
			s := strings.TrimSpace(v)
			if s != "" && !seen[s] {
				seen[s] = true
				result = append(result, s)
			}
		}
	}
	return result
}

func sliceContains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

func toInterfaceSlice(ss []string) []interface{} {
	result := make([]interface{}, len(ss))
	for i, s := range ss {
		result[i] = s
	}
	return result
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func parseTimestamp(s string) int64 {
	// ISO 8601 轻量解析
	if s == "" {
		return 0
	}
	t, err := parseISO8601(s)
	if err != nil {
		return 0
	}
	return t
}

func parseISO8601(s string) (int64, error) {
	layouts := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.UnixMilli(), nil
		}
	}
	return 0, fmt.Errorf("cannot parse timestamp: %s", s)
}
