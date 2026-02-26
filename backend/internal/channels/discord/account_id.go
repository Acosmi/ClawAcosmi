package discord

import (
	"regexp"
	"strings"
)

// Discord 账户 ID 工具 — 继承自 src/routing/session-key.ts 中的 normalizeAccountId + DEFAULT_ACCOUNT_ID

// defaultAccountID 默认账户 ID（与 channels.DefaultAccountID 一致，避免循环导入）
const defaultAccountID = "default"

var (
	validIDRe      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	invalidCharsRe = regexp.MustCompile(`[^a-z0-9_-]+`)
	leadingDashRe  = regexp.MustCompile(`^-+`)
	trailingDashRe = regexp.MustCompile(`-+$`)
)

// NormalizeAccountID 规范化账户 ID：小写 + 正则校验 + 非法字符消毒 + 64字符截断。
// 完整继承 TS normalizeAccountId 的校验、消毒、截断逻辑。
func NormalizeAccountID(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultAccountID
	}
	lower := strings.ToLower(trimmed)
	if validIDRe.MatchString(lower) {
		return lower
	}
	sanitized := invalidCharsRe.ReplaceAllString(lower, "-")
	sanitized = leadingDashRe.ReplaceAllString(sanitized, "")
	sanitized = trailingDashRe.ReplaceAllString(sanitized, "")
	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}
	if sanitized == "" {
		return defaultAccountID
	}
	return sanitized
}
