package autoreply

import "strings"

// TS 对照: auto-reply/send-policy.ts

// SendPolicyOverride 发送策略覆盖。
type SendPolicyOverride string

const (
	SendPolicyAllow SendPolicyOverride = "allow"
	SendPolicyDeny  SendPolicyOverride = "deny"
)

// NormalizeSendPolicyOverride 规范化发送策略覆盖。
// TS 对照: send-policy.ts L5-17
func NormalizeSendPolicyOverride(raw string) (SendPolicyOverride, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return "", false
	}
	switch value {
	case "allow", "on":
		return SendPolicyAllow, true
	case "deny", "off":
		return SendPolicyDeny, true
	}
	return "", false
}

// ParseSendPolicyCommand 解析 /send 命令。
// TS 对照: send-policy.ts L19-44
func ParseSendPolicyCommand(raw string) (hasCommand bool, mode string) {
	if raw == "" {
		return false, ""
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false, ""
	}
	normalized := NormalizeCommandBody(trimmed, nil)
	lower := strings.ToLower(strings.TrimSpace(normalized))
	if !strings.HasPrefix(lower, "/send") {
		return false, ""
	}
	rest := strings.TrimSpace(lower[len("/send"):])
	// 确保是完整命令
	parts := strings.Fields(normalized)
	if len(parts) > 2 {
		return false, ""
	}
	if rest == "" {
		return true, ""
	}
	token := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if token == "inherit" || token == "default" || token == "reset" {
		return true, "inherit"
	}
	policy, ok := NormalizeSendPolicyOverride(token)
	if ok {
		return true, string(policy)
	}
	return true, ""
}
