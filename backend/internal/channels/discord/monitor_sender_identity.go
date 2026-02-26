package discord

import "strings"

// Discord 发送者身份识别 — 继承自 src/discord/monitor/sender-identity.ts (83L)

// DiscordSenderIdentity 发送者身份
type DiscordSenderIdentity struct {
	ID          string                      `json:"id"`
	Name        string                      `json:"name,omitempty"`
	Tag         string                      `json:"tag,omitempty"`
	Label       string                      `json:"label"`
	IsPluralKit bool                        `json:"isPluralKit"`
	PluralKit   *DiscordSenderPluralKitInfo `json:"pluralkit,omitempty"`
}

// DiscordSenderPluralKitInfo PluralKit 信息
type DiscordSenderPluralKitInfo struct {
	MemberID   string `json:"memberId"`
	MemberName string `json:"memberName,omitempty"`
	SystemID   string `json:"systemId,omitempty"`
	SystemName string `json:"systemName,omitempty"`
}

// DiscordAuthorInfo 作者基本信息（替代 @buape/carbon User）
type DiscordAuthorInfo struct {
	ID            string `json:"id"`
	Username      string `json:"username,omitempty"`
	GlobalName    string `json:"global_name,omitempty"`
	Discriminator string `json:"discriminator,omitempty"`
}

// DiscordMemberInfo 成员信息
type DiscordMemberInfo struct {
	Nickname string `json:"nick,omitempty"`
}

// ResolveDiscordWebhookID 从消息中解析 webhook ID。
// W-030 fix: 对齐 TS resolveDiscordWebhookId — 检查 webhookId 和 webhook_id 双字段。
// TS ref: const candidate = message.webhookId ?? message.webhook_id;
// 在 Go 中 discordgo 的 Message.WebhookID 映射 json:"webhook_id"，
// 但调用方可能传入来自不同 JSON 格式的值（如 camelCase webhookId），
// 因此接受两个候选值并返回第一个非空值。
func ResolveDiscordWebhookID(webhookID string, fallbackWebhookIDs ...string) string {
	if s := strings.TrimSpace(webhookID); s != "" {
		return s
	}
	// 检查备选字段（对齐 TS: message.webhookId ?? message.webhook_id）
	for _, fb := range fallbackWebhookIDs {
		if s := strings.TrimSpace(fb); s != "" {
			return s
		}
	}
	return ""
}

// ResolveDiscordSenderIdentity 解析发送者身份
func ResolveDiscordSenderIdentity(author DiscordAuthorInfo, member *DiscordMemberInfo, pkInfo *PluralKitMessageInfo) DiscordSenderIdentity {
	// PluralKit 处理
	if pkInfo != nil && pkInfo.Member != nil {
		memberID := strings.TrimSpace(pkInfo.Member.ID)
		memberName := ""
		if pkInfo.Member.DisplayName != nil {
			memberName = strings.TrimSpace(*pkInfo.Member.DisplayName)
		}
		if memberName == "" && pkInfo.Member.Name != nil {
			memberName = strings.TrimSpace(*pkInfo.Member.Name)
		}
		if memberID != "" && memberName != "" {
			systemName := ""
			if pkInfo.System != nil && pkInfo.System.Name != nil {
				systemName = strings.TrimSpace(*pkInfo.System.Name)
			}
			label := memberName + " (PK)"
			if systemName != "" {
				label = memberName + " (PK:" + systemName + ")"
			}
			pkInfoResult := &DiscordSenderPluralKitInfo{
				MemberID:   memberID,
				MemberName: memberName,
			}
			if pkInfo.System != nil {
				if pkInfo.System.ID != "" {
					pkInfoResult.SystemID = strings.TrimSpace(pkInfo.System.ID)
				}
				pkInfoResult.SystemName = systemName
			}
			tag := ""
			if pkInfo.Member.Name != nil {
				tag = strings.TrimSpace(*pkInfo.Member.Name)
			}
			return DiscordSenderIdentity{
				ID: memberID, Name: memberName, Tag: tag,
				Label: label, IsPluralKit: true, PluralKit: pkInfoResult,
			}
		}
	}

	// 普通用户
	senderTag := FormatDiscordUserTag(author.Username, author.Discriminator, author.ID)
	senderDisplay := ""
	if member != nil && member.Nickname != "" {
		senderDisplay = member.Nickname
	} else if author.GlobalName != "" {
		senderDisplay = author.GlobalName
	} else {
		senderDisplay = author.Username
	}
	senderLabel := senderDisplay
	if senderDisplay != "" && senderTag != "" && senderDisplay != senderTag {
		senderLabel = senderDisplay + " (" + senderTag + ")"
	} else if senderDisplay == "" {
		senderLabel = senderTag
		if senderLabel == "" {
			senderLabel = author.ID
		}
	}
	return DiscordSenderIdentity{
		ID: author.ID, Name: author.Username, Tag: senderTag,
		Label: senderLabel, IsPluralKit: false,
	}
}

// ResolveDiscordSenderLabel 解析发送者标签
func ResolveDiscordSenderLabel(author DiscordAuthorInfo, member *DiscordMemberInfo, pkInfo *PluralKitMessageInfo) string {
	return ResolveDiscordSenderIdentity(author, member, pkInfo).Label
}
