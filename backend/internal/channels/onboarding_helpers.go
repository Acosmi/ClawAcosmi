package channels

import "strings"

// Onboarding 辅助 — 继承自 src/channels/plugins/onboarding/helpers.ts (46L)

// NormalizeAccountID 规范化账户 ID（小写+去空格）
func NormalizeAccountID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return DefaultAccountID
	}
	return strings.ToLower(trimmed)
}

// AddWildcardAllowFrom 确保 allowFrom 列表包含通配符
func AddWildcardAllowFrom(allowFrom []string) []string {
	var next []string
	hasWildcard := false
	for _, v := range allowFrom {
		t := strings.TrimSpace(v)
		if t == "" {
			continue
		}
		if t == "*" {
			hasWildcard = true
		}
		next = append(next, t)
	}
	if !hasWildcard {
		next = append(next, "*")
	}
	return next
}
