package channels

import "strings"

// 配置写入权限 — 继承自 src/channels/plugins/config-writes.ts (41L)

// ChannelConfigWithAccounts 配置写入权限结构
type ChannelConfigWithAccounts struct {
	ConfigWrites *bool                  `json:"configWrites,omitempty"`
	Accounts     map[string]interface{} `json:"accounts,omitempty"`
}

// resolveAccountConfigWrites 解析账户级配置写入权限
func resolveAccountConfigWrites(accounts map[string]interface{}, accountID string) *bool {
	if len(accounts) == 0 {
		return nil
	}
	if acct, ok := accounts[accountID]; ok {
		if m, ok := acct.(map[string]interface{}); ok {
			if v, ok := m["configWrites"].(bool); ok {
				return &v
			}
		}
		return nil
	}
	// 大小写不敏感回退
	lower := strings.ToLower(accountID)
	for key, acct := range accounts {
		if strings.ToLower(key) == lower {
			if m, ok := acct.(map[string]interface{}); ok {
				if v, ok := m["configWrites"].(bool); ok {
					return &v
				}
			}
			return nil
		}
	}
	return nil
}

// ResolveChannelConfigWrites 判断频道是否允许配置写入
func ResolveChannelConfigWrites(channelConfig map[string]interface{}, channelID, accountID string) bool {
	if channelID == "" {
		return true
	}
	if len(channelConfig) == 0 {
		return true
	}
	raw, ok := channelConfig[channelID]
	if !ok {
		return true
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return true
	}
	// 账户级覆盖
	normalizedAccountID := strings.ToLower(strings.TrimSpace(accountID))
	if normalizedAccountID == "" {
		normalizedAccountID = DefaultAccountID
	}
	if accts, ok := m["accounts"].(map[string]interface{}); ok {
		if v := resolveAccountConfigWrites(accts, normalizedAccountID); v != nil {
			return *v
		}
	}
	// 频道级
	if v, ok := m["configWrites"].(bool); ok {
		return v
	}
	return true
}
