package types

// 执行审批配置类型 — 继承自 src/config/types.approvals.ts (30 行)

// ExecApprovalForwardingMode 审批转发模式
type ExecApprovalForwardingMode string

const (
	ExecApprovalSession ExecApprovalForwardingMode = "session"
	ExecApprovalTargets ExecApprovalForwardingMode = "targets"
	ExecApprovalBoth    ExecApprovalForwardingMode = "both"
)

// ExecApprovalForwardTarget 审批转发目标
type ExecApprovalForwardTarget struct {
	Channel   string      `json:"channel"`
	To        string      `json:"to"`
	AccountID string      `json:"accountId,omitempty"`
	ThreadID  interface{} `json:"threadId,omitempty"` // string|number
}

// ExecApprovalForwardingConfig 执行审批转发配置
type ExecApprovalForwardingConfig struct {
	Enabled       *bool                       `json:"enabled,omitempty"`
	Mode          ExecApprovalForwardingMode  `json:"mode,omitempty"`
	AgentFilter   []string                    `json:"agentFilter,omitempty"`
	SessionFilter []string                    `json:"sessionFilter,omitempty"`
	Targets       []ExecApprovalForwardTarget `json:"targets,omitempty"`
}

// ApprovalsConfig 审批总配置
type ApprovalsConfig struct {
	Exec *ExecApprovalForwardingConfig `json:"exec,omitempty"`
}
