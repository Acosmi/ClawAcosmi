package config

// session_group.go — 群组 session key 解析
// TS 参考: src/config/sessions/group.ts

import (
	"fmt"
	"strings"
)

// ---------- 内部辅助 ----------

// groupSurfaces 常见群组渠道标识（对应 TS listDeliverableMessageChannels + "webchat"）
var groupSurfaces = map[string]struct{}{
	"telegram":  {},
	"whatsapp":  {},
	"discord":   {},
	"slack":     {},
	"line":      {},
	"messenger": {},
	"instagram": {},
	"twitter":   {},
	"wechat":    {},
	"viber":     {},
	"signal":    {},
	"matrix":    {},
	"webchat":   {},
}

func isGroupSurface(s string) bool {
	_, ok := groupSurfaces[s]
	return ok
}

// normalizeGroupLabel 规范化群组标签（小写、空格转连字符、去除非法字符）
// TS 参考: group.ts normalizeGroupLabel
func normalizeGroupLabel(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ""
	}
	// 空格 → -
	dashed := strings.Join(strings.Fields(trimmed), "-")
	// 只保留合法字符
	var buf strings.Builder
	for _, r := range dashed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
			r == '#' || r == '@' || r == '.' || r == '_' || r == '+' || r == '-' {
			buf.WriteRune(r)
		} else {
			buf.WriteRune('-')
		}
	}
	s := buf.String()
	// 合并多连字符
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	// 去首尾 - 和 .
	s = strings.TrimLeft(s, "-.")
	s = strings.TrimRight(s, "-.")
	return s
}

// shortenGroupId 截短过长的群组 ID（超过 14 字符时取头 6 + ... + 尾 4）
// TS 参考: group.ts shortenGroupId
func shortenGroupId(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if len([]rune(trimmed)) <= 14 {
		return trimmed
	}
	runes := []rune(trimmed)
	return string(runes[:6]) + "..." + string(runes[len(runes)-4:])
}

// ---------- BuildGroupDisplayNameParams ----------

// BuildGroupDisplayNameParams BuildGroupDisplayName 的参数
type BuildGroupDisplayNameParams struct {
	Provider     string
	Subject      string
	GroupChannel string
	Space        string
	ID           string
	Key          string
}

// BuildGroupDisplayName 构建群组的显示名称。
// TS 参考: src/config/sessions/group.ts buildGroupDisplayName
func BuildGroupDisplayName(p BuildGroupDisplayNameParams) string {
	providerKey := strings.TrimSpace(strings.ToLower(p.Provider))
	if providerKey == "" {
		providerKey = "group"
	}

	groupChannel := strings.TrimSpace(p.GroupChannel)
	space := strings.TrimSpace(p.Space)
	subject := strings.TrimSpace(p.Subject)

	var detail string
	if groupChannel != "" && space != "" {
		sep := ""
		if !strings.HasPrefix(groupChannel, "#") {
			sep = "#"
		}
		detail = fmt.Sprintf("%s%s%s", space, sep, groupChannel)
	} else if groupChannel != "" {
		detail = groupChannel
	} else if subject != "" {
		detail = subject
	} else if space != "" {
		detail = space
	}

	fallbackId := strings.TrimSpace(p.ID)
	if fallbackId == "" {
		fallbackId = p.Key
	}
	rawLabel := detail
	if rawLabel == "" {
		rawLabel = fallbackId
	}

	token := normalizeGroupLabel(rawLabel)
	if token == "" {
		token = normalizeGroupLabel(shortenGroupId(rawLabel))
	}

	// 若无 groupChannel 指定且 token 以 # 开头，去除 #
	if p.GroupChannel == "" && strings.HasPrefix(token, "#") {
		token = strings.TrimLeft(token, "#")
	}

	// 不以 @ 或 # 开头、不以 g- 开头、不含 # 时，加 g- 前缀
	if token != "" &&
		!strings.HasPrefix(token, "@") &&
		!strings.HasPrefix(token, "#") &&
		!strings.HasPrefix(token, "g-") &&
		!strings.Contains(token, "#") {
		token = "g-" + token
	}

	if token != "" {
		return fmt.Sprintf("%s:%s", providerKey, token)
	}
	return providerKey
}

// ---------- ResolveGroupSessionKey ----------

// ResolveGroupSessionKey 从消息上下文解析群组 session key。
// 返回 nil 表示不是群组会话。
// TS 参考: src/config/sessions/group.ts resolveGroupSessionKey
func ResolveGroupSessionKey(ctx MsgContext) *GroupKeyResolution {
	from := strings.TrimSpace(ctx.From)
	chatTypeRaw := strings.TrimSpace(strings.ToLower(ctx.ChatType))

	var normalizedChatType SessionChatType
	switch chatTypeRaw {
	case "channel":
		normalizedChatType = ChatTypeChannel
	case "group":
		normalizedChatType = ChatTypeGroup
	}

	isWhatsAppGroupId := strings.HasSuffix(strings.ToLower(from), "@g.us")
	looksLikeGroup := normalizedChatType == ChatTypeGroup ||
		normalizedChatType == ChatTypeChannel ||
		strings.Contains(from, ":group:") ||
		strings.Contains(from, ":channel:") ||
		isWhatsAppGroupId

	if !looksLikeGroup {
		return nil
	}

	providerHint := strings.TrimSpace(strings.ToLower(ctx.Provider))

	parts := splitNonEmpty(from, ":")
	head := ""
	if len(parts) > 0 {
		head = strings.TrimSpace(strings.ToLower(parts[0]))
	}
	headIsSurface := head != "" && isGroupSurface(head)

	provider := ""
	if headIsSurface {
		provider = head
	} else {
		if providerHint != "" {
			provider = providerHint
		} else if isWhatsAppGroupId {
			provider = "whatsapp"
		}
	}
	if provider == "" {
		return nil
	}

	var second string
	if len(parts) > 1 {
		second = strings.TrimSpace(strings.ToLower(parts[1]))
	}
	secondIsKind := second == "group" || second == "channel"

	kind := ""
	if secondIsKind {
		kind = second
	} else if strings.Contains(from, ":channel:") || normalizedChatType == ChatTypeChannel {
		kind = "channel"
	} else {
		kind = "group"
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
	finalId := strings.TrimSpace(strings.ToLower(id))
	if finalId == "" {
		return nil
	}

	ct := ChatTypeGroup
	if kind == "channel" {
		ct = ChatTypeChannel
	}

	return &GroupKeyResolution{
		Key:      fmt.Sprintf("%s:%s:%s", provider, kind, finalId),
		Channel:  provider,
		ID:       finalId,
		ChatType: ct,
	}
}

// splitNonEmpty 按 sep 分割字符串，过滤空串
func splitNonEmpty(s, sep string) []string {
	raw := strings.Split(s, sep)
	var result []string
	for _, r := range raw {
		if r != "" {
			result = append(result, r)
		}
	}
	return result
}
