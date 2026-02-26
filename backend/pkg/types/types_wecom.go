package types

// 企业微信 (WeCom) 频道配置类型 — 对应 china-channel-sdk-tracker.md Phase 2
// 企业微信仅支持 HTTP 回调模式（不支持长连接）

// WeComAccountConfig 企业微信账号配置
type WeComAccountConfig struct {
	Name    string `json:"name,omitempty"`
	CorpID  string `json:"corpId,omitempty"`  // 企业 ID
	Secret  string `json:"secret,omitempty"`  // 应用 Secret
	AgentID *int   `json:"agentId,omitempty"` // 应用 AgentID（数值类型）
	Token   string `json:"token,omitempty"`   // 回调 URL 验证 token
	AESKey  string `json:"aesKey,omitempty"`  // 消息加解密密钥
	Enabled *bool  `json:"enabled,omitempty"`

	// 通用频道字段
	GroupPolicy    GroupPolicy                       `json:"groupPolicy,omitempty"`
	Heartbeat      *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	ResponsePrefix string                            `json:"responsePrefix,omitempty"`
}

// WeComConfig 企业微信总配置（嵌入 WeComAccountConfig 实现 TS 的 & 交叉类型）
type WeComConfig struct {
	WeComAccountConfig
	Accounts map[string]*WeComAccountConfig `json:"accounts,omitempty"`
}
