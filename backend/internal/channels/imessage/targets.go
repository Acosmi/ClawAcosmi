//go:build darwin

package imessage

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/utils"
)

// iMessage handle/target 规范化 — 继承自 src/imessage/targets.ts (234L)

// IMessageService iMessage 服务类型
type IMessageService string

const (
	ServiceIMessage IMessageService = "imessage"
	ServiceSMS      IMessageService = "sms"
	ServiceAuto     IMessageService = "auto"
)

// IMessageTargetKind 发送目标类型
type IMessageTargetKind string

const (
	TargetKindChatID         IMessageTargetKind = "chat_id"
	TargetKindChatGUID       IMessageTargetKind = "chat_guid"
	TargetKindChatIdentifier IMessageTargetKind = "chat_identifier"
	TargetKindHandle         IMessageTargetKind = "handle"
)

// IMessageTarget 解析后的 iMessage 发送目标
type IMessageTarget struct {
	Kind           IMessageTargetKind
	ChatID         int
	ChatGUID       string
	ChatIdentifier string
	To             string
	Service        IMessageService
}

// IMessageAllowTarget 允许列表形式的目标（用于 allowFrom 匹配）
type IMessageAllowTarget struct {
	Kind           IMessageTargetKind
	ChatID         int
	ChatGUID       string
	ChatIdentifier string
	Handle         string
}

// 前缀常量
var (
	chatIDPrefixes         = []string{"chat_id:", "chatid:", "chat:"}
	chatGUIDPrefixes       = []string{"chat_guid:", "chatguid:", "guid:"}
	chatIdentifierPrefixes = []string{"chat_identifier:", "chatidentifier:", "chatident:"}
	servicePrefixes        = []struct {
		prefix  string
		service IMessageService
	}{
		{"imessage:", ServiceIMessage},
		{"sms:", ServiceSMS},
		{"auto:", ServiceAuto},
	}
)

var whitespaceRe = regexp.MustCompile(`\s+`)

// NormalizeIMessageHandle 规范化 iMessage handle（电话号码/邮箱/chat_id 等）
func NormalizeIMessageHandle(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lowered := strings.ToLower(trimmed)

	// 递归剥离 service 前缀
	if strings.HasPrefix(lowered, "imessage:") {
		return NormalizeIMessageHandle(trimmed[9:])
	}
	if strings.HasPrefix(lowered, "sms:") {
		return NormalizeIMessageHandle(trimmed[4:])
	}
	if strings.HasPrefix(lowered, "auto:") {
		return NormalizeIMessageHandle(trimmed[5:])
	}

	// 规范化 chat_id/chat_guid/chat_identifier 前缀（大小写不敏感）
	for _, prefix := range chatIDPrefixes {
		if strings.HasPrefix(lowered, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			return "chat_id:" + value
		}
	}
	for _, prefix := range chatGUIDPrefixes {
		if strings.HasPrefix(lowered, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			return "chat_guid:" + value
		}
	}
	for _, prefix := range chatIdentifierPrefixes {
		if strings.HasPrefix(lowered, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			return "chat_identifier:" + value
		}
	}

	// 邮箱地址 → 小写
	if strings.Contains(trimmed, "@") {
		return strings.ToLower(trimmed)
	}

	// 电话号码 → E.164 规范化
	normalized := utils.NormalizeE164(trimmed)
	if normalized != trimmed {
		return normalized
	}

	// 兜底：去除空白
	return whitespaceRe.ReplaceAllString(trimmed, "")
}

// ParseIMessageTarget 解析 iMessage 发送目标字符串
func ParseIMessageTarget(raw string) (*IMessageTarget, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("iMessage target is required")
	}
	lower := strings.ToLower(trimmed)

	// 检查 service 前缀
	for _, sp := range servicePrefixes {
		if strings.HasPrefix(lower, sp.prefix) {
			remainder := strings.TrimSpace(trimmed[len(sp.prefix):])
			if remainder == "" {
				return nil, fmt.Errorf("%s target is required", sp.prefix)
			}
			remainderLower := strings.ToLower(remainder)
			// 检查是否是 chat 类型目标
			isChatTarget := hasPrefixAny(remainderLower, chatIDPrefixes) ||
				hasPrefixAny(remainderLower, chatGUIDPrefixes) ||
				hasPrefixAny(remainderLower, chatIdentifierPrefixes)
			if isChatTarget {
				return ParseIMessageTarget(remainder)
			}
			return &IMessageTarget{Kind: TargetKindHandle, To: remainder, Service: sp.service}, nil
		}
	}

	// 检查 chat_id 前缀
	for _, prefix := range chatIDPrefixes {
		if strings.HasPrefix(lower, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			chatID, err := strconv.Atoi(value)
			if err != nil || math.IsInf(float64(chatID), 0) {
				return nil, fmt.Errorf("invalid chat_id: %s", value)
			}
			return &IMessageTarget{Kind: TargetKindChatID, ChatID: chatID}, nil
		}
	}

	// 检查 chat_guid 前缀
	for _, prefix := range chatGUIDPrefixes {
		if strings.HasPrefix(lower, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			if value == "" {
				return nil, fmt.Errorf("chat_guid is required")
			}
			return &IMessageTarget{Kind: TargetKindChatGUID, ChatGUID: value}, nil
		}
	}

	// 检查 chat_identifier 前缀
	for _, prefix := range chatIdentifierPrefixes {
		if strings.HasPrefix(lower, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			if value == "" {
				return nil, fmt.Errorf("chat_identifier is required")
			}
			return &IMessageTarget{Kind: TargetKindChatIdentifier, ChatIdentifier: value}, nil
		}
	}

	// 默认：handle, service "auto"
	return &IMessageTarget{Kind: TargetKindHandle, To: trimmed, Service: ServiceAuto}, nil
}

