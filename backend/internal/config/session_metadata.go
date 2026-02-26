package config

// session_metadata.go — session 元数据衍生
// TS 参考: src/config/sessions/metadata.ts

import (
	"strings"
)

// ---------- Session 核心类型 ----------

// SessionScope 会话范围
type SessionScope string

const (
	SessionScopePerSender SessionScope = "per-sender"
	SessionScopeGlobal    SessionScope = "global"
)

// SessionChatType 会话聊天类型
type SessionChatType string

const (
	ChatTypePrivate SessionChatType = "private"
	ChatTypeGroup   SessionChatType = "group"
	ChatTypeChannel SessionChatType = "channel"
)

// SessionOrigin 会话来源信息
// TS 参考: src/config/sessions/types.ts SessionOrigin
type SessionOrigin struct {
	Label     string          `json:"label,omitempty"`
	Provider  string          `json:"provider,omitempty"`
	Surface   string          `json:"surface,omitempty"`
	ChatType  SessionChatType `json:"chatType,omitempty"`
	From      string          `json:"from,omitempty"`
	To        string          `json:"to,omitempty"`
	AccountId string          `json:"accountId,omitempty"`
	ThreadId  string          `json:"threadId,omitempty"` // 统一为 string（TS 为 string | number）
}

// SessionEntry 会话条目（核心字段子集，满足本模块需求）
// TS 参考: src/config/sessions/types.ts SessionEntry
type SessionEntry struct {
	SessionId    string          `json:"sessionId"`
	UpdatedAt    int64           `json:"updatedAt"`
	SessionFile  string          `json:"sessionFile,omitempty"`
	ChatType     SessionChatType `json:"chatType,omitempty"`
	Channel      string          `json:"channel,omitempty"`
	GroupId      string          `json:"groupId,omitempty"`
	Subject      string          `json:"subject,omitempty"`
	GroupChannel string          `json:"groupChannel,omitempty"`
	Space        string          `json:"space,omitempty"`
	DisplayName  string          `json:"displayName,omitempty"`
	Label        string          `json:"label,omitempty"`
	Origin       *SessionOrigin  `json:"origin,omitempty"`
}

// GroupKeyResolution 群组 key 解析结果
// TS 参考: src/config/sessions/types.ts GroupKeyResolution
type GroupKeyResolution struct {
	Key      string          `json:"key"`
	Channel  string          `json:"channel,omitempty"`
	ID       string          `json:"id,omitempty"`
	ChatType SessionChatType `json:"chatType,omitempty"`
}

// ---------- MsgContext 简化接口 ----------

// MsgContext 消息上下文（从 event/context 提取的字段）
// TS 参考: src/auto-reply/templating.ts MsgContext
type MsgContext struct {
	From               string
	To                 string
	OriginatingTo      string
	OriginatingChannel string
	Provider           string
	Surface            string
	ChatType           string
	AccountId          string
	MessageThreadId    string
	GroupSubject       string
	GroupSpace         string
	GroupChannel       string
	ConversationLabel  string
	SessionKey         string // 显式 session key 覆盖（TS: ctx.SessionKey）
}

// ---------- 内部辅助函数 ----------

// mergeOrigin 合并两个 SessionOrigin（next 字段优先覆盖 existing）
// TS 参考: metadata.ts mergeOrigin
func mergeOrigin(existing, next *SessionOrigin) *SessionOrigin {
	if existing == nil && next == nil {
		return nil
	}
	merged := &SessionOrigin{}
	if existing != nil {
		*merged = *existing
	}
	if next == nil {
		if isEmptyOrigin(merged) {
			return nil
		}
		return merged
	}
	if next.Label != "" {
		merged.Label = next.Label
	}
	if next.Provider != "" {
		merged.Provider = next.Provider
	}
	if next.Surface != "" {
		merged.Surface = next.Surface
	}
	if next.ChatType != "" {
		merged.ChatType = next.ChatType
	}
	if next.From != "" {
		merged.From = next.From
	}
	if next.To != "" {
		merged.To = next.To
	}
	if next.AccountId != "" {
		merged.AccountId = next.AccountId
	}
	if next.ThreadId != "" {
		merged.ThreadId = next.ThreadId
	}
	if isEmptyOrigin(merged) {
		return nil
	}
	return merged
}

func isEmptyOrigin(o *SessionOrigin) bool {
	return o == nil ||
		(o.Label == "" && o.Provider == "" && o.Surface == "" &&
			o.ChatType == "" && o.From == "" && o.To == "" &&
			o.AccountId == "" && o.ThreadId == "")
}

// normalizeMessageChannel 规范化消息渠道标识符（小写并去空格）
func normalizeMessageChannel(raw string) string {
	return strings.TrimSpace(strings.ToLower(raw))
}

// normalizeChatType 将 ChatType 字符串规范化为已知枚举
func normalizeChatType(raw string) SessionChatType {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "private":
		return ChatTypePrivate
	case "group":
		return ChatTypeGroup
	case "channel":
		return ChatTypeChannel
	default:
		return ""
	}
}

// ---------- 公开函数 ----------

