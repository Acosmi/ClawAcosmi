package signal

import (
	"regexp"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/utils"
)

// Signal 发送者身份解析 — 继承自 src/signal/identity.ts (136L)

// SignalSenderKind 发送者类型
type SignalSenderKind string

const (
	SignalSenderPhone SignalSenderKind = "phone"
	SignalSenderUUID  SignalSenderKind = "uuid"
)

// SignalSender Signal 发送者身份
type SignalSender struct {
	Kind SignalSenderKind
	Raw  string
	E164 string // 仅 phone 类型有值
}

// SignalAllowEntry allowlist 条目
type signalAllowEntry struct {
	kind string // "any"|"phone"|"uuid"
	e164 string // phone 类型
	raw  string // uuid 类型
}

var uuidHyphenatedRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var uuidCompactRE = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
var hexAlphaRE = regexp.MustCompile(`(?i)[a-f]`)

func looksLikeUUID(value string) bool {
	if uuidHyphenatedRE.MatchString(value) || uuidCompactRE.MatchString(value) {
		return true
	}
	compact := strings.ReplaceAll(value, "-", "")
	if !regexp.MustCompile(`^[0-9a-fA-F]+$`).MatchString(compact) {
		return false
	}
	return hexAlphaRE.MatchString(compact)
}

func stripSignalPrefix(value string) string {
	if strings.HasPrefix(strings.ToLower(value), "signal:") {
		return strings.TrimSpace(value[len("signal:"):])
	}
	return strings.TrimSpace(value)
}

// ResolveSignalSender 从 envelope 中解析发送者身份
func ResolveSignalSender(sourceNumber, sourceUuid string) *SignalSender {
	number := strings.TrimSpace(sourceNumber)
	if number != "" {
		return &SignalSender{
			Kind: SignalSenderPhone,
			Raw:  number,
			E164: utils.NormalizeE164(number),
		}
	}
	uuid := strings.TrimSpace(sourceUuid)
	if uuid != "" {
		return &SignalSender{
			Kind: SignalSenderUUID,
			Raw:  uuid,
		}
	}
	return nil
}

// FormatSignalSenderId 格式化发送者 ID
func FormatSignalSenderId(sender *SignalSender) string {
	if sender.Kind == SignalSenderPhone {
		return sender.E164
	}
	return "uuid:" + sender.Raw
}

// FormatSignalSenderDisplay 格式化发送者显示名
func FormatSignalSenderDisplay(sender *SignalSender) string {
	if sender.Kind == SignalSenderPhone {
		return sender.E164
	}
	return "uuid:" + sender.Raw
}

// FormatSignalPairingIdLine 格式化配对 ID 行
func FormatSignalPairingIdLine(sender *SignalSender) string {
	if sender.Kind == SignalSenderPhone {
		return "Your Signal number: " + sender.E164
	}
	return "Your Signal sender id: " + FormatSignalSenderId(sender)
}

// ResolveSignalRecipient 解析收件人地址
func ResolveSignalRecipient(sender *SignalSender) string {
	if sender.Kind == SignalSenderPhone {
		return sender.E164
	}
	return sender.Raw
}

// ResolveSignalPeerId 解析对等 ID
func ResolveSignalPeerId(sender *SignalSender) string {
	if sender.Kind == SignalSenderPhone {
		return sender.E164
	}
	return "uuid:" + sender.Raw
}

// parseSignalAllowEntry 解析 allowlist 条目
func parseSignalAllowEntry(entry string) *signalAllowEntry {
	trimmed := strings.TrimSpace(entry)
	if trimmed == "" {
		return nil
	}
	if trimmed == "*" {
		return &signalAllowEntry{kind: "any"}
	}

	stripped := stripSignalPrefix(trimmed)
	lower := strings.ToLower(stripped)
	if strings.HasPrefix(lower, "uuid:") {
		raw := strings.TrimSpace(stripped[len("uuid:"):])
		if raw == "" {
			return nil
		}
		return &signalAllowEntry{kind: "uuid", raw: raw}
	}

	if looksLikeUUID(stripped) {
		return &signalAllowEntry{kind: "uuid", raw: stripped}
	}

	return &signalAllowEntry{kind: "phone", e164: utils.NormalizeE164(stripped)}
}

// IsSignalSenderAllowed 判断发送者是否在 allowlist 中
func IsSignalSenderAllowed(sender *SignalSender, allowFrom []string) bool {
	if len(allowFrom) == 0 {
		return false
	}
	var entries []*signalAllowEntry
	for _, raw := range allowFrom {
		if e := parseSignalAllowEntry(raw); e != nil {
			entries = append(entries, e)
		}
	}
	for _, e := range entries {
		if e.kind == "any" {
			return true
		}
	}
	for _, e := range entries {
		if e.kind == "phone" && sender.Kind == SignalSenderPhone {
			if e.e164 == sender.E164 {
				return true
			}
		}
		if e.kind == "uuid" && sender.Kind == SignalSenderUUID {
			if e.raw == sender.Raw {
				return true
			}
		}
	}
	return false
}

// IsSignalGroupAllowed 判断群组消息是否允许
func IsSignalGroupAllowed(groupPolicy string, allowFrom []string, sender *SignalSender) bool {
	if groupPolicy == "disabled" {
		return false
	}
	if groupPolicy == "open" {
		return true
	}
	if len(allowFrom) == 0 {
		return false
	}
	return IsSignalSenderAllowed(sender, allowFrom)
}
