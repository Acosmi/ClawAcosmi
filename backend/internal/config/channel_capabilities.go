package config

// 频道能力解析 — 对应 src/config/channel-capabilities.ts (74 行)
//
// 解析频道的 capabilities 配置，支持 account 级别覆盖。
// 查找优先级: accounts[accountId].capabilities → 顶层 capabilities
//
// 注意: Telegram 的 capabilities 特殊，既可以是 string[] 也可以是 {inlineButtons, tags} 对象。
// 在 Go 中对应 *types.TelegramCapabilitiesConfig，通过 Tags 字段获取字符串列表。

import (
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// normalizeCapabilities 规范化能力列表
// 对应 TS channel-capabilities.ts:L11-L18
func normalizeCapabilities(capabilities []string) []string {
	if len(capabilities) == 0 {
		return nil
	}
	var result []string
	for _, entry := range capabilities {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// extractTelegramCaps 从 TelegramCapabilitiesConfig 提取标签列表
func extractTelegramCaps(caps *types.TelegramCapabilitiesConfig) []string {
	if caps == nil {
		return nil
	}
	return normalizeCapabilities(caps.Tags)
}

// ResolveChannelCapabilities 解析频道的能力列表。
// 查找优先级: channels.[channel].accounts[accountId].capabilities
//
//	→ channels.[channel].capabilities
//
// 对应 TS channel-capabilities.ts:L51-L73
func ResolveChannelCapabilities(cfg *types.OpenAcosmiConfig, channel string, accountID string) []string {
	if cfg == nil || channel == "" {
		return nil
	}

	normalized := strings.ToLower(strings.TrimSpace(channel))
	if normalized == "" || cfg.Channels == nil {
		return nil
	}

	normalizedAcct := normalizeAccountId(accountID)

	switch normalized {
	case "telegram":
		if tg := cfg.Channels.Telegram; tg != nil {
			topLevel := extractTelegramCaps(tg.Capabilities)
			if normalizedAcct != "" && tg.Accounts != nil {
				if acct := findAccountCaseInsensitive(tg.Accounts, normalizedAcct); acct != nil {
					if caps := extractTelegramCaps(acct.Capabilities); caps != nil {
						return caps
					}
				}
			}
			return topLevel
		}
	case "discord":
		if dc := cfg.Channels.Discord; dc != nil {
			return resolveStringCapabilities(dc.Capabilities, dc.Accounts, normalizedAcct,
				func(a *types.DiscordAccountConfig) []string { return a.Capabilities })
		}
	case "slack":
		if sl := cfg.Channels.Slack; sl != nil {
			return resolveStringCapabilities(sl.Capabilities, sl.Accounts, normalizedAcct,
				func(a *types.SlackAccountConfig) []string { return a.Capabilities })
		}
	case "signal":
		if sg := cfg.Channels.Signal; sg != nil {
			return resolveStringCapabilities(sg.Capabilities, sg.Accounts, normalizedAcct,
				func(a *types.SignalAccountConfig) []string { return a.Capabilities })
		}
	case "whatsapp":
		if wa := cfg.Channels.WhatsApp; wa != nil {
			return resolveStringCapabilities(wa.Capabilities, wa.Accounts, normalizedAcct,
				func(a *types.WhatsAppAccountConfig) []string { return a.Capabilities })
		}
	}

	return nil
}

// resolveStringCapabilities 泛化处理 []string 类型的 capabilities
func resolveStringCapabilities[T any](
	topLevelCaps []string,
	accounts map[string]*T,
	normalizedAcct string,
	getCaps func(*T) []string,
) []string {
	topLevel := normalizeCapabilities(topLevelCaps)

	if normalizedAcct == "" || len(accounts) == 0 {
		return topLevel
	}

	// 精确匹配
	if acct, ok := accounts[normalizedAcct]; ok && acct != nil {
		if caps := normalizeCapabilities(getCaps(acct)); caps != nil {
			return caps
		}
		return topLevel
	}

	// 大小写不敏感匹配
	for key, acct := range accounts {
		if strings.ToLower(key) == normalizedAcct && acct != nil {
			if caps := normalizeCapabilities(getCaps(acct)); caps != nil {
				return caps
			}
			return topLevel
		}
	}

	return topLevel
}

// findAccountCaseInsensitive 大小写不敏感查找 Telegram 账号
func findAccountCaseInsensitive(accounts map[string]*types.TelegramAccountConfig, normalizedAcct string) *types.TelegramAccountConfig {
	if acct := accounts[normalizedAcct]; acct != nil {
		return acct
	}
	for key, acct := range accounts {
		if strings.ToLower(key) == normalizedAcct {
			return acct
		}
	}
	return nil
}
