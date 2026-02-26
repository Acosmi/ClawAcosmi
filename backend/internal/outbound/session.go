package outbound

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ---------- 出站会话路由 ----------

// OutboundSessionRoute 出站消息会话路由结果。
type OutboundSessionRoute struct {
	SessionKey     string `json:"sessionKey"`
	BaseSessionKey string `json:"baseSessionKey"`
	PeerKind       string `json:"peerKind"` // "direct", "group", "channel"
	PeerID         string `json:"peerId"`
	ChatType       string `json:"chatType"` // "direct", "group", "channel"
	From           string `json:"from"`
	To             string `json:"to"`
	ThreadID       string `json:"threadId,omitempty"`
}

// SessionResolveParams 会话解析参数。
type SessionResolveParams struct {
	Channel   string
	AgentID   string
	AccountID string
	Target    string
	ReplyToID string
	ThreadID  string
}

// SessionKeyBuilder 会话 key 构建器（依赖注入）。
type SessionKeyBuilder interface {
	BuildAgentSessionKey(agentID, channel, accountID, peerKind, peerID, dmScope string) string
	ResolveThreadSessionKey(baseKey, threadID string) string
}

// ---------- 频道会话解析器注册表 ----------

// ChannelSessionResolver 频道会话解析函数。
type ChannelSessionResolver func(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute

var channelResolvers = map[string]ChannelSessionResolver{}

// RegisterChannelSessionResolver 注册频道会话解析器。
func RegisterChannelSessionResolver(channel string, resolver ChannelSessionResolver) {
	channelResolvers[channel] = resolver
}

// ResolveOutboundSessionRoute 解析出站消息的会话路由。
func ResolveOutboundSessionRoute(params SessionResolveParams, kb SessionKeyBuilder) (*OutboundSessionRoute, error) {
	resolver, ok := channelResolvers[params.Channel]
	if !ok {
		return nil, fmt.Errorf("unsupported channel: %s", params.Channel)
	}
	route := resolver(params, kb)
	if route == nil {
		return nil, fmt.Errorf("failed to resolve session for channel=%s target=%s", params.Channel, params.Target)
	}
	return route, nil
}

// ---------- 工具函数 ----------

var (
	uuidRE        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	uuidCompactRE = regexp.MustCompile(`^[0-9a-f]{32}$`)
	hexRE         = regexp.MustCompile(`^[0-9a-f]+$`)
	hexLetterRE   = regexp.MustCompile(`[a-f]`)
)

func looksLikeUUID(v string) bool {
	lower := strings.ToLower(v)
	if uuidRE.MatchString(lower) || uuidCompactRE.MatchString(lower) {
		return true
	}
	compact := strings.ReplaceAll(lower, "-", "")
	if !hexRE.MatchString(compact) {
		return false
	}
	return hexLetterRE.MatchString(compact)
}

func normalizeThreadID(v string) string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return ""
	}
	// 如果是数字，取整
	if n, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return strconv.FormatInt(int64(n), 10)
	}
	return trimmed
}

func stripProviderPrefix(raw, channel string) string {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	prefix := strings.ToLower(channel) + ":"
	if strings.HasPrefix(lower, prefix) {
		return strings.TrimSpace(trimmed[len(prefix):])
	}
	return trimmed
}

func stripKindPrefix(raw string) string {
	prefixes := []string{"user:", "channel:", "group:", "conversation:", "room:", "dm:"}
	lower := strings.ToLower(raw)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(raw[len(p):])
		}
	}
	return strings.TrimSpace(raw)
}

// ---------- 内置频道解析器 ----------

func init() {
	RegisterChannelSessionResolver("whatsapp", resolveWhatsAppSession)
	RegisterChannelSessionResolver("signal", resolveSignalSession)
	RegisterChannelSessionResolver("telegram", resolveTelegramSession)
	RegisterChannelSessionResolver("discord", resolveDiscordSession)
	RegisterChannelSessionResolver("slack", resolveSlackSession)
	RegisterChannelSessionResolver("imessage", resolveIMessageSession)
	RegisterChannelSessionResolver("matrix", resolveMatrixSession)
	RegisterChannelSessionResolver("msteams", resolveMSTeamsSession)
	RegisterChannelSessionResolver("mattermost", resolveMattermostSession)
	RegisterChannelSessionResolver("nostr", resolveNostrSession)
	RegisterChannelSessionResolver("nextcloud-talk", resolveNextcloudTalkSession)
	RegisterChannelSessionResolver("zalo", resolveZaloSession)
	RegisterChannelSessionResolver("zalouser", resolveZalouserSession)
	RegisterChannelSessionResolver("tlon", resolveTlonSession)
	RegisterChannelSessionResolver("bluebubbles", resolveBlueBubblesSession)
}