// ParseIMessageAllowTarget 解析 iMessage 允许列表目标
func ParseIMessageAllowTarget(raw string) IMessageAllowTarget {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return IMessageAllowTarget{Kind: TargetKindHandle, Handle: ""}
	}
	lower := strings.ToLower(trimmed)

	// 剥离 service 前缀
	for _, sp := range servicePrefixes {
		if strings.HasPrefix(lower, sp.prefix) {
			remainder := strings.TrimSpace(trimmed[len(sp.prefix):])
			if remainder == "" {
				return IMessageAllowTarget{Kind: TargetKindHandle, Handle: ""}
			}
			return ParseIMessageAllowTarget(remainder)
		}
	}

	// 检查 chat_id 前缀
	for _, prefix := range chatIDPrefixes {
		if strings.HasPrefix(lower, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			chatID, err := strconv.Atoi(value)
			if err == nil {
				return IMessageAllowTarget{Kind: TargetKindChatID, ChatID: chatID}
			}
		}
	}

	// 检查 chat_guid 前缀
	for _, prefix := range chatGUIDPrefixes {
		if strings.HasPrefix(lower, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			if value != "" {
				return IMessageAllowTarget{Kind: TargetKindChatGUID, ChatGUID: value}
			}
		}
	}

	// 检查 chat_identifier 前缀
	for _, prefix := range chatIdentifierPrefixes {
		if strings.HasPrefix(lower, prefix) {
			value := strings.TrimSpace(trimmed[len(prefix):])
			if value != "" {
				return IMessageAllowTarget{Kind: TargetKindChatIdentifier, ChatIdentifier: value}
			}
		}
	}

	// 默认：handle
	return IMessageAllowTarget{Kind: TargetKindHandle, Handle: NormalizeIMessageHandle(trimmed)}
}

// IsAllowedIMessageSender 检查发送者是否在允许列表中
func IsAllowedIMessageSender(params IsAllowedParams) bool {
	if len(params.AllowFrom) == 0 {
		return true
	}

	// 规范化 allowFrom 列表
	var allowEntries []string
	for _, entry := range params.AllowFrom {
		s := strings.TrimSpace(fmt.Sprintf("%v", entry))
		allowEntries = append(allowEntries, s)
	}

	// 通配符检查
	for _, entry := range allowEntries {
		if entry == "*" {
			return true
		}
	}

	senderNormalized := NormalizeIMessageHandle(params.Sender)

	for _, entry := range allowEntries {
		if entry == "" {
			continue
		}
		parsed := ParseIMessageAllowTarget(entry)
		switch parsed.Kind {
		case TargetKindChatID:
			if params.ChatID != nil && parsed.ChatID == *params.ChatID {
				return true
			}
		case TargetKindChatGUID:
			if params.ChatGUID != "" && parsed.ChatGUID == params.ChatGUID {
				return true
			}
		case TargetKindChatIdentifier:
			if params.ChatIdentifier != "" && parsed.ChatIdentifier == params.ChatIdentifier {
				return true
			}
		case TargetKindHandle:
			if senderNormalized != "" && parsed.Handle == senderNormalized {
				return true
			}
		}
	}
	return false
}

// IsAllowedParams IsAllowedIMessageSender 的参数
type IsAllowedParams struct {
	AllowFrom      []interface{}
	Sender         string
	ChatID         *int
	ChatGUID       string
	ChatIdentifier string
}

// FormatIMessageChatTarget 格式化 chat_id 目标字符串
func FormatIMessageChatTarget(chatID *int) string {
	if chatID == nil || *chatID == 0 {
		return ""
	}
	return fmt.Sprintf("chat_id:%d", *chatID)
}

// hasPrefixAny 检查字符串是否以任意前缀开头
func hasPrefixAny(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
