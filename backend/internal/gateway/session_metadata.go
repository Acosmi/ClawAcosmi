package gateway

import (
	"strings"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/internal/session"
)

// TS 对照: config/sessions/metadata.ts (173L)
// 会话元数据推导链: origin/group/meta patch 构建。

// ---------- Origin 合并 ----------

// mergeOrigin 合并两个 SessionOrigin（next 字段覆盖 existing 空字段）。
// TS 对照: metadata.ts L10-43
func mergeOrigin(existing, next *session.SessionOrigin) *session.SessionOrigin {
	if existing == nil && next == nil {
		return nil
	}
	var merged session.SessionOrigin
	if existing != nil {
		merged = *existing
	}
	if next == nil {
		return &merged
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
	if next.ThreadId != nil {
		s := strings.TrimSpace(stringifyThreadId(next.ThreadId))
		if s != "" {
			merged.ThreadId = next.ThreadId
		}
	}

	// 空检查
	if isEmpty := merged.Label == "" && merged.Provider == "" && merged.Surface == "" &&
		merged.ChatType == "" && merged.From == "" && merged.To == "" &&
		merged.AccountId == "" && merged.ThreadId == nil; isEmpty {
		return nil
	}
	return &merged
}

// stringifyThreadId 将 threadId 转为字符串用于判空。
func stringifyThreadId(tid interface{}) string {
	if tid == nil {
		return ""
	}
	switch v := tid.(type) {
	case string:
		return v
	default:
		_ = v
		return ""
	}
}

// DeriveSessionOrigin 从入站消息上下文推导 SessionOrigin。
// TS 对照: metadata.ts L45-87
func DeriveSessionOrigin(ctx *autoreply.MsgContext) *session.SessionOrigin {
	if ctx == nil {
		return nil
	}

	label := strings.TrimSpace(ctx.ConversationLabel)
	providerRaw := ctx.OriginatingChannel
	if providerRaw == "" {
		providerRaw = ctx.Surface
	}
	if providerRaw == "" {
		providerRaw = ctx.Provider
	}
	provider := NormalizeMessageChannel(providerRaw)
	surface := strings.ToLower(strings.TrimSpace(ctx.Surface))
	chatType := NormalizeChatType(ctx.ChatType)
	from := strings.TrimSpace(ctx.From)
	to := strings.TrimSpace(ctx.OriginatingTo)
	if to == "" {
		to = strings.TrimSpace(ctx.To)
	}
	accountId := strings.TrimSpace(ctx.AccountID)
	threadId := ctx.MessageThreadID

	origin := &session.SessionOrigin{}
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

	// 空检查
	if origin.Label == "" && origin.Provider == "" && origin.Surface == "" &&
		origin.ChatType == "" && origin.From == "" && origin.To == "" &&
		origin.AccountId == "" && origin.ThreadId == nil {
		return nil
	}
	return origin
}

// SnapshotSessionOrigin 快照 session origin。
// TS 对照: metadata.ts L89-94
func SnapshotSessionOrigin(entry *SessionEntry) *session.SessionOrigin {
	if entry == nil || entry.Origin == nil {
		return nil
	}
	copy := *entry.Origin
	return &copy
}

// SessionMetaPatch 会话元数据 patch。
type SessionMetaPatch struct {
	ChatType     string
	Channel      string
	GroupId      string
	Subject      string
	GroupChannel string
	Space        string
	DisplayName  string
	Origin       *session.SessionOrigin
}

// DeriveSessionMetaPatch 推导完整的会话元数据 patch。
// TS 对照: metadata.ts L153-172
func DeriveSessionMetaPatch(ctx *autoreply.MsgContext, sessionKey string, existing *SessionEntry) *SessionMetaPatch {
	if ctx == nil {
		return nil
	}

	origin := DeriveSessionOrigin(ctx)

	var existingOrigin *session.SessionOrigin
	if existing != nil {
		existingOrigin = existing.Origin
	}

	mergedOrigin := mergeOrigin(existingOrigin, origin)

	// 简化 group patch（完整的 groupResolution 需要 channels 包支持，此处用简化逻辑）
	hasGroupInfo := ctx.IsGroup || ctx.ChatType == "group" || ctx.ChatType == "channel"

	if !hasGroupInfo && mergedOrigin == nil {
		return nil
	}

	patch := &SessionMetaPatch{}

	if hasGroupInfo {
		chatType := NormalizeChatType(ctx.ChatType)
		if chatType == "" {
			chatType = "group"
		}
		patch.ChatType = chatType

		channel := ctx.ChannelType
		if channel == "" {
			channel = ctx.Provider
		}
		patch.Channel = channel

		if ctx.GroupChannel != "" {
			patch.GroupChannel = strings.TrimSpace(ctx.GroupChannel)
		}

		subject := strings.TrimSpace(ctx.GroupSubject)
		if subject != "" {
			// 如果 subject 以 # 开头且为频道类型，归入 groupChannel
			if patch.ChatType == "channel" && strings.HasPrefix(subject, "#") {
				patch.GroupChannel = subject
			} else {
				patch.Subject = subject
			}
		}
	}

	if mergedOrigin != nil {
		patch.Origin = mergedOrigin
	}

	return patch
}

// ---------- 辅助函数 ----------

// NormalizeMessageChannel 规范化消息频道名。
// TS 对照: utils/message-channel.ts normalizeMessageChannel
func NormalizeMessageChannel(ch string) string {
	return strings.ToLower(strings.TrimSpace(ch))
}

// NormalizeChatType 规范化聊天类型。
// TS 对照: channels/chat-type.ts normalizeChatType
func NormalizeChatType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	switch ct {
	case "group", "channel":
		return ct
	case "supergroup":
		return "group"
	default:
		return ""
	}
}