// ----- WhatsApp -----
func resolveWhatsAppSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	target := strings.TrimSpace(p.Target)
	if target == "" {
		return nil
	}
	isGroup := strings.HasSuffix(target, "@g.us")
	kind := "direct"
	if isGroup {
		kind = "group"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "whatsapp", p.AccountID, kind, target, "main")
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: target, ChatType: kind, From: target, To: target}
}

// ----- Signal -----
func resolveSignalSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	stripped := stripProviderPrefix(p.Target, "signal")
	lower := strings.ToLower(stripped)
	if strings.HasPrefix(lower, "group:") {
		gid := strings.TrimSpace(stripped[6:])
		if gid == "" {
			return nil
		}
		base := kb.BuildAgentSessionKey(p.AgentID, "signal", p.AccountID, "group", gid, "main")
		return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: "group", PeerID: gid, ChatType: "group", From: "group:" + gid, To: "group:" + gid}
	}
	recipient := strings.TrimSpace(stripped)
	for _, pre := range []string{"username:", "u:"} {
		if strings.HasPrefix(lower, pre) {
			recipient = strings.TrimSpace(stripped[len(pre):])
			break
		}
	}
	if recipient == "" {
		return nil
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "signal", p.AccountID, "direct", recipient, "main")
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: "direct", PeerID: recipient, ChatType: "direct", From: "signal:" + recipient, To: "signal:" + recipient}
}

// ----- Telegram -----
func resolveTelegramSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	target := strings.TrimSpace(p.Target)
	if target == "" {
		return nil
	}
	// 简化: 负数 chatId 视为 group
	isGroup := strings.HasPrefix(target, "-") || strings.HasPrefix(strings.ToLower(target), "group:")
	chatID := stripKindPrefix(stripProviderPrefix(target, "telegram"))
	if chatID == "" {
		return nil
	}
	kind := "direct"
	if isGroup {
		kind = "group"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "telegram", p.AccountID, kind, chatID, "main")
	from := "telegram:" + chatID
	if isGroup {
		from = "telegram:group:" + chatID
	}
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: chatID, ChatType: kind, From: from, To: "telegram:" + chatID}
}

// ----- Discord -----
func resolveDiscordSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	target := stripProviderPrefix(p.Target, "discord")
	lower := strings.ToLower(target)
	isDM := strings.HasPrefix(lower, "user:")
	rawID := stripKindPrefix(target)
	if rawID == "" {
		return nil
	}
	kind := "channel"
	if isDM {
		kind = "direct"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "discord", p.AccountID, kind, rawID, "main")
	threadID := normalizeThreadID(p.ThreadID)
	if threadID == "" {
		threadID = normalizeThreadID(p.ReplyToID)
	}
	sk := base
	if threadID != "" {
		sk = kb.ResolveThreadSessionKey(base, threadID)
	}
	from := "discord:channel:" + rawID
	if isDM {
		from = "discord:" + rawID
	}
	to := "channel:" + rawID
	if isDM {
		to = "user:" + rawID
	}
	return &OutboundSessionRoute{SessionKey: sk, BaseSessionKey: base, PeerKind: kind, PeerID: rawID, ChatType: kind, From: from, To: to, ThreadID: threadID}
}

// ----- Slack -----
func resolveSlackSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	target := stripProviderPrefix(p.Target, "slack")
	lower := strings.ToLower(target)
	isDM := strings.HasPrefix(lower, "user:")
	rawID := stripKindPrefix(target)
	if rawID == "" {
		return nil
	}
	kind := "channel"
	if isDM {
		kind = "direct"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "slack", p.AccountID, kind, rawID, "main")
	threadID := normalizeThreadID(p.ThreadID)
	if threadID == "" {
		threadID = normalizeThreadID(p.ReplyToID)
	}
	sk := base
	if threadID != "" {
		sk = kb.ResolveThreadSessionKey(base, threadID)
	}
	from := "slack:channel:" + rawID
	if isDM {
		from = "slack:" + rawID
	}
	to := "channel:" + rawID
	if isDM {
		to = "user:" + rawID
	}
	return &OutboundSessionRoute{SessionKey: sk, BaseSessionKey: base, PeerKind: kind, PeerID: rawID, ChatType: kind, From: from, To: to, ThreadID: threadID}
}

