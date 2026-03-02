package whatsapp

import (
	"regexp"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/utils"
)

// WhatsApp 号码/JID 规范化 — 继承自 src/whatsapp/normalize.ts (81L)

var (
	whatsappUserJidRe = regexp.MustCompile(`(?i)^(\d+)(?::\d+)?@s\.whatsapp\.net$`)
	whatsappLidRe     = regexp.MustCompile(`(?i)^(\d+)@lid$`)
	groupLocalPartRe  = regexp.MustCompile(`^[0-9]+(-[0-9]+)*$`)
)

// stripWhatsAppTargetPrefixes 循环去除 "whatsapp:" 前缀
func stripWhatsAppTargetPrefixes(value string) string {
	candidate := strings.TrimSpace(value)
	for {
		before := candidate
		lower := strings.ToLower(candidate)
		if strings.HasPrefix(lower, "whatsapp:") {
			candidate = strings.TrimSpace(candidate[len("whatsapp:"):])
		}
		if candidate == before {
			return candidate
		}
	}
}

// IsWhatsAppGroupJid 判断是否为 WhatsApp 群组 JID（如 "120363XXX@g.us"）
func IsWhatsAppGroupJid(value string) bool {
	candidate := stripWhatsAppTargetPrefixes(value)
	lower := strings.ToLower(candidate)
	if !strings.HasSuffix(lower, "@g.us") {
		return false
	}
	localPart := candidate[:len(candidate)-len("@g.us")]
	if localPart == "" || strings.Contains(localPart, "@") {
		return false
	}
	return groupLocalPartRe.MatchString(localPart)
}

// IsWhatsAppUserTarget 判断是否为 WhatsApp 用户目标
// 支持格式：
//   - "41796666864:0@s.whatsapp.net"（标准 JID）
//   - "123456@lid"（LID 格式）
func IsWhatsAppUserTarget(value string) bool {
	candidate := stripWhatsAppTargetPrefixes(value)
	return whatsappUserJidRe.MatchString(candidate) || whatsappLidRe.MatchString(candidate)
}

// extractUserJidPhone 从 WhatsApp 用户 JID 中提取电话号码
// "41796666864:0@s.whatsapp.net" -> "41796666864"
// "123456@lid" -> "123456"
func extractUserJidPhone(jid string) string {
	if m := whatsappUserJidRe.FindStringSubmatch(jid); len(m) > 1 {
		return m[1]
	}
	if m := whatsappLidRe.FindStringSubmatch(jid); len(m) > 1 {
		return m[1]
	}
	return ""
}

// NormalizeWhatsAppTarget 规范化 WhatsApp 目标地址
// 支持群组 JID、用户 JID/LID、纯电话号码
func NormalizeWhatsAppTarget(value string) string {
	candidate := stripWhatsAppTargetPrefixes(value)
	if candidate == "" {
		return ""
	}
	// 群组 JID
	if IsWhatsAppGroupJid(candidate) {
		localPart := candidate[:len(candidate)-len("@g.us")]
		return localPart + "@g.us"
	}
	// 用户 JID（"41796666864:0@s.whatsapp.net"）或 LID（"123@lid"）
	if IsWhatsAppUserTarget(candidate) {
		phone := extractUserJidPhone(candidate)
		if phone == "" {
			return ""
		}
		normalized := utils.NormalizeE164(phone)
		if len(normalized) > 1 {
			return normalized
		}
		return ""
	}
	// 含 @ 但不是已知格式 → 拒绝
	if strings.Contains(candidate, "@") {
		return ""
	}
	// 纯电话号码
	normalized := utils.NormalizeE164(candidate)
	if len(normalized) > 1 {
		return normalized
	}
	return ""
}
