package channels

// 频道配置辅助 — 继承自 src/channels/plugins/config-helpers.ts (114 行)
// 管理账户启用、删除、嵌套账户遍历

// AccountConfig 通用账户配置接口（简化）
type AccountConfig struct {
	Enabled  *bool                  `json:"enabled,omitempty"`
	Accounts map[string]interface{} `json:"accounts,omitempty"`
}

// ListAccountIDs 列出频道配置中的所有账户 ID
// 支持两种结构：嵌套 accounts map 或顶层作为 default 账户
func ListAccountIDs(cfg *AccountConfig) []string {
	if cfg == nil {
		return nil
	}
	if len(cfg.Accounts) > 0 {
		ids := make([]string, 0, len(cfg.Accounts))
		for id := range cfg.Accounts {
			ids = append(ids, id)
		}
		return ids
	}
	return []string{DefaultAccountID}
}

// IsAccountEnabledInConfig 判断嵌套账户是否启用
func IsAccountEnabledInConfig(cfg *AccountConfig, accountID string) bool {
	if cfg == nil {
		return false
	}
	// 检查顶层 enabled
	if cfg.Enabled != nil && !*cfg.Enabled {
		return false
	}
	// 嵌套账户
	if len(cfg.Accounts) > 0 {
		acct, ok := cfg.Accounts[accountID]
		if !ok {
			return false
		}
		if m, ok := acct.(map[string]interface{}); ok {
			if e, ok := m["enabled"].(bool); ok {
				return e
			}
		}
		return true // 存在但无 enabled 字段 = 默认启用
	}
	// 顶层作为 default 账户
	if accountID == DefaultAccountID || accountID == "" {
		return IsAccountEnabled(cfg.Enabled)
	}
	return false
}

// DeleteAccountConfig 删除指定账户配置
func DeleteAccountConfig(cfg *AccountConfig, accountID string) {
	if cfg == nil {
		return
	}
	if len(cfg.Accounts) > 0 {
		delete(cfg.Accounts, accountID)
		return
	}
	// 顶层账户：置空 enabled
	if accountID == DefaultAccountID || accountID == "" {
		f := false
		cfg.Enabled = &f
	}
}

// ToggleAccountEnabled 切换账户启停状态
func ToggleAccountEnabled(cfg *AccountConfig, accountID string, enabled bool) {
	if cfg == nil {
		return
	}
	if len(cfg.Accounts) > 0 {
		acct, ok := cfg.Accounts[accountID]
		if !ok {
			return
		}
		if m, ok := acct.(map[string]interface{}); ok {
			m["enabled"] = enabled
		}
		return
	}
	if accountID == DefaultAccountID || accountID == "" {
		cfg.Enabled = &enabled
	}
}