// ----- iMessage -----
func resolveIMessageSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	target := strings.TrimSpace(p.Target)
	if target == "" {
		return nil
	}
	lower := strings.ToLower(target)
	isGroup := strings.HasPrefix(lower, "chat_id:") || strings.HasPrefix(lower, "chat_guid:") || strings.HasPrefix(lower, "chat_identifier:")
	peerId := stripKindPrefix(target)
	if peerId == "" {
		return nil
	}
	kind := "direct"
	if isGroup {
		kind = "group"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "imessage", p.AccountID, kind, peerId, "main")
	from := "imessage:" + peerId
	if isGroup {
		from = "imessage:group:" + peerId
	}
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: peerId, ChatType: kind, From: from, To: "imessage:" + peerId}
}

// ----- Matrix -----
func resolveMatrixSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	stripped := stripProviderPrefix(p.Target, "matrix")
	isUser := strings.HasPrefix(stripped, "@") || strings.HasPrefix(strings.ToLower(stripped), "user:")
	rawID := stripKindPrefix(stripped)
	if rawID == "" {
		return nil
	}
	kind := "channel"
	if isUser {
		kind = "direct"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "matrix", p.AccountID, kind, rawID, "main")
	from := "matrix:channel:" + rawID
	if isUser {
		from = "matrix:" + rawID
	}
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: rawID, ChatType: kind, From: from, To: "room:" + rawID}
}

// ----- MSTeams -----
func resolveMSTeamsSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	trimmed := strings.TrimSpace(p.Target)
	if trimmed == "" {
		return nil
	}
	for _, pre := range []string{"msteams:", "teams:"} {
		if strings.HasPrefix(strings.ToLower(trimmed), pre) {
			trimmed = strings.TrimSpace(trimmed[len(pre):])
		}
	}
	lower := strings.ToLower(trimmed)
	isUser := strings.HasPrefix(lower, "user:")
	rawID := stripKindPrefix(trimmed)
	if rawID == "" {
		return nil
	}
	parts := strings.SplitN(rawID, ";", 2)
	convID := parts[0]
	isChannel := !isUser && strings.Contains(strings.ToLower(convID), "@thread.tacv2")
	kind := "group"
	if isUser {
		kind = "direct"
	} else if isChannel {
		kind = "channel"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "msteams", p.AccountID, kind, convID, "main")
	from := "msteams:group:" + convID
	if isUser {
		from = "msteams:" + convID
	} else if isChannel {
		from = "msteams:channel:" + convID
	}
	to := "conversation:" + convID
	if isUser {
		to = "user:" + convID
	}
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: convID, ChatType: kind, From: from, To: to}
}

// ----- Mattermost -----
func resolveMattermostSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	trimmed := strings.TrimSpace(p.Target)
	if trimmed == "" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "mattermost:") {
		trimmed = strings.TrimSpace(trimmed[11:])
	}
	lower := strings.ToLower(trimmed)
	isUser := strings.HasPrefix(lower, "user:") || strings.HasPrefix(trimmed, "@")
	if strings.HasPrefix(trimmed, "@") {
		trimmed = trimmed[1:]
	}
	rawID := stripKindPrefix(trimmed)
	if rawID == "" {
		return nil
	}
	kind := "channel"
	if isUser {
		kind = "direct"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "mattermost", p.AccountID, kind, rawID, "main")
	threadID := normalizeThreadID(p.ReplyToID)
	if threadID == "" {
		threadID = normalizeThreadID(p.ThreadID)
	}
	sk := base
	if threadID != "" {
		sk = kb.ResolveThreadSessionKey(base, threadID)
	}
	from := "mattermost:channel:" + rawID
	if isUser {
		from = "mattermost:" + rawID
	}
	to := "channel:" + rawID
	if isUser {
		to = "user:" + rawID
	}
	return &OutboundSessionRoute{SessionKey: sk, BaseSessionKey: base, PeerKind: kind, PeerID: rawID, ChatType: kind, From: from, To: to, ThreadID: threadID}
}

// ----- Nostr -----
func resolveNostrSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	trimmed := stripProviderPrefix(p.Target, "nostr")
	if trimmed == "" {
		return nil
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "nostr", p.AccountID, "direct", trimmed, "main")
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: "direct", PeerID: trimmed, ChatType: "direct", From: "nostr:" + trimmed, To: "nostr:" + trimmed}
}

