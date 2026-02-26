// Package sessions — 会话元数据推导。
//
// 对齐 TS: src/config/sessions/metadata.ts (173L)
package sessions

import (
	"strings"
)

// ---------- Origin 合并 ----------

// MergeOrigin 合并两个 SessionOrigin，next 的非空字段覆盖 existing。
// 对齐 TS: metadata.ts mergeOrigin()
func MergeOrigin(existing, next *SessionOrigin) *SessionOrigin {
	if existing == nil && next == nil {
		return nil
	}
	merged := &SessionOrigin{}
	if existing != nil {
		*merged = *existing
	}
	if next == nil {
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
	if next.AccountID != "" {
		merged.AccountID = next.AccountID
	}
	if next.ThreadID != nil && next.ThreadID != "" {
		merged.ThreadID = next.ThreadID
	}
	return merged
}

// ---------- 元数据推导 ----------

// MetadataContext 用于元数据推导的消息上下文。
type MetadataContext struct {
	From               string
	ChatType           string
	Provider           string
	Surface            string
	To                 string
	OriginatingTo      string
	OriginatingChannel string
	AccountID          string
	MessageThreadID    interface{}
	GroupSubject       string
	GroupSpace         string
	GroupChannel       string
	SessionKey         string
}

// DeriveSessionOrigin 从消息上下文推导会话来源信息。
// 对齐 TS: metadata.ts deriveSessionOrigin()
func DeriveSessionOrigin(ctx MetadataContext) *SessionOrigin {
	provider := strings.ToLower(strings.TrimSpace(ctx.Provider))
	if ocStr := strings.TrimSpace(ctx.OriginatingChannel); ocStr != "" {
		provider = strings.ToLower(ocStr)
	}
	surface := strings.ToLower(strings.TrimSpace(ctx.Surface))
	if provider == "" && surface != "" {
		provider = surface
	}

	chatType := strings.ToLower(strings.TrimSpace(ctx.ChatType))
	from := strings.TrimSpace(ctx.From)

	to := strings.TrimSpace(ctx.OriginatingTo)
	if to == "" {
		to = strings.TrimSpace(ctx.To)
	}
	accountID := strings.TrimSpace(ctx.AccountID)

	origin := &SessionOrigin{}
	hasFields := false

	if provider != "" {
		origin.Provider = provider
		hasFields = true
	}
	if surface != "" {
		origin.Surface = surface
		hasFields = true
	}
	if chatType != "" {
		origin.ChatType = chatType
		hasFields = true
	}
	if from != "" {
		origin.From = from
		hasFields = true
	}
	if to != "" {
		origin.To = to
		hasFields = true
	}
	if accountID != "" {
		origin.AccountID = accountID
		hasFields = true
	}
	if ctx.MessageThreadID != nil {
		switch v := ctx.MessageThreadID.(type) {
		case string:
			if v != "" {
				origin.ThreadID = v
				hasFields = true
			}
		default:
			origin.ThreadID = v
			hasFields = true
		}
	}

	if !hasFields {
		return nil
	}
	return origin
}

// SnapshotSessionOrigin 快照会话来源（深拷贝）。
func SnapshotSessionOrigin(entry *FullSessionEntry) *SessionOrigin {
	if entry == nil || entry.Origin == nil {
		return nil
	}
	copy := *entry.Origin
	return &copy
}

// ---------- 群组元数据补丁 ----------

// DeriveGroupSessionPatch 推导群组会话的补丁。
// 对齐 TS: metadata.ts deriveGroupSessionPatch()
func DeriveGroupSessionPatch(ctx MetadataContext, sessionKey string, existing *FullSessionEntry, groupResolution *GroupKeyResolution) map[string]interface{} {
	resolution := groupResolution
	if resolution == nil {
		resolution = ResolveGroupSessionKey(MsgContextForGroup{
			From:     ctx.From,
			ChatType: ctx.ChatType,
			Provider: ctx.Provider,
		})
	}
	if resolution == nil || resolution.Channel == "" {
		return nil
	}

	channel := resolution.Channel
	subject := strings.TrimSpace(ctx.GroupSubject)
	space := strings.TrimSpace(ctx.GroupSpace)
	explicitChannel := strings.TrimSpace(ctx.GroupChannel)

	nextGroupChannel := explicitChannel
	if nextGroupChannel == "" {
		if (resolution.ChatType == "channel") && subject != "" && strings.HasPrefix(subject, "#") {
			nextGroupChannel = subject
		}
	}
	nextSubject := subject
	if nextGroupChannel != "" {
		nextSubject = ""
	}

	patch := map[string]interface{}{
		"chatType": resolution.ChatType,
		"channel":  channel,
		"groupId":  resolution.ID,
	}
	if nextSubject != "" {
		patch["subject"] = nextSubject
	}
	if nextGroupChannel != "" {
		patch["groupChannel"] = nextGroupChannel
	}
	if space != "" {
		patch["space"] = space
	}

	existingSubject := ""
	existingGroupChannel := ""
	existingSpace := ""
	if existing != nil {
		existingSubject = existing.Subject
		existingGroupChannel = existing.GroupChannel
		existingSpace = existing.Space
	}

	displayName := BuildGroupDisplayName(GroupDisplayNameParams{
		Provider:     channel,
		Subject:      firstNonEmpty(nextSubject, existingSubject),
		GroupChannel: firstNonEmpty(nextGroupChannel, existingGroupChannel),
		Space:        firstNonEmpty(space, existingSpace),
		ID:           resolution.ID,
		Key:          sessionKey,
	})
	if displayName != "" {
		patch["displayName"] = displayName
	}

	return patch
}

// DeriveSessionMetaPatch 推导完整的会话元数据补丁。
// 对齐 TS: metadata.ts deriveSessionMetaPatch()
func DeriveSessionMetaPatch(ctx MetadataContext, sessionKey string, existing *FullSessionEntry, groupResolution *GroupKeyResolution) map[string]interface{} {
	groupPatch := DeriveGroupSessionPatch(ctx, sessionKey, existing, groupResolution)
	origin := DeriveSessionOrigin(ctx)

	if groupPatch == nil && origin == nil {
		return nil
	}

	patch := make(map[string]interface{})
	for k, v := range groupPatch {
		patch[k] = v
	}

	var existingOrigin *SessionOrigin
	if existing != nil {
		existingOrigin = existing.Origin
	}
	mergedOrigin := MergeOrigin(existingOrigin, origin)
	if mergedOrigin != nil {
		patch["origin"] = mergedOrigin
	}

	if len(patch) > 0 {
		return patch
	}
	return nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