// DeriveSessionOrigin 从 MsgContext 衍生 SessionOrigin。
// TS 参考: src/config/sessions/metadata.ts deriveSessionOrigin
func DeriveSessionOrigin(ctx MsgContext) *SessionOrigin {
	label := strings.TrimSpace(ctx.ConversationLabel)

	providerRaw := ctx.OriginatingChannel
	if providerRaw == "" {
		providerRaw = ctx.Surface
	}
	if providerRaw == "" {
		providerRaw = ctx.Provider
	}
	provider := normalizeMessageChannel(providerRaw)
	surface := strings.TrimSpace(strings.ToLower(ctx.Surface))
	chatType := normalizeChatType(ctx.ChatType)

	from := strings.TrimSpace(ctx.From)

	to := strings.TrimSpace(ctx.OriginatingTo)
	if to == "" {
		to = strings.TrimSpace(ctx.To)
	}

	accountId := strings.TrimSpace(ctx.AccountId)
	threadId := ctx.MessageThreadId

	origin := &SessionOrigin{}
	if label != "" {
		origin.Label = label
	}
	if provider != "" {
		origin.Provider = provider
	}
	if surface != "" {
		origin.Surface = surface
	}
	if chatType != "" {
		origin.ChatType = chatType
	}
	if from != "" {
		origin.From = from
	}
	if to != "" {
		origin.To = to
	}
	if accountId != "" {
		origin.AccountId = accountId
	}
	if threadId != "" {
		origin.ThreadId = threadId
	}

	if isEmptyOrigin(origin) {
		return nil
	}
	return origin
}

// SnapshotSessionOrigin 快照 SessionEntry 的 origin 字段（浅拷贝）。
// TS 参考: metadata.ts snapshotSessionOrigin
func SnapshotSessionOrigin(entry *SessionEntry) *SessionOrigin {
	if entry == nil || entry.Origin == nil {
		return nil
	}
	cp := *entry.Origin
	return &cp
}

// DeriveGroupSessionPatch 从 MsgContext 衍生群组相关的 SessionEntry 补丁。
// TS 参考: metadata.ts deriveGroupSessionPatch
func DeriveGroupSessionPatch(params struct {
	Ctx             MsgContext
	SessionKey      string
	Existing        *SessionEntry
	GroupResolution *GroupKeyResolution
}) *SessionEntry {
	resolution := params.GroupResolution
	if resolution == nil {
		resolution = ResolveGroupSessionKey(params.Ctx)
	}
	if resolution == nil || resolution.Channel == "" {
		return nil
	}

	channel := resolution.Channel
	subject := strings.TrimSpace(params.Ctx.GroupSubject)
	space := strings.TrimSpace(params.Ctx.GroupSpace)
	explicitChannel := strings.TrimSpace(params.Ctx.GroupChannel)

	isChannelProvider := resolution.ChatType == ChatTypeChannel
	nextGroupChannel := explicitChannel
	if nextGroupChannel == "" &&
		(resolution.ChatType == ChatTypeChannel || isChannelProvider) &&
		subject != "" && strings.HasPrefix(subject, "#") {
		nextGroupChannel = subject
	}

	nextSubject := subject
	if nextGroupChannel != "" {
		nextSubject = ""
	}

	chatType := resolution.ChatType
	if chatType == "" {
		chatType = ChatTypeGroup
	}

	patch := &SessionEntry{
		ChatType: chatType,
		Channel:  channel,
		GroupId:  resolution.ID,
	}
	if nextSubject != "" {
		patch.Subject = nextSubject
	}
	if nextGroupChannel != "" {
		patch.GroupChannel = nextGroupChannel
	}
	if space != "" {
		patch.Space = space
	}

	existingSubject := ""
	existingGroupChannel := ""
	existingSpace := ""
	if params.Existing != nil {
		existingSubject = params.Existing.Subject
		existingGroupChannel = params.Existing.GroupChannel
		existingSpace = params.Existing.Space
	}

	subjectForDisplay := nextSubject
	if subjectForDisplay == "" {
		subjectForDisplay = existingSubject
	}
	groupChannelForDisplay := nextGroupChannel
	if groupChannelForDisplay == "" {
		groupChannelForDisplay = existingGroupChannel
	}
	spaceForDisplay := space
	if spaceForDisplay == "" {
		spaceForDisplay = existingSpace
	}

	displayName := BuildGroupDisplayName(BuildGroupDisplayNameParams{
		Provider:     channel,
		Subject:      subjectForDisplay,
		GroupChannel: groupChannelForDisplay,
		Space:        spaceForDisplay,
		ID:           resolution.ID,
		Key:          params.SessionKey,
	})
	if displayName != "" {
		patch.DisplayName = displayName
	}

	return patch
}

// DeriveSessionMetaPatch 从 MsgContext 衍生完整 session 元数据补丁。
// TS 参考: metadata.ts deriveSessionMetaPatch
func DeriveSessionMetaPatch(params struct {
	Ctx             MsgContext
	SessionKey      string
	Existing        *SessionEntry
	GroupResolution *GroupKeyResolution
}) *SessionEntry {
	groupPatch := DeriveGroupSessionPatch(params)
	origin := DeriveSessionOrigin(params.Ctx)
	if groupPatch == nil && origin == nil {
		return nil
	}

	var patch SessionEntry
	if groupPatch != nil {
		patch = *groupPatch
	}

	var existingOrigin *SessionOrigin
	if params.Existing != nil {
		existingOrigin = params.Existing.Origin
	}
	mergedOrigin := mergeOrigin(existingOrigin, origin)
	if mergedOrigin != nil {
		patch.Origin = mergedOrigin
	}

	return &patch
}
