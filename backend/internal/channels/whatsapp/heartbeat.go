package whatsapp

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// WhatsApp 心跳收件人解析 — 继承自 src/channels/plugins/whatsapp-heartbeat.ts (78L)

// ResolveWhatsAppHeartbeatRecipients 解析心跳消息的收件人
// 优先使用配置的 allowFrom 列表，如果为空则从会话存储中获取
func ResolveWhatsAppHeartbeatRecipients(
	cfg *types.OpenAcosmiConfig,
	accountID string,
	sessionRecipients []string,
) []string {
	account := ResolveWhatsAppAccount(cfg, accountID)

	// 1. 如果有显式允许列表（非通配符），使用它
	var explicit []string
	for _, entry := range account.AllowFrom {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" && trimmed != "*" {
			explicit = append(explicit, trimmed)
		}
	}
	if len(explicit) > 0 {
		return deduplicateRecipients(explicit)
	}

	// 2. 使用会话存储中的最近联系人
	if len(sessionRecipients) > 0 {
		return deduplicateRecipients(sessionRecipients)
	}

	return nil
}

// deduplicateRecipients 去重收件人列表
func deduplicateRecipients(recipients []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, r := range recipients {
		normalized := strings.TrimSpace(r)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}
