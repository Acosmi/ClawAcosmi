package types

// Node Host 配置类型 — 继承自 src/config/types.node-host.ts (12 行)

// NodeHostBrowserProxyConfig Node Host 浏览器代理配置
type NodeHostBrowserProxyConfig struct {
	Enabled       *bool    `json:"enabled,omitempty"`
	AllowProfiles []string `json:"allowProfiles,omitempty"`
}

// NodeHostConfig Node Host 配置
type NodeHostConfig struct {
	BrowserProxy *NodeHostBrowserProxyConfig `json:"browserProxy,omitempty"`
}
