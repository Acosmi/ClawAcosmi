package types

// 飞书/Lark 频道配置类型 — 对应 china-channel-sdk-tracker.md Phase 2
// 飞书支持 WebSocket 长连接模式（推荐）和 HTTP 回调模式

// FeishuAccountConfig 飞书账号配置（SDK WebSocket 长连接模式）
type FeishuAccountConfig struct {
	Name      string `json:"name,omitempty"`
	AppID     string `json:"appId,omitempty"`
	AppSecret string `json:"appSecret,omitempty"`
	Domain    string `json:"domain,omitempty"` // "feishu" | "lark"（国际版）
	Enabled   *bool  `json:"enabled,omitempty"`

	// 审批通知专用群 ID（可选）。
	// 配置后，所有审批卡片（Escalation/CoderConfirmation）会发送到该群，
	// 解决 Web UI 发起的审批请求无法送达飞书的问题。
	ApprovalChatID string `json:"approvalChatId,omitempty"`

	// 通用频道字段
	GroupPolicy    GroupPolicy                       `json:"groupPolicy,omitempty"`
	Heartbeat      *ChannelHeartbeatVisibilityConfig `json:"heartbeat,omitempty"`
	ResponsePrefix string                            `json:"responsePrefix,omitempty"`
}

// FeishuConfig 飞书总配置（嵌入 FeishuAccountConfig 实现 TS 的 & 交叉类型）
type FeishuConfig struct {
	FeishuAccountConfig
	Accounts map[string]*FeishuAccountConfig `json:"accounts,omitempty"`
}