// ----- Nextcloud Talk -----
func resolveNextcloudTalkSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	trimmed := strings.TrimSpace(p.Target)
	if trimmed == "" {
		return nil
	}
	for _, pre := range []string{"nextcloud-talk:", "nc-talk:", "nc:", "room:"} {
		if strings.HasPrefix(strings.ToLower(trimmed), pre) {
			trimmed = strings.TrimSpace(trimmed[len(pre):])
		}
	}
	if trimmed == "" {
		return nil
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "nextcloud-talk", p.AccountID, "group", trimmed, "main")
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: "group", PeerID: trimmed, ChatType: "group", From: "nextcloud-talk:room:" + trimmed, To: "nextcloud-talk:" + trimmed}
}

// ----- Zalo -----
func resolveZaloSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	trimmed := stripProviderPrefix(p.Target, "zalo")
	if strings.HasPrefix(strings.ToLower(trimmed), "zl:") {
		trimmed = strings.TrimSpace(trimmed[3:])
	}
	if trimmed == "" {
		return nil
	}
	isGroup := strings.HasPrefix(strings.ToLower(trimmed), "group:")
	peerId := stripKindPrefix(trimmed)
	kind := "direct"
	if isGroup {
		kind = "group"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "zalo", p.AccountID, kind, peerId, "main")
	from := "zalo:" + peerId
	if isGroup {
		from = "zalo:group:" + peerId
	}
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: peerId, ChatType: kind, From: from, To: "zalo:" + peerId}
}

// ----- Zalouser -----
func resolveZalouserSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	trimmed := stripProviderPrefix(p.Target, "zalouser")
	if strings.HasPrefix(strings.ToLower(trimmed), "zlu:") {
		trimmed = strings.TrimSpace(trimmed[4:])
	}
	if trimmed == "" {
		return nil
	}
	isGroup := strings.HasPrefix(strings.ToLower(trimmed), "group:")
	peerId := stripKindPrefix(trimmed)
	kind := "direct"
	if isGroup {
		kind = "group"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "zalouser", p.AccountID, kind, peerId, "main")
	from := "zalouser:" + peerId
	if isGroup {
		from = "zalouser:group:" + peerId
	}
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: peerId, ChatType: kind, From: from, To: "zalouser:" + peerId}
}

// ----- Tlon -----
func resolveTlonSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	trimmed := stripProviderPrefix(p.Target, "tlon")
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	isGroup := strings.HasPrefix(lower, "group:") || strings.HasPrefix(lower, "room:") || strings.HasPrefix(lower, "chat/")
	peerId := trimmed
	if strings.HasPrefix(lower, "dm:") {
		peerId = strings.TrimSpace(trimmed[3:])
		if !strings.HasPrefix(peerId, "~") {
			peerId = "~" + peerId
		}
		isGroup = false
	} else if strings.HasPrefix(lower, "group:") || strings.HasPrefix(lower, "room:") {
		peerId = stripKindPrefix(trimmed)
		isGroup = true
	}
	kind := "direct"
	if isGroup {
		kind = "group"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "tlon", p.AccountID, kind, peerId, "main")
	from := "tlon:" + peerId
	if isGroup {
		from = "tlon:group:" + peerId
	}
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: peerId, ChatType: kind, From: from, To: "tlon:" + peerId}
}

// ----- BlueBubbles -----
func resolveBlueBubblesSession(p SessionResolveParams, kb SessionKeyBuilder) *OutboundSessionRoute {
	stripped := stripProviderPrefix(p.Target, "bluebubbles")
	lower := strings.ToLower(stripped)
	isGroup := strings.HasPrefix(lower, "chat_id:") || strings.HasPrefix(lower, "chat_guid:") || strings.HasPrefix(lower, "chat_identifier:") || strings.HasPrefix(lower, "group:")
	peerId := stripped
	if isGroup {
		peerId = stripKindPrefix(stripped)
		// 去除 chat_* 前缀对齐 inbound
		for _, pre := range []string{"chat_id:", "chat_guid:", "chat_identifier:"} {
			if strings.HasPrefix(strings.ToLower(peerId), pre) {
				peerId = peerId[len(pre):]
				break
			}
		}
	} else {
		for _, pre := range []string{"imessage:", "sms:", "auto:"} {
			if strings.HasPrefix(lower, pre) {
				peerId = stripped[len(pre):]
				break
			}
		}
	}
	if peerId == "" {
		return nil
	}
	kind := "direct"
	if isGroup {
		kind = "group"
	}
	base := kb.BuildAgentSessionKey(p.AgentID, "bluebubbles", p.AccountID, kind, peerId, "main")
	from := "bluebubbles:" + peerId
	if isGroup {
		from = "group:" + peerId
	}
	return &OutboundSessionRoute{SessionKey: base, BaseSessionKey: base, PeerKind: kind, PeerID: peerId, ChatType: kind, From: from, To: "bluebubbles:" + stripped}
}
