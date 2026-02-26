package channels

import "strings"

// Setup 辅助 — 继承自 src/channels/plugins/setup-helpers.ts (122L)
// 账户名写入和迁移逻辑

// channelHasAccounts 检查频道是否配置了多账户
func channelHasAccounts(channelConfig map[string]interface{}, channelKey string) bool {
	ch, ok := channelConfig[channelKey].(map[string]interface{})
	if !ok {
		return false
	}
	accts, ok := ch["accounts"].(map[string]interface{})
	return ok && len(accts) > 0
}

// shouldStoreNameInAccounts 判断账户名应写入嵌套 accounts 还是顶层
func shouldStoreNameInAccounts(channelConfig map[string]interface{}, channelKey, accountID string, alwaysUseAccounts bool) bool {
	if alwaysUseAccounts {
		return true
	}
	normalized := strings.ToLower(strings.TrimSpace(accountID))
	if normalized == "" {
		normalized = DefaultAccountID
	}
	if normalized != DefaultAccountID {
		return true
	}
	return channelHasAccounts(channelConfig, channelKey)
}

// ApplyAccountNameToChannelSection 将账户名写入频道配置
func ApplyAccountNameToChannelSection(channelConfig map[string]interface{}, channelKey, accountID, name string, alwaysUseAccounts bool) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return
	}
	normalized := strings.ToLower(strings.TrimSpace(accountID))
	if normalized == "" {
		normalized = DefaultAccountID
	}

	// 确保频道配置存在
	ch, ok := channelConfig[channelKey].(map[string]interface{})
	if !ok {
		ch = make(map[string]interface{})
		channelConfig[channelKey] = ch
	}

	useAccounts := shouldStoreNameInAccounts(channelConfig, channelKey, normalized, alwaysUseAccounts)
	if !useAccounts && normalized == DefaultAccountID {
		ch["name"] = trimmed
		return
	}

	// 写入嵌套 accounts
	accts, ok := ch["accounts"].(map[string]interface{})
	if !ok {
		accts = make(map[string]interface{})
		ch["accounts"] = accts
	}
	acct, ok := accts[normalized].(map[string]interface{})
	if !ok {
		acct = make(map[string]interface{})
		accts[normalized] = acct
	}
	acct["name"] = trimmed

	// 如果是 default 账户，移除顶层 name
	if normalized == DefaultAccountID {
		delete(ch, "name")
	}
}

// MigrateBaseNameToDefaultAccount 将顶层 name 迁移到 default 账户
func MigrateBaseNameToDefaultAccount(channelConfig map[string]interface{}, channelKey string, alwaysUseAccounts bool) {
	if alwaysUseAccounts {
		return
	}
	ch, ok := channelConfig[channelKey].(map[string]interface{})
	if !ok {
		return
	}
	baseName, ok := ch["name"].(string)
	if !ok || strings.TrimSpace(baseName) == "" {
		return
	}
	accts, ok := ch["accounts"].(map[string]interface{})
	if !ok {
		accts = make(map[string]interface{})
		ch["accounts"] = accts
	}
	defAcct, ok := accts[DefaultAccountID].(map[string]interface{})
	if !ok {
		defAcct = make(map[string]interface{})
		accts[DefaultAccountID] = defAcct
	}
	if _, hasName := defAcct["name"]; !hasName {
		defAcct["name"] = strings.TrimSpace(baseName)
	}
	delete(ch, "name")
}
