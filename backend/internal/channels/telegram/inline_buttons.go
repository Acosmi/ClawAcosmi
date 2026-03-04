package telegram

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// Telegram 内联按钮 — 继承自 src/telegram/inline-buttons.ts (82L)

const defaultInlineButtonsScope = types.TelegramInlineAllowlist

// ResolveTelegramInlineButtonsScope 解析内联按钮启用范围
func ResolveTelegramInlineButtonsScope(cfg *types.OpenAcosmiConfig, accountID string) types.TelegramInlineButtonsScope {
	account := ResolveTelegramAccount(cfg, accountID)
	caps := account.Config.Capabilities
	if caps == nil {
		return defaultInlineButtonsScope
	}
	// 对齐 TS: 先检查 tags 数组（数组形式 capabilities 优先）
	// 使用 caps.Tags != nil 而非 len > 0，以处理空数组 JSON [] 的情况（TS 返回 "off"）
	if caps.Tags != nil {
		for _, tag := range caps.Tags {
			if strings.EqualFold(strings.TrimSpace(tag), "inlinebuttons") {
				return types.TelegramInlineAll
			}
		}
		// 数组形式但不含 inlineButtons → 明确关闭（对齐 TS 行为）
		return types.TelegramInlineOff
	}
	// 对象形式 → 检查 InlineButtons 字段
	if scope := caps.InlineButtons; scope != "" {
		normalized := types.TelegramInlineButtonsScope(strings.TrimSpace(strings.ToLower(string(scope))))
		switch normalized {
		case types.TelegramInlineOff, types.TelegramInlineDM, types.TelegramInlineGroup,
			types.TelegramInlineAll, types.TelegramInlineAllowlist:
			return normalized
		}
	}
	return defaultInlineButtonsScope
}

// IsTelegramInlineButtonsEnabled 检查是否有任何账户启用了内联按钮
func IsTelegramInlineButtonsEnabled(cfg *types.OpenAcosmiConfig, accountID string) bool {
	if accountID != "" {
		return ResolveTelegramInlineButtonsScope(cfg, accountID) != types.TelegramInlineOff
	}
	ids := ListTelegramAccountIds(cfg)
	// 对齐 TS: 无账户时检查默认/根级配置
	if len(ids) == 0 {
		return ResolveTelegramInlineButtonsScope(cfg, "") != types.TelegramInlineOff
	}
	for _, id := range ids {
		if ResolveTelegramInlineButtonsScope(cfg, id) != types.TelegramInlineOff {
			return true
		}
	}
	return false
}
