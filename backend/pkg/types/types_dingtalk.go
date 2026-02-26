package types

// 钉钉频道配置类型 — 对应 china-channel-sdk-tracker.md Phase 2
// 钉钉支持 Stream 长连接模式（推荐）和 Outgoing 回调模式

// DingTalkAccountConfig 钉钉账号配置
type DingTalkAccountConfig struct {
	Name      string `json:"name,omitempty"`
	AppKey    string `json:"appKey,omitempty"`
	AppSecret string `json:"appSecret,omitempty"`
	RobotCode string `json:"robotCode,omitempty"` // 机器人代码
	Token     string `json:"token,omitempty"`     // Outgoing 回调验证 token
	AESKey    string `json:"aesKey,omitempty"`    // 事件加密密钥
	Enabled   *bool  `json:"enabled,omitempty"`

	// 通用频道字段
	GroupPolicy    GroupPolicy                       `json:"groupPolicy,omitempty"`
	Heartbeat      *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	ResponsePrefix string                            `json:"responsePrefix,omitempty"`
}

// DingTalkConfig 钉钉总配置（嵌入 DingTalkAccountConfig 实现 TS 的 & 交叉类型）
type DingTalkConfig struct {
	DingTalkAccountConfig
	Accounts map[string]*DingTalkAccountConfig `json:"accounts,omitempty"`
}
