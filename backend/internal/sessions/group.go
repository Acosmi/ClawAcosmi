// Package sessions — 群组会话键解析。
//
// 对齐 TS: src/config/sessions/group.ts (113L)
package sessions

import (
	"regexp"
	"strings"
)

// ---------- 群组表面列表 ----------

// getGroupSurfaces 返回已知的可投递消息频道名 + "webchat"。
// 对齐 TS: group.ts getGroupSurfaces()
func getGroupSurfaces() map[string]bool {
	// 硬编码常见频道（与 TS listDeliverableMessageChannels() 对齐）
	return map[string]bool{
		"telegram":  true,
		"discord":   true,
		"slack":     true,
		"whatsapp":  true,
		"signal":    true,
		"imessage":  true,
		"line":      true,
		"webchat":   true,
		"instagram": true,
		"messenger": true,
	}
}

// ---------- 标签规范化 ----------

var (
	groupLabelSpacesRE    = regexp.MustCompile(`\s+`)
	groupLabelInvalidRE   = regexp.MustCompile(`[^a-z0-9#@._+\-]+`)
	groupLabelMultiDashRE = regexp.MustCompile(`-{2,}`)
	groupLabelTrimRE      = regexp.MustCompile(`^[.\-]+|[.\-]+$`)
)

// normalizeGroupLabel 规范化群组标签。
func normalizeGroupLabel(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return ""
	}
	dashed := groupLabelSpacesRE.ReplaceAllString(trimmed, "-")
	cleaned := groupLabelInvalidRE.ReplaceAllString(dashed, "-")
	collapsed := groupLabelMultiDashRE.ReplaceAllString(cleaned, "-")
	return groupLabelTrimRE.ReplaceAllString(collapsed, "")
}

// shortenGroupID 截短过长的群组 ID。
func shortenGroupID(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 14 {
		return trimmed
	}
	return trimmed[:6] + "..." + trimmed[len(trimmed)-4:]
}

// ---------- 显示名构建 ----------

// BuildGroupDisplayName 构建群组显示名。
// 对齐 TS: group.ts buildGroupDisplayName()
func BuildGroupDisplayName(params GroupDisplayNameParams) string {
	providerKey := strings.TrimSpace(strings.ToLower(strings.TrimSpace(params.Provider)))
	if providerKey == "" {
		providerKey = "group"
	}
	groupChannel := strings.TrimSpace(params.GroupChannel)
	space := strings.TrimSpace(params.Space)
	subject := strings.TrimSpace(params.Subject)

	var detail string
	if groupChannel != "" && space != "" {
		prefix := ""
		if !strings.HasPrefix(groupChannel, "#") {
			prefix = "#"
		}
		detail = space + prefix + groupChannel
	} else if groupChannel != "" {
		detail = groupChannel
	} else if subject != "" {
		detail = subject
	} else if space != "" {
		detail = space
	}

	fallbackID := strings.TrimSpace(params.ID)
	if fallbackID == "" {
		fallbackID = params.Key
	}
	rawLabel := detail
	if rawLabel == "" {
		rawLabel = fallbackID
	}

	token := normalizeGroupLabel(rawLabel)
	if token == "" {
		token = normalizeGroupLabel(shortenGroupID(rawLabel))
	}
	if params.GroupChannel == "" && strings.HasPrefix(token, "#") {
		token = strings.TrimLeft(token, "#")
	}
	if token != "" && !strings.HasPrefix(token, "@") && !strings.HasPrefix(token, "#") &&
		!strings.HasPrefix(token, "g-") && !strings.Contains(token, "#") {
		token = "g-" + token
	}
	if token != "" {
		return providerKey + ":" + token
	}
	return providerKey
}

// GroupDisplayNameParams 构建群组显示名的参数。
type GroupDisplayNameParams struct {
	Provider     string
	Subject      string
	GroupChannel string
	Space        string
	ID           string
	Key          string
}

// ---------- 群组会话键解析 ----------

// GroupKeyResolution 群组会话键解析结果。
type GroupKeyResolution struct {
	Key      string
	Channel  string
	ID       string
	ChatType string // "group" | "channel"
}

// MsgContextForGroup 用于群组解析的消息上下文（最小接口）。
type MsgContextForGroup struct {
	From     string
	ChatType string
	Provider string
}

// ResolveGroupSessionKey 从消息上下文解析群组会话键。
// 对齐 TS: group.ts resolveGroupSessionKey()
func ResolveGroupSessionKey(ctx MsgContextForGroup) *GroupKeyResolution {
	from := strings.TrimSpace(ctx.From)
	chatType := strings.ToLower(strings.TrimSpace(ctx.ChatType))

	var normalizedChatType string
	switch chatType {
	case "channel":
		normalizedChatType = "channel"
	case "group":
		normalizedChatType = "group"
	}

	isWhatsAppGroupID := strings.HasSuffix(strings.ToLower(from), "@g.us")
	looksLikeGroup := normalizedChatType == "group" ||
		normalizedChatType == "channel" ||
		strings.Contains(from, ":group:") ||
		strings.Contains(from, ":channel:") ||
		isWhatsAppGroupID

	if !looksLikeGroup {
		return nil
	}

	providerHint := strings.ToLower(strings.TrimSpace(ctx.Provider))
	surfaces := getGroupSurfaces()

	parts := splitFilterEmpty(from, ":")
	head := ""
	if len(parts) > 0 {
		head = strings.ToLower(strings.TrimSpace(parts[0]))
	}
	headIsSurface := head != "" && surfaces[head]

	provider := ""
	if headIsSurface {
		provider = head
	} else if providerHint != "" {
		provider = providerHint
	} else if isWhatsAppGroupID {
		provider = "whatsapp"
	}
	if provider == "" {
		return nil
	}

	second := ""
	if len(parts) > 1 {
		second = strings.ToLower(strings.TrimSpace(parts[1]))
	}
	secondIsKind := second == "group" || second == "channel"

	kind := "group"
	if secondIsKind {
		kind = second
	} else if strings.Contains(from, ":channel:") || normalizedChatType == "channel" {
		kind = "channel"
	}

	var id string
	if headIsSurface {
		if secondIsKind {
			id = strings.Join(parts[2:], ":")
		} else {
			id = strings.Join(parts[1:], ":")
		}
	} else {
		id = from
	}
	finalID := strings.ToLower(strings.TrimSpace(id))
	if finalID == "" {
		return nil
	}

	resultChatType := "group"
	if kind == "channel" {
		resultChatType = "channel"
	}

	return &GroupKeyResolution{
		Key:      provider + ":" + kind + ":" + finalID,
		Channel:  provider,
		ID:       finalID,
		ChatType: resultChatType,
	}
}

// splitFilterEmpty 按分隔符拆分并过滤空字符串。
func splitFilterEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
